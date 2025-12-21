package helm

import (
	"context"
	"fmt"
	"helm-chart-mirror/config"
	"log"
	"os"
	"slices"
	"strings"

	"helm.sh/helm/v4/pkg/registry"
)

func pushChartFileToRegistry(chartFile, repoName, chartName, chartVersion string, helmChartMirrorConfig config.Config) {
	log.Println("INFO: reading Helm Chart file")
	chartData, err := os.ReadFile(chartFile)
	if err != nil {
		log.Fatalf("ERROR: unable to read chart file (%s)", err)
	}

	// We need to strip the 'v' from 'v1.2.3' because Helm has not been consistently enforcing SemVer in all their subcommands.
	// See: https://github.com/helm/helm/issues/11107
	//      https://github.com/helm/helm/issues/12403
	// This is still not fixed in Helm v4.
	semVerVersion := strings.Replace(chartVersion, "v", "", 1)
	chartRef := fmt.Sprintf("%s/%s/charts/%s/%s:%s", helmChartMirrorConfig.DestinationRegistry, helmChartMirrorConfig.DestinationRepository, repoName, chartName, semVerVersion)

	destRegistryCreds, err := config.OCICredentials.Get(context.Background(), helmChartMirrorConfig.DestinationRegistry)
	helmRegistryClient, err := registry.NewClient()
	if err != nil {
		log.Fatal("ERROR: Unable to create registry client", err)
	}

	err = helmRegistryClient.Login(helmChartMirrorConfig.DestinationRegistry, registry.LoginOptBasicAuth(destRegistryCreds.Username, destRegistryCreds.Password))
	if err != nil {
		log.Printf("ERROR: authentication to %s failed", helmChartMirrorConfig.DestinationRegistry)
	}

	chartTags, err := helmRegistryClient.Tags(chartRef)
	if err != nil {
		log.Printf("INFO: helm chart '%s/%s' is not present\n", repoName, chartName)
	}

	if slices.Contains(chartTags, semVerVersion) {
		log.Printf("INFO: helm chart '%s' is already present\n", chartRef)
	} else {
		log.Printf("INFO: pushing helm chart to '%s'\n", chartRef)
		// Because of the above semver issue we must also disable Strict Mode when pushing.
		strictMode := registry.PushOptStrictMode(false)
		_, err = helmRegistryClient.Push(chartData, chartRef, strictMode)
		if err != nil {
			log.Printf("ERROR: unable to push chart to repository! (%s)", err)
		}
	}
}
