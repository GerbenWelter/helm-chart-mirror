package main

import (
	"log"

	"helm-chart-mirror/config"
	"helm-chart-mirror/helm"
)

func main() {
	log.Println("INFO: starting helm-chart-mirror")
	config := config.LoadConfig()

	helm.MirrorHelmCharts(config)
}
