# helm-chart-mirror

## Preface

This is the repository for `helm-chart-mirror`, a tool for mirroring Helm charts and their container images to another OCI-compliant registry. This is useful for environments that are air-gapped and/or don't want to depend on the availability of external resources.

## Features

Helm-chart-mirror has the following features:

- Can pull from classic Helm or OCI-compliant registries.
- Mirror container images used by Helm chart.
- Supports multiple sets values for configurations that might otherwise conflict.
- Supports authentication to destination registry.

## Future features

- Source registry authentication (are there any public?).
- Multi-arch container image syncing if possible. Currently supports mirroring a single architecture only.

## Usage

Helm-chart-mirror when compiled to a single binary can be used in scripting or in a cronjob. It's configured through a `config.yaml`. The default location is `/etc/helm-chart-mirror/config.yaml` but it can be overridden with the `HELM_CHART_MIRROR_CONFIG` environment variable. Helm-chart-mirror can be configured as following:

```yaml
destinationRegistry: myregistry.example.com:5043 # if no port is specified it will default to 443
destinationRepository: mirror # prefix used relative to the root of the registry
kubernetesVersion: '1.33' # if not present will use kubernetes cluster version
overridePlatform: linux/amd64 # if not present it will default to the platform used
repositories:
  - name: cert-manager # name for repository
    source: oci://ghcr.io/cert-manager/charts # Helm chart source is in a OCI-compliant registry
    charts:
      - name: openshift-routes # name of helm chart
        version: 0.8.4 # version of helm chart
  - name: grafana
    source: https://grafana.github.io/helm-charts # Helm chart in a classic style registry
    charts:
      - name: loki
        version: 6.48.0
        templateConfigurations: # list template configurations
          - loki:
              useTestSchema: true
              storage:
                bucketNames:
                  admin: dummy
                  chunks: 1
            enterprise:
              enabled: true
              adminToken:
                secret: dummy
            minio:
              enabled: true
            sidecar:
              rules:
                enabled: true
```

Template configurations can be specified multiple times to get different outputs. This is useful when charts can be used in different but conflicting ways or to activate additional container images. Helm-chart-mirror will mirror the combination of used container images. Contents is the same as as if the configuration was supplied in a `values.yaml` file.

Images and helm charts are mirrored taking the original repistry and repository in account for clarity of origin. E.g. the 'openshift-routes' helm chart uses the following image:

'ghcr.io/cert-manager/cert-manager-openshift-routes:v0.8.4'

Then the helm chart and image will be synced as following:

image: `myregistry.example.com:5043/mirror/ghcr.io/cert-manager/cert-manager-openshift-routes:v0.8.4`
chart: `myregistry.example.com:5043/mirror/charts/cert-manager/openshift-routes:0.8.4`

Charts are mirrored to their own 'charts' subdir to prevent name conflicts with a image used in the chart.
