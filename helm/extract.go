package helm

import (
	"log"

	"helm-chart-mirror/client"
	"helm-chart-mirror/config"

	"helm.sh/helm/v4/pkg/action"
	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release"
	"k8s.io/client-go/discovery"
)

func extractChartImages(helmChartMirrorConfig config.Config, loadedChart *v2.Chart, tc map[string]any) []string {
	actionConfig := new(action.Configuration)
	install := action.NewInstall(actionConfig)
	install.DryRunStrategy = "client"
	install.ReleaseName = "helm-chart-mirror"
	install.Namespace = "helm-chart-mirror"

	// if not specified in the config use the kubernetes version of the cluster for the charts that depend on it
	if helmChartMirrorConfig.KubernetesVersion != "" {
		install.APIVersions = []string{helmChartMirrorConfig.KubernetesVersion}
	} else {
		// get current Kubernetes version
		kubeConfig, _ := client.GetKubeConfig()
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
		if err != nil {
			log.Fatalf("ERROR: unable to connect to Kubernetes: %s\n", err)
		}
		kubeVersion, err := discoveryClient.ServerVersion()
		if err != nil {
			log.Fatalf("ERROR: unable to retrieve cluster server version: %s", err)
		}

		install.APIVersions = []string{kubeVersion.String()}
	}

	releaser, err := install.Run(loadedChart, tc)
	accessor, err := release.NewAccessor(releaser)
	if err != nil {
		log.Fatalf("ERROR: unable to get templated manifests: %s\n", err)
	}
	manifests := accessor.Manifest()

	completeManifest := manifests

	// also get manifests from helm chart hooks
	completeManifest += accessor.Manifest()

	return collectAllImages(completeManifest)
}
