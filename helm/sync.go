package helm

import (
	"log"

	"helm-chart-mirror/config"
)

func MirrorHelmCharts(helmChartMirrorConfig config.Config) {
	for _, repo := range helmChartMirrorConfig.Repositories {
		for _, chart := range repo.Charts {
			helmChart, err := loadHelmChart(repo, chart, helmChartMirrorConfig.TmpDir)
			if err != nil {
				log.Printf("ERROR: unable to load Helm chart '%s/%s', skipping!\n", repo.Name, chart.Name)
				continue
			}

			log.Println("INFO: getting all images used by chart based on supplied template configurations")
			var allChartImages []string
			if len(chart.TemplateConfigurations) == 0 {
				chartImages := extractChartImages(helmChartMirrorConfig, helmChart, make(map[string]any))
				allChartImages = dedupImages(chartImages)
			} else {
				for _, tc := range chart.TemplateConfigurations {
					chartImages := extractChartImages(helmChartMirrorConfig, helmChart, tc)
					allChartImages = append(allChartImages, chartImages...)
				}
				allChartImages = dedupImages(allChartImages)
			}
		}
	}
}
