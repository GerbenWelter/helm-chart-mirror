package helm

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"

	"helm-chart-mirror/config"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

			for _, image := range allChartImages {
				SyncImage(image, helmChartMirrorConfig)
			}
		}
	}
}

func SyncImage(image string, helmChartMirrorConfig config.Config) {
	s := strings.SplitN(image, ":", 2)
	img := s[0]
	tagDigest := s[1]
	tag := strings.Split(tagDigest, "@")[0]
	if !strings.Contains(img, ".") {
		img = "docker.io/" + img
	}
	r := strings.SplitN(img, "/", 2)
	sourceRegistry := r[0]
	sourceRepository := r[1]
	destinationRepository := sourceRepository

	log.Printf("INFO: syncing '%s/%s:%s' to '%s/%s/%s/%s:%s'", sourceRegistry, sourceRepository, tagDigest, helmChartMirrorConfig.DestinationRegistry, helmChartMirrorConfig.DestinationRepository, sourceRegistry, destinationRepository, tag)

	// oras.Copy() doesn't know how to handle library images like docker.io/memcached
	if !strings.Contains(sourceRepository, "/") {
		sourceRepository = "library/" + sourceRepository
	}

	srcReg, err := remote.NewRegistry(sourceRegistry)
	if err != nil {
		log.Fatalf("ERROR: unable to execute NewRegistry() for source registry '%s'", sourceRegistry)
	}

	srcRegistryCreds, err := config.OCICredentials.Get(context.Background(), sourceRegistry)

	srcReg.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(helmChartMirrorConfig.DestinationRegistry, auth.Credential{
			Username: srcRegistryCreds.Username,
			Password: srcRegistryCreds.Password,
		}),
	}

	source, err := srcReg.Repository(context.Background(), sourceRepository)
	if err != nil {
		log.Fatalf("ERROR: unable to execute Repository() for source repository '%s'", sourceRepository)
	}

	destReg, err := remote.NewRegistry(helmChartMirrorConfig.DestinationRegistry)
	if err != nil {
		log.Fatalf("ERROR: unable to execute NewRegistry() for destination registry '%s'", helmChartMirrorConfig.DestinationRegistry)
	}

	destRegistryCreds, err := config.OCICredentials.Get(context.Background(), helmChartMirrorConfig.DestinationRegistry)
	destReg.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(helmChartMirrorConfig.DestinationRegistry, auth.Credential{
			Username: destRegistryCreds.Username,
			Password: destRegistryCreds.Password,
		}),
	}

	dest, err := destReg.Repository(context.Background(), fmt.Sprintf("%s/%s/%s", helmChartMirrorConfig.DestinationRepository, sourceRegistry, destinationRepository))
	if err != nil {
		log.Fatalf("ERROR: unable to execute Repository() for destination repository '%s'", sourceRepository)
	}

	// Check if image already exists in the destination repository
	destRepoURL := fmt.Sprintf("%s/%s/%s/%s", helmChartMirrorConfig.DestinationRegistry, helmChartMirrorConfig.DestinationRepository, sourceRegistry, destinationRepository)
	repo, err := remote.NewRepository(destRepoURL)
	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(helmChartMirrorConfig.DestinationRegistry, auth.Credential{
			Username: destRegistryCreds.Username,
			Password: destRegistryCreds.Password,
		}),
	}
	if err != nil {
		log.Fatalf("ERROR: unable setup connection to '%s'", destRepoURL)
	}

	reference := fmt.Sprintf("%s:%s", destRepoURL, tag)

	_, err = repo.Resolve(context.Background(), reference)
	if err != nil {
		// Use current platform unless overridden
		copyOptions := oras.DefaultCopyOptions
		if helmChartMirrorConfig.OverridePlatform != "" {
			platform := strings.Split(helmChartMirrorConfig.OverridePlatform, "/")
			platformOS := platform[0]
			platformArch := platform[1]
			copyOptions.WithTargetPlatform(&ocispec.Platform{
				OS:           platformOS,
				Architecture: platformArch,
			})
		} else {
			copyOptions.WithTargetPlatform(&ocispec.Platform{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			})
		}

		_, err = oras.Copy(context.Background(), source, tagDigest, dest, tag, copyOptions)
		if err != nil {
			log.Printf("ERROR: unable to copy image from '%s/%s:%s' to '%s/%s/%s:%s' (%s)", sourceRegistry, sourceRepository, tagDigest, helmChartMirrorConfig.DestinationRegistry, helmChartMirrorConfig.DestinationRepository, destinationRepository, tag, err)
		}
	} else {
		log.Printf("INFO: skipping, image '%s' already exists", reference)
	}
}
