// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"oras.land/oras-go/v2/registry"
)

type (
	CheckRequest struct {
		Source *Source `json:"source"`
	}

	CheckResponse struct {
		Source  *Source `json:"source"`
		Version string  `json:"version"`
		Digest  string  `json:"digest"`
	}
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
	tags, err := registry.Tags(ctx, repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch tags")
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found for source %s", request.Source.String())
	}

	// Sorting tags. The latest tag is the last one
	semver.Sort(tags)
	latestTag := tags[len(tags)-1]

	digest, err := getDigestForTag(ctx, repo, latestTag)
	if err != nil {
		return nil, err
	}
	return &CheckResponse{
		Source:  request.Source,
		Version: latestTag,
		Digest:  digest,
	}, nil
}
