// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudoperators/concourse-oci-helm-chart-resource/pkg/resource"
)

func main() {
	var req resource.CheckRequest

	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		log.Fatalf("failed to unmarshal request: %s", err)
	}

	if err := req.Validate(); err != nil {
		log.Fatalf("invalid source configuration: %s", err)
	}

	resp, err := resource.Check(context.Background(), req)
	if err != nil {
		log.Fatalf("invalid source configuration: %s", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("failed to marshal response: %s", err)
	}
}
