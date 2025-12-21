package helm

import (
	"fmt"
	"helm-chart-mirror/config"
	"log"
)

func loadHelmChart(repo config.Repository, chartDefinition config.Chart, tmpdir string) {
	chartFile, err := pullHelmChartFile(repo.Source, chartDefinition.Name, chartDefinition.Version, tmpdir)
	if err != nil {
		log.Printf("ERROR: unable to load Helm Chart '%s' (%s)", chartDefinition.Name, err)
	}

	fmt.Printf("%+v\n", chartFile)
}
