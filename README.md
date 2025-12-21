# helm-chart-mirror

## Preface

This is the repository for `helm-chart-mirror`, a tool for mirroring Helm charts and their container images to another OCI-compliant registry. This is useful for environments that are air-gapped and/or don't want to depend on availability of external resources.

## Features

Helm-chart-mirror has the following features:

- Can pull from classic Helm or OCI-compliant registries.
- Mirror container images used by Helm chart.
- Supports multiple sets values for configurations that might otherwise conflict.
- Supports authentication to destination registry.

## Future features

- Source registry authentication.
- Multi-arch container image syncing if possible. Currently supports mirroring a single architecture only.
