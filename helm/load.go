package helm

import (
	"helm-chart-mirror/config"
	"log"

	v2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

func loadHelmChart(repo config.Repository, chartDefinition config.Chart, tmpdir string) (*v2.Chart, error) {
	chartFile, err := pullHelmChartFile(repo.Source, chartDefinition.Name, chartDefinition.Version, tmpdir)
	if err != nil {
		log.Printf("ERROR: unable to load Helm Chart '%s' (%s)", chartDefinition.Name, err)
	}

	log.Printf("INFO: loading chart for: %s\n", chartDefinition.Name)
	helmChart, err := loader.LoadFile(chartFile)

	if err != nil {
		log.Fatalf("ERROR: Unable to load chart from '%s'\n", chartFile)
		return nil, err
	}
	return helmChart, nil
}
