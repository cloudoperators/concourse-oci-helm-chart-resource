// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

var allowedMediaTypes = []string{
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json, ",
	"application/vnd.oci.image.index.v1+json",
	"*/*",
}

func newRepositoryForSource(_ context.Context, s Source) (*remote.Repository, error) {
	repo, err := remote.NewRepository(s.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create repository from source %s", s.String())
	}
	repo.ManifestMediaTypes = allowedMediaTypes

	client := &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
	}
	// Set up credentials from docker.
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		panic(err)
	}
	client.Credential = credentials.Credential(credStore)

	// Set up basic auth credentials.
	if s.AuthUsername != "" && s.AuthPassword != "" {
		client.Credential = auth.StaticCredential(s.Registry, auth.Credential{
			Username: s.AuthUsername,
			Password: s.AuthPassword,
		})
	}
	repo.Client = client
	return repo, nil
}

func getDigestForTag(ctx context.Context, repo *remote.Repository, tag string) (string, error) {
	desc, err := repo.Resolve(ctx, fmt.Sprintf("%s:%s", repo.Reference.String(), tag))
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}

// getCreatedAtForTag fetches the OCI image config for the given tag and returns
// the creation timestamp recorded in it. Returns nil if the config has no
// Created field set.
func getCreatedAtForTag(ctx context.Context, repo *remote.Repository, tag string) (*time.Time, error) {
	// FetchReference goes directly to the manifests endpoint for the tag.
	_, manifestReader, err := repo.FetchReference(ctx, tag)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch manifest for tag %q", tag)
	}
	defer manifestReader.Close()

	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to parse manifest for tag %q", tag)
	}

	// Fetch the config blob by digest via the blobs endpoint.
	configReader, err := repo.Blobs().Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch config blob for tag %q", tag)
	}
	defer configReader.Close()

	var image ocispec.Image
	if err := json.NewDecoder(configReader).Decode(&image); err != nil {
		return nil, errors.Wrapf(err, "failed to parse image config for tag %q", tag)
	}

	return image.Created, nil
}
