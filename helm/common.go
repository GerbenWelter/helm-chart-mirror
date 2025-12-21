package helm

import (
	"log"
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	gologging "gopkg.in/op/go-logging.v1"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/registry"
)

var helmConfig = cli.New()

func helmClient() (*action.Configuration, error) {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	actionConfig := action.Configuration{
		RegistryClient: registryClient,
	}

	return &actionConfig, nil
}

func dedupImages(chartImages []string) []string {
	var allChartImages []string
	seen := make(map[string]bool)
	for _, img := range chartImages {
		if !seen[img] {
			allChartImages = append(allChartImages, img)
			seen[img] = true
		}
	}
	return allChartImages
}

func collectAllImages(outputManifest string) []string {
	// set yqlib logging to 'WARNING'
	gologging.SetLevel(gologging.WARNING, "yq-lib")

	yqPrefs := yqlib.NewDefaultYamlPreferences()
	yqPrefs.PrintDocSeparators = false
	yqDecoder := yqlib.NewYamlDecoder(yqPrefs)
	yqEncoder := yqlib.NewYamlEncoder(yqPrefs)
	yqFilter := `select(.kind == "Deployment" or .kind == "DeploymentConfig" or .kind == "StatefulSet" or .kind == "DaemonSet" or .kind == "CronJob" or .kind == "Job") | .. | select(has("image")) | .image`
	allImages, err := yqlib.NewStringEvaluator().EvaluateAll(yqFilter, outputManifest, yqEncoder, yqDecoder)
	if err != nil {
		log.Fatalf("ERROR: unable to parse Helm templated output for images: %s\n", err)
	}

	splitImages := strings.Split(allImages, "\n")

	var images []string
	for _, img := range splitImages {
		if img != "" {
			images = append(images, img)
		}
	}

	return images
}
