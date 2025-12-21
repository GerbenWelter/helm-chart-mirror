package main

import (
	"log"

	"helm-chart-mirror/config"
	"helm-chart-mirror/helm"
)

func main() {
	log.Println("INFO: starting helm-chart-mirror")
	helmChartMirrorConfig := config.LoadConfig()
	config.LoadOCICredentials()

	helm.MirrorHelmCharts(helmChartMirrorConfig)
}
