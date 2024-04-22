// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
)

type (
	PutRequest  struct{}
	PutResponse struct{}
)

func (pr *PutRequest) Validate() error {
	return nil
}

func Put(ctx context.Context, request PutRequest, inputDir string) (*PutResponse, error) {
	return nil, errors.New("not implemented")
}
