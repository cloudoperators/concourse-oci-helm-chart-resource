// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

type (
	CheckRequest struct {
		Source  Source   `json:"source"`
		Version *Version `json:"version,omitempty"`
	}

	CheckResponse []Version
)

func (cr *CheckRequest) Validate() error {
	return cr.Source.Validate()
}

func Check(ctx context.Context, request CheckRequest) (*CheckResponse, error) {
	repo, err := newRepositoryForSource(ctx, request.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create repository client")
	}
	if request.Source.Tag != "" {
		return checkBySingleTag(ctx, request, repo)
	}
	if request.Source.TagRegex != "" {
		return checkByTagRegex(ctx, request, repo)
	}
	return checkBySemver(ctx, request, repo)
}

// checkBySemver is the original check implementation: fetches all tags, parses
// them as semver, sorts ascending, and trims at the requested version or returns
// the last 10.
func checkBySemver(ctx context.Context, request CheckRequest, repo *remote.Repository) (*CheckResponse, error) {
	allTags, err := registry.Tags(ctx, repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch tags")
	}

	// Sort tags by semver
	sortedSemvers := sortBySemver(allTags)

	// chop the list at the index of the requested version, if there was one
	if request.Version != nil {
		requestedVersion, err := semver.NewVersion(request.Version.Tag)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to parse semver in requested version %q", request.Version.Tag))
		}
		for i, version := range sortedSemvers {
			if version.GreaterThanEqual(requestedVersion) {
				sortedSemvers = sortedSemvers[i:]
				break
			}
		}
	} else {
		// if no version was requested, return the latest 10 versions
		sortedSemvers = sortedSemvers[max(0, len(sortedSemvers)-10):]
	}

	if len(sortedSemvers) == 0 {
		return nil, fmt.Errorf("no latest tag found for source %s", request.Source.String())
	}

	resolvedVersions, err := resolveImageDigests(ctx, sortedSemvers, repo)
	if err != nil {
		return nil, err
	}
	return &resolvedVersions, nil
}

// checkBySingleTag returns a CheckResponse containing exactly one version for the
// tag specified in source.Tag.
func checkBySingleTag(ctx context.Context, request CheckRequest, repo *remote.Repository) (*CheckResponse, error) {
	digest, err := getDigestForTag(ctx, repo, request.Source.Tag)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to fetch digest for tag %q", request.Source.Tag))
	}
	return &CheckResponse{Version{Tag: request.Source.Tag, Digest: digest}}, nil
}

// checkByTagRegex filters repository tags by source.TagRegex, optionally sorts
// them by OCI image creation time (source.CreatedAtSort), then applies the same
// windowing logic as the semver path (trim at requested version or return last 10).
func checkByTagRegex(ctx context.Context, request CheckRequest, repo *remote.Repository) (*CheckResponse, error) {
	re, err := regexp.Compile(request.Source.TagRegex)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to compile tag_regex %q", request.Source.TagRegex))
	}

	tags, err := registry.Tags(ctx, repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch tags")
	}

	// Filter by regex
	matched := make([]string, 0, len(tags))
	for _, t := range tags {
		if re.MatchString(t) {
			matched = append(matched, t)
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no tags matched tag_regex %q in source %s", request.Source.TagRegex, request.Source.String())
	}

	// Sort by creation timestamp if requested, otherwise keep registry order
	if request.Source.CreatedAtSort {
		times := make([]time.Time, len(matched))
		g, gctx := errgroup.WithContext(ctx)
		for i, tag := range matched {
			g.Go(func() error {
				ts, err := getCreatedAtForTag(gctx, repo, tag)
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("failed to get created_at for tag %q", tag))
				}
				if ts != nil {
					times[i] = *ts
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		tagToTime := make(map[string]time.Time, len(matched))
		for i, tag := range matched {
			tagToTime[tag] = times[i]
		}
		slices.SortStableFunc(matched, func(a, b string) int {
			return tagToTime[a].Compare(tagToTime[b])
		})
	}

	// Trim at requested version (by tag equality), same windowing as semver path
	if request.Version != nil {
		for i, t := range matched {
			if t == request.Version.Tag {
				matched = matched[i:]
				break
			}
		}
	} else {
		matched = matched[max(0, len(matched)-10):]
	}

	resolved, err := resolveTagDigests(ctx, matched, repo)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func sortBySemver(allTags []string) []semver.Version {
	allVersions := make([]semver.Version, len(allTags))
	j := 0
	for _, tag := range allTags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping tag %q because it does not look like a semver\n", tag)
			continue
		}
		allVersions[j] = *v
		j++
	}
	slices.SortStableFunc(allVersions, func(i, j semver.Version) int {
		return i.Compare(&j)
	})
	return allVersions
}

func resolveImageDigests(ctx context.Context, sortedSemvers []semver.Version, repo *remote.Repository) (CheckResponse, error) {
	resolvedVersions := make(CheckResponse, len(sortedSemvers))
	for i, version := range sortedSemvers {
		digest, err := getDigestForTag(ctx, repo, version.Original())
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to fetch digest for latest tag %q (parsed as %s)", version.Original(), version.String()))
		}
		resolvedVersions[i] = Version{Tag: version.Original(), Digest: digest}
	}
	return resolvedVersions, nil
}

// resolveTagDigests resolves OCI digests for a list of raw tag strings.
func resolveTagDigests(ctx context.Context, tags []string, repo *remote.Repository) (CheckResponse, error) {
	resolved := make(CheckResponse, len(tags))
	for i, tag := range tags {
		digest, err := getDigestForTag(ctx, repo, tag)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to fetch digest for tag %q", tag))
		}
		resolved[i] = Version{Tag: tag, Digest: digest}
	}
	return resolved, nil
}
