package helm

import (
	"helm-chart-mirror/config"
)

func MirrorHelmCharts(helmChartMirrorConfig config.Config) {
	for _, repo := range helmChartMirrorConfig.Repositories {
		for _, chart := range repo.Charts {
			loadHelmChart(repo, chart, helmChartMirrorConfig.TmpDir)
		}
	}
}
