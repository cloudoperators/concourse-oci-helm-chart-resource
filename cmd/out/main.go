// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudoperators/concourse-oci-helm-chart-resource/pkg/resource"
)

func main() {
	var req resource.PutRequest

	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %s\n", err)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing arguments")
	}
	if err := req.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid source configuration: %s\n", err)
	}
	inputDir := os.Args[1]
	response, err := resource.Put(context.Background(), req, inputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get failed: %s\n", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %s\n", err)
	}
}
