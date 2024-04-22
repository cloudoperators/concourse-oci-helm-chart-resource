// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"

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
	// 	Set up credentials from docker.
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
