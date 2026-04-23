// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Greenhouse contributors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"errors"
	"fmt"
)

type Source struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	ChartName  string `json:"chart_name"`

	Tag           string `json:"tag,omitempty"`
	TagRegex      string `json:"tag_regex,omitempty"`
	CreatedAtSort bool   `json:"created_at_sort,omitempty"`

	AuthUsername string `json:"auth_username,omitempty"`
	AuthPassword string `json:"auth_password,omitempty"`
}

func (s *Source) Validate() error {
	if s.Registry == "" {
		return errors.New("registry cannot be empty")
	}
	if s.Repository == "" {
		return errors.New("repository cannot be empty")
	}
	if s.ChartName == "" {
		return errors.New("chart_name cannot be empty")
	}
	if s.Tag != "" && s.TagRegex != "" {
		return errors.New("tag and tag_regex are mutually exclusive")
	}
	if s.CreatedAtSort && s.TagRegex == "" {
		return errors.New("created_at_sort requires tag_regex to be set")
	}
	return nil
}

func (s *Source) String() string {
	return fmt.Sprintf("%s/%s/%s", s.Registry, s.Repository, s.ChartName)
}

type Version struct {
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}
