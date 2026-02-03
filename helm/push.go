package helm

import (
	"context"
	"fmt"
	"helm-chart-mirror/config"
	"log"
	"os"
	"slices"

	"helm.sh/helm/v4/pkg/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func pushChartFileToRegistry(chartFile, repoName, chartName, chartVersion string, helmChartMirrorConfig config.Config) {
	ctx := context.Background()
	log.Println("INFO: reading Helm Chart file")

	chartData, err := os.ReadFile(chartFile)
	if err != nil {
		log.Fatalf("ERROR: unable to read chart file (%s)", err)
	}

	repository := fmt.Sprintf(
		"%s/%s/charts/%s/%s",
		helmChartMirrorConfig.DestinationRegistry,
		helmChartMirrorConfig.DestinationRepository,
		repoName,
		chartName,
	)
	chartRef := fmt.Sprintf("%s:%s", repository, chartVersion)

	repo, err := remote.NewRepository(repository)
	if err != nil {
		log.Fatalf("ERROR: unable to create ORAS repository: %v", err)
	}

	creds, err := config.OCICredentials.Get(ctx, helmChartMirrorConfig.DestinationRegistry)
	if err != nil {
		log.Fatalf("ERROR: unable to get registry credentials: %v", err)
	}

	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(
			helmChartMirrorConfig.DestinationRegistry,
			auth.Credential{
				Username: creds.Username,
				Password: creds.Password,
			},
		),
	}

	var existingTags []string
	err = repo.Tags(ctx, "", func(batch []string) error {
		existingTags = append(existingTags, batch...)
		return nil
	})
	if err != nil {
		log.Fatalf("ERROR: unable to list tags: %v", err)
	}

	if slices.Contains(existingTags, chartVersion) {
		log.Printf("INFO: helm chart '%s' already exists\n", chartRef)
		return
	}

	helmRegistryClient, err := registry.NewClient()
	if err != nil {
		log.Fatal("ERROR: Unable to create registry client", err)
	}

	err = helmRegistryClient.Login(helmChartMirrorConfig.DestinationRegistry, registry.LoginOptBasicAuth(creds.Username, creds.Password))
	if err != nil {
		log.Printf("ERROR: authentication to %s failed", helmChartMirrorConfig.DestinationRegistry)
	}

	log.Printf("INFO: pushing helm chart to '%s'\n", chartRef)
	// Because of the above semver issue we must also disable Strict Mode when pushing.
	strictMode := registry.PushOptStrictMode(false)
	_, err = helmRegistryClient.Push(chartData, chartRef, strictMode)
	if err != nil {
		log.Printf("ERROR: unable to push chart to repository! (%s)", err)
	}
}
