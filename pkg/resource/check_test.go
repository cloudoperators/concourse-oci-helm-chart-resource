// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// fakeHelmChart builds a minimal OCI manifest + config blob pair that mimics
// what `helm push` produces: the config blob is Chart.yaml JSON (no Created
// field), and the manifest carries the creation time in annotations only.
func fakeHelmChart(version string, created time.Time) (manifest ocispec.Manifest, manifestJSON []byte, configJSON []byte) {
	// Config blob: Chart.yaml metadata — no Created field (like real helm charts)
	type chartMeta struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	configJSON, _ = json.Marshal(chartMeta{Name: "metal-maintenance-operator-remote", Version: version})
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(configJSON))

	// Layer blob (empty)
	layerData := []byte("fake-chart-content")
	layerDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(layerData))

	manifest = ocispec.Manifest{
		Config: ocispec.Descriptor{
			MediaType: "application/vnd.cncf.helm.config.v1+json",
			Digest:    digest.Digest(configDigest),
			Size:      int64(len(configJSON)),
		},
		Layers: []ocispec.Descriptor{
			{
				MediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
				Digest:    digest.Digest(layerDigest),
				Size:      int64(len(layerData)),
			},
		},
		Annotations: map[string]string{
			"org.opencontainers.image.created": created.UTC().Format(time.RFC3339),
			"org.opencontainers.image.version": version,
			"org.opencontainers.image.title":   "metal-maintenance-operator-remote",
		},
	}
	manifestJSON, _ = json.Marshal(manifest)
	return manifest, manifestJSON, configJSON
}

// fakeRegistry spins up an httptest TLS server that serves a minimal OCI
// distribution API for a single repository "ccloud-helm/metal-maintenance-operator-remote".
func fakeRegistry(t *testing.T, tags map[string]time.Time) *httptest.Server {
	t.Helper()

	type tagEntry struct {
		tag          string
		manifestJSON []byte
		configJSON   []byte
		manifestDig  string
	}

	entries := make([]tagEntry, 0, len(tags))
	// Collect in deterministic order (alphabetical, like keppel returns)
	sortedTags := make([]string, 0, len(tags))
	for tag := range tags {
		sortedTags = append(sortedTags, tag)
	}
	for i := 0; i < len(sortedTags)-1; i++ {
		for j := i + 1; j < len(sortedTags); j++ {
			if sortedTags[i] > sortedTags[j] {
				sortedTags[i], sortedTags[j] = sortedTags[j], sortedTags[i]
			}
		}
	}
	for _, tag := range sortedTags {
		_, mj, cj := fakeHelmChart(tag, tags[tag])
		dig := fmt.Sprintf("sha256:%x", sha256.Sum256(mj))
		entries = append(entries, tagEntry{tag: tag, manifestJSON: mj, configJSON: cj, manifestDig: dig})
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/v2/ccloud-helm/metal-maintenance-operator-remote/tags/list", func(w http.ResponseWriter, r *http.Request) {
		tagList := make([]string, len(entries))
		for i, e := range entries {
			tagList[i] = e.tag
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "ccloud-helm/metal-maintenance-operator-remote",
			"tags": tagList,
		})
	})

	mux.HandleFunc("/v2/ccloud-helm/metal-maintenance-operator-remote/manifests/", func(w http.ResponseWriter, r *http.Request) {
		ref := strings.TrimPrefix(r.URL.Path, "/v2/ccloud-helm/metal-maintenance-operator-remote/manifests/")
		for _, e := range entries {
			if e.tag == ref || e.manifestDig == ref {
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.Header().Set("Docker-Content-Digest", e.manifestDig)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(e.manifestJSON)
				return
			}
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/v2/ccloud-helm/metal-maintenance-operator-remote/blobs/", func(w http.ResponseWriter, r *http.Request) {
		blobDig := strings.TrimPrefix(r.URL.Path, "/v2/ccloud-helm/metal-maintenance-operator-remote/blobs/")
		for _, e := range entries {
			if fmt.Sprintf("sha256:%x", sha256.Sum256(e.configJSON)) == blobDig {
				w.Header().Set("Content-Type", "application/vnd.cncf.helm.config.v1+json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(e.configJSON)
				return
			}
		}
		http.NotFound(w, r)
	})

	return httptest.NewTLSServer(mux)
}

// TestCheckByTagRegex_HelmCharts_CreatedAtSort reproduces the bug where
// created_at_sort with Helm charts (no Created in config blob) causes the
// windowing to clip all new tags when the requested version sorts last
// alphabetically.
func TestCheckByTagRegex_HelmCharts_CreatedAtSort(t *testing.T) {
	now := time.Now().UTC()

	// Three dev tags in the order a registry returns them (alphabetical).
	// 0.0.0-aaaaaaaa is alphabetically last and is the "current" version.
	// 0.0.0-1aaaaaaa and 0.0.0-2aaaaaaa were pushed later but sort earlier alphabetically.
	srv := fakeRegistry(t, map[string]time.Time{
		"0.0.0-1aaaaaaa": now.Add(-2 * time.Hour),  // pushed 2h ago
		"0.0.0-2aaaaaaa": now.Add(-1 * time.Hour),  // pushed 1h ago (newest)
		"0.0.0-aaaaaaaa": now.Add(-24 * time.Hour), // original, pushed 1 day ago
		"1.1.8":          now.Add(-48 * time.Hour), // release tag, should be filtered out
	})
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "https://")

	// Build a repo client that trusts the test TLS cert
	repo, err := remote.NewRepository(fmt.Sprintf("%s/ccloud-helm/metal-maintenance-operator-remote", host))
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	repo.Client = &auth.Client{
		Client: srv.Client(), // test TLS client
		Cache:  auth.NewCache(),
	}
	repo.ManifestMediaTypes = allowedMediaTypes

	req := CheckRequest{
		Source: Source{
			Registry:      host,
			Repository:    "ccloud-helm",
			ChartName:     "metal-maintenance-operator-remote",
			TagRegex:      `^0\.0\.0-`,
			CreatedAtSort: true,
		},
		Version: &Version{
			Tag: "0.0.0-aaaaaaaa",
		},
	}

	resp, err := checkByTagRegex(context.Background(), req, repo)
	if err != nil {
		t.Fatalf("checkByTagRegex failed: %v", err)
	}

	tags := make([]string, len(*resp))
	for i, v := range *resp {
		tags[i] = v.Tag
	}

	// All three dev tags must be present (sorted ascending by created_at,
	// so aaaaaaaa first, then 1aaaaaaa, then 2aaaaaaa newest last).
	expected := []string{"0.0.0-aaaaaaaa", "0.0.0-1aaaaaaa", "0.0.0-2aaaaaaa"}
	if len(*resp) != len(expected) {
		t.Fatalf("expected %d versions, got %d: %v", len(expected), len(*resp), tags)
	}
	for i, want := range expected {
		if tags[i] != want {
			t.Errorf("position %d: expected %s, got %s (all: %v)", i, want, tags[i], tags)
		}
	}
}
