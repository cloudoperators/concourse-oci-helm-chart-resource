// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"oras.land/oras-go/v2/registry"
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
		return nil, err
	}

	// Fetching repository tags
	allTags, err := registry.Tags(ctx, repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch tags")
	}

	// Sorting tags.
	var latestTag *semver.Version
	for _, tag := range allTags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			return nil, err
		}
		if latestTag == nil || v.GreaterThan(latestTag) {
			latestTag = v
		}
	}
	if latestTag == nil {
		return nil, fmt.Errorf("no latest tag found for source %s", request.Source.String())
	}

	digest, err := getDigestForTag(ctx, repo, latestTag.String())
	if err != nil {
		return nil, err
	}
	return &CheckResponse{{Tag: latestTag.String(), Digest: digest}}, nil
}
