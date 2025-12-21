package helm

import (
	"fmt"
	"log"
	"strings"

	"helm.sh/helm/v4/pkg/action"
)

func pullHelmChartFile(repository, chartName, chartVersion, tmpDir string) (string, error) {
	var chartFile string
	actionConfig, err := helmClient()
	if err != nil {
		log.Fatalf("ERROR: unable to create a helm registry client (%s)", err)
	}

	if strings.HasPrefix(repository, "oci://") {
		pull := action.NewPull(action.WithConfig(actionConfig))
		pull.DestDir = tmpDir
		pull.Settings = helmConfig
		pull.Version = chartVersion

		_, err := pull.Run(fmt.Sprintf("%s/%s", repository, chartName))
		if err != nil {
			log.Printf("ERROR: unable to pull helm chart from %s/%s (%s)", repository, chartName, err)
			return "", err
		}
		chartFile = fmt.Sprintf("%s/%s-%s.tgz", pull.DestDir, chartName, chartVersion)
	} else {

		chartOptions := action.ChartPathOptions{
			RepoURL: repository,
			Version: chartVersion,
		}

		cf, err := chartOptions.LocateChart(chartName, helmConfig)
		if err != nil {
			log.Printf("ERROR: unable to pull chart '%s' from '%s' (%s)", chartName, repository, err)
			return "", err
		}
		chartFile = cf
	}

	return chartFile, nil
}
