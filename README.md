Concourse resource for artifacts in an OCI registry
===================================================
[![REUSE status](https://api.reuse.software/badge/github.com/cloudoperators/concourse-oci-helm-chart-resource)](https://api.reuse.software/info/github.com/cloudoperators/concourse-oci-helm-chart-resource)

Fetches, verifies and publishes Helm Charts from a running OCI registry.

## Installation

Add a new resource type to your Concourse CI pipeline:

```yaml
resource_types:
- name: oci-registry
  type: docker-image
  source:
    repository: ghcr.io/cloudoperators/concourse-oci-helm-chart-resource
    tag: f932b76 # Replace with the latest stable release.
```

## Configuration

```
resources:
  - name: my.chart
    type: oci-registry
    source:
      registry: ghcr.io
      repository: cloudoperators/all-my-helm-charts
      chart_name: my-chart
```

### Source Parameters

| Parameter         | Type   | Required | Description                                                                                                                                            |
| ----------------- | ------ | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `registry`        | string | yes      | OCI registry hostname (e.g. `ghcr.io`).                                                                                                                |
| `repository`      | string | yes      | Repository path within the registry (e.g. `my-org/my-charts`).                                                                                         |
| `chart_name`      | string | yes      | Name of the Helm chart to check/download.                                                                                                              |
| `auth_username`   | string | optional | Username for basic authentication. Requires `auth_password` to be set.                                                                                 |
| `auth_password`   | string | optional | Password for basic authentication. Requires `auth_username` to be set.                                                                                 |
| `tag`             | string | optional | A specific tag to check. Mutually exclusive with `tag_regex`.                                                                                          |
| `tag_regex`       | string | optional | A regular expression to filter tags by partial or full match (e.g. `^0.0.0-` matches all pre-release builds). When set, semver sorting is skipped. |
| `created_at_sort` | bool   | optional | Sort matched tags by OCI image creation timestamp (ascending, newest last). Requires `tag_regex` or `tag` to be set.                                   |

#### Authentication

The resource supports two ways of authenticating against the registry:
* By default, the docker credential store is used.
* Use the `auth_username` and `auth_password` parameters within the `source` block for basic authentication.

## Behavior

The resource implements the `check` and `in` action.

### check: Check for new versions of the Helm chart

Checks for new versions of the specified Helm chart. Returns the latest version of the Helm chart based on semantic versioning.

### in: Download the Helm chart and the metadata file

Places the packaged Helm chart and the metadata file in the destination directory following the `<$>chart_name>-<chart_version>.{tgz|json}` naming convention.

### out

* Currently not supported. Use `helm push` to push Helm charts to an OCI registry.

## Examples

### Track pre-release builds sorted by creation time

```yaml
resources:
  - name: my-chart-prerelease
    type: oci-registry
    source:
      registry: ghcr.io
      repository: my-org/my-charts
      chart_name: my-chart
      tag_regex: "^0.0.0-"
      created_at_sort: true
```

This example uses `tag_regex` to filter for pre-release tags matching the pattern `^0.0.0-` and sorts them by OCI image creation timestamp (ascending, newest last). Note that `tag_regex` uses Go's `regexp.MatchString` which performs partial/substring matching — use `^` and `$` anchors as needed for full match patterns.
