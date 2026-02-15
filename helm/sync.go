package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"

	"helm-chart-mirror/config"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func MirrorHelmCharts(helmChartMirrorConfig config.Config) {
	for _, repo := range helmChartMirrorConfig.Repositories {
		for _, chart := range repo.Charts {
			helmChart, chartFile, err := loadHelmChart(repo, chart, helmChartMirrorConfig.TmpDir)
			if err != nil {
				continue
			}

			log.Println("INFO: getting all images used by chart based on supplied template configurations")
			var allChartImages []string
			if len(chart.TemplateConfigurations) == 0 {
				chartImages := extractChartImages(helmChartMirrorConfig, repo, chart, helmChart, make(map[string]any))
				allChartImages = dedupImages(chartImages)
			} else {
				for _, tc := range chart.TemplateConfigurations {
					chartImages := extractChartImages(helmChartMirrorConfig, repo, chart, helmChart, tc)
					allChartImages = append(allChartImages, chartImages...)
				}
				allChartImages = dedupImages(allChartImages)
			}

			for _, image := range allChartImages {
				SyncImage(image, helmChartMirrorConfig, chart)
			}

			pushChartFileToRegistry(chartFile, repo.Name, chart.Name, chart.Version, helmChartMirrorConfig)
		}
	}
}

func SyncImage(image string, helmChartMirrorConfig config.Config, chart config.Chart) {
	log.Printf("DEBUG: started processing  '%s'", image)

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

	// Oras fails to pull 'library' images on docker.io which omit the library part
	// e.g. docker.io/busybox. Tools like podman have similar workarounds.
	if strings.Contains(sourceRegistry, "docker.io") && !strings.Contains(sourceRepository, "/") {
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

	parts := []string{}
	if helmChartMirrorConfig.ImageDestinationRepository != "" {
		parts = append(parts, helmChartMirrorConfig.ImageDestinationRepository)
	}
	if helmChartMirrorConfig.IncludeOriginalImageRegistry {
		parts = append(parts, sourceRegistry)
	}
	parts = append(parts, destinationRepository)
	dest := strings.Join(parts, "/")

	destinationRepositoryUrl := fmt.Sprintf("%s/%s", helmChartMirrorConfig.DestinationRegistry, dest)

	repo, err := remote.NewRepository(destinationRepositoryUrl)
	if err != nil {
		log.Fatalf("ERROR: unable setup connection to '%s'", destinationRepositoryUrl)
	}
	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(helmChartMirrorConfig.DestinationRegistry, auth.Credential{
			Username: destRegistryCreds.Username,
			Password: destRegistryCreds.Password,
		}),
	}

	reference := fmt.Sprintf("%s:%s", destinationRepositoryUrl, tag)
	// destRepo, err := destReg.Repository(context.Background(), dest)
	destRepo, err := remote.NewRepository(fmt.Sprintf("%s:%s", destinationRepositoryUrl, tag))
	if err != nil {
		log.Fatalf("ERROR: unable to execute Repository() for destination repository '%s'\n", dest)
	}
	destRepo.Client = &auth.Client{
		Credential: auth.StaticCredential(helmChartMirrorConfig.DestinationRegistry, auth.Credential{
			Username: destRegistryCreds.Username,
			Password: destRegistryCreds.Password,
		}),
	}

	srcRepo, err := remote.NewRepository(fmt.Sprintf("%s/%s:%s", sourceRegistry, sourceRepository, tag))
	if err != nil {
		log.Fatalf("ERROR: unable to initialize source repository for '%s/%s:%s': %s\n", sourceRegistry, sourceRepository, tag, err)
	}

	rootDesc, err := srcRepo.Resolve(context.Background(), tag)
	if err != nil {
		log.Fatalf("ERROR: unable to resolve source reference for '%s/%s:%s': %s\n", sourceRegistry, sourceRepository, tag, err)
	}

	indexBytes, err := content.FetchAll(context.Background(), srcRepo, rootDesc)
	if err != nil {
		log.Fatalf("ERROR:unable to get index for '%s/%s:%s': %s\n", sourceRegistry, sourceRepository, tag, err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		log.Fatalf("ERROR: unable to parse index for '%s/%s:%s': %s\n", sourceRegistry, sourceRepository, tag, err)
	}

	copyOptions := oras.DefaultCopyOptions
	if helmChartMirrorConfig.AllPlatforms {
		_, err = oras.Copy(context.Background(), source, tagDigest, destRepo, tag, copyOptions)
		return
	}

	var desiredPlatforms []ocispec.Platform
	if index.MediaType == ocispec.MediaTypeImageIndex || index.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		// image is multi-arch
		if len(chart.Platforms) > 0 && helmChartMirrorConfig.OverridePlatform == "" {
			// Define platforms to keep
			for _, p := range chart.Platforms {
				platform := strings.Split(p, "/")
				desiredPlatforms = append(desiredPlatforms, ocispec.Platform{
					Architecture: platform[1],
					OS:           platform[0],
				})
			}

			// ctx := context.Background()
			// _, err := oras.Copy(ctx, srcRepo, tagDigest, destRepo, tag, copyOptions)
			err = copyWithPlatformFilter(context.Background(), srcRepo, destRepo, tag, tag, desiredPlatforms, true)
			if err != nil {
				log.Fatalf("ERROR: unable to copy filtered multi-arch image '%s/%s:%s' to '%s' (%s)", sourceRegistry, sourceRepository, tagDigest, reference, err)
			}
		}
		_, err = repo.Resolve(context.Background(), reference)
		if err != nil {
			fmt.Println(err)
			// Use current platform unless overridden
			// copyOptions := oras.DefaultCopyOptions
			if !helmChartMirrorConfig.AllPlatforms {
				if helmChartMirrorConfig.OverridePlatform != "" {
					platform := strings.Split(helmChartMirrorConfig.OverridePlatform, "/")
					copyOptions.WithTargetPlatform(&ocispec.Platform{
						OS:           platform[0],
						Architecture: platform[1],
					})
				} else {
					copyOptions.WithTargetPlatform(&ocispec.Platform{
						OS:           runtime.GOOS,
						Architecture: runtime.GOARCH,
					})
				}
			}

			_, err = oras.Copy(context.Background(), source, tagDigest, destRepo, tag, copyOptions)
			// err = copyWithPlatformFilter(context.Background(), srcRepo, destRepo, tag, desiredPlatforms)
			if err != nil {
				log.Printf("ERROR: unable to copy image from '%s/%s:%s' to '%s' (%s)", sourceRegistry, sourceRepository, tagDigest, reference, err)
			} else {
				log.Printf("INFO: succesfully copied image from '%s/%s:%s' to '%s'", sourceRegistry, sourceRepository, tagDigest, reference)
			}
		} else {
			log.Printf("INFO: skipping, image '%s' already exists", reference)
		}
	}
}

// copyWithPlatformFilter copies an image from source to destination, keeping only specified platforms
// If cleanupUnwanted is true, also removes unwanted platforms that may already exist in destination
func copyWithPlatformFilter(ctx context.Context, src, dst *remote.Repository, srcTag, dstTag string, platforms []ocispec.Platform, cleanupUnwanted bool) error {
	fmt.Println("=== Phase 1: Copy filtered platforms from source ===")
	fmt.Printf("Resolving source tag: %s\n", srcTag)

	// Step 1: Resolve the source tag to get the root descriptor
	rootDesc, err := src.Resolve(ctx, srcTag)
	if err != nil {
		return fmt.Errorf("failed to resolve source tag: %w", err)
	}

	fmt.Printf("Root descriptor: %s (type: %s)\n", rootDesc.Digest, rootDesc.MediaType)

	// Step 2: Determine what needs to be copied based on the root type
	switch rootDesc.MediaType {
	case ocispec.MediaTypeImageManifest, "application/vnd.docker.distribution.manifest.v2+json":
		// Single-platform image
		if err := copySinglePlatformImage(ctx, src, dst, rootDesc, dstTag, platforms); err != nil {
			return err
		}

	case ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.list.v2+json":
		// Multi-platform image
		if err := copyMultiPlatformImage(ctx, src, dst, rootDesc, dstTag, platforms); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported media type: %s", rootDesc.MediaType)
	}

	// Step 3: Optionally cleanup unwanted platforms that might already exist in destination
	if cleanupUnwanted {
		fmt.Println("\n=== Phase 2: Clean up unwanted platforms in destination ===")
		if err := cleanupUnwantedPlatforms(ctx, dst, dstTag, platforms); err != nil {
			// Log the error but don't fail - cleanup is optional
			fmt.Printf("Warning: Cleanup failed (non-fatal): %v\n", err)
		}
	}

	return nil
}

// copySinglePlatformImage handles copying a single-platform manifest
func copySinglePlatformImage(ctx context.Context, src, dst *remote.Repository, manifestDesc ocispec.Descriptor, dstTag string, platforms []ocispec.Platform) error {
	// Check if the platform matches
	if manifestDesc.Platform != nil && !isPlatformDesired(manifestDesc.Platform, platforms) {
		return fmt.Errorf("manifest platform %s/%s not in desired platforms",
			manifestDesc.Platform.OS, manifestDesc.Platform.Architecture)
	}

	fmt.Printf("Copying single-platform manifest: %s\n", manifestDesc.Digest)

	// Copy the entire graph (manifest + config + layers)
	if err := oras.CopyGraph(ctx, src, dst, manifestDesc, oras.CopyGraphOptions{}); err != nil {
		return fmt.Errorf("failed to copy graph: %w", err)
	}

	// Tag the manifest at destination
	if err := dst.Tag(ctx, manifestDesc, dstTag); err != nil {
		return fmt.Errorf("failed to tag manifest: %w", err)
	}

	return nil
}

// copyMultiPlatformImage handles copying and filtering a multi-platform index
func copyMultiPlatformImage(ctx context.Context, src, dst *remote.Repository, indexDesc ocispec.Descriptor, dstTag string, platforms []ocispec.Platform) error {
	fmt.Printf("Processing multi-platform index: %s\n", indexDesc.Digest)

	// Step 1: Fetch the index
	indexBytes, err := content.FetchAll(ctx, src, indexDesc)
	if err != nil {
		return fmt.Errorf("failed to fetch index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	fmt.Printf("Found %d manifests in index\n", len(index.Manifests))

	// Debug: Print all manifests in source index
	fmt.Println("\nDEBUG: All manifests in source index:")
	for i, m := range index.Manifests {
		platformStr := formatPlatform(m.Platform)
		fmt.Printf("  [%d] %s - %s\n", i, m.Digest, platformStr)
	}
	fmt.Println()

	// Step 2: Filter manifests to only desired platforms
	var filteredManifests []ocispec.Descriptor
	seenDigests := make(map[string]bool) // Track digests to prevent true duplicates

	for _, manifest := range index.Manifests {
		if manifest.Platform == nil {
			fmt.Printf("  Including manifest without platform: %s\n", manifest.Digest)
			filteredManifests = append(filteredManifests, manifest)
			seenDigests[manifest.Digest.String()] = true
			continue
		}

		if !isPlatformDesired(manifest.Platform, platforms) {
			fmt.Printf("  Skipping platform %s/%s", manifest.Platform.OS, manifest.Platform.Architecture)
			if manifest.Platform.OSVersion != "" {
				fmt.Printf(" (OSVersion: %s)", manifest.Platform.OSVersion)
			}
			if manifest.Platform.Variant != "" {
				fmt.Printf(" (Variant: %s)", manifest.Platform.Variant)
			}
			fmt.Println()
			continue
		}

		// Check if we've already seen this exact digest
		digestStr := manifest.Digest.String()
		if seenDigests[digestStr] {
			fmt.Printf("  ! WARNING: Duplicate manifest digest %s - skipping to avoid duplicates in index\n", digestStr)
			continue
		}
		seenDigests[digestStr] = true

		// Build platform key for logging
		platformKey := formatPlatform(manifest.Platform)
		fmt.Printf("  Including platform %s: %s\n", platformKey, manifest.Digest)
		filteredManifests = append(filteredManifests, manifest)
	}

	if len(filteredManifests) == 0 {
		return fmt.Errorf("no manifests match desired platforms")
	}

	fmt.Printf("Filtered to %d unique manifests\n", len(filteredManifests))

	// Debug: Print what we're including in filtered index
	fmt.Println("\nDEBUG: Filtered manifests to be included:")
	for i, m := range filteredManifests {
		platformStr := formatPlatform(m.Platform)
		fmt.Printf("  [%d] %s - %s\n", i, m.Digest, platformStr)
	}
	fmt.Println()

	// Step 3: CRITICAL - Copy each selected manifest and its layers FIRST
	// This ensures all referenced content exists at the destination before we create the index
	for i, manifest := range filteredManifests {
		fmt.Printf("Copying manifest %d/%d: %s\n", i+1, len(filteredManifests), manifest.Digest)

		if err := oras.CopyGraph(ctx, src, dst, manifest, oras.CopyGraphOptions{}); err != nil {
			return fmt.Errorf("failed to copy manifest %s: %w", manifest.Digest, err)
		}
	}

	fmt.Println("All manifests and layers copied successfully")

	// Step 4: NOW create a new filtered index
	// At this point, all manifests referenced by this index exist at the destination
	filteredIndex := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:   ocispec.MediaTypeImageIndex,
		Manifests:   filteredManifests,
		Annotations: index.Annotations,
	}

	// Step 5: Marshal and push the filtered index to destination
	filteredIndexBytes, err := json.Marshal(filteredIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal filtered index: %w", err)
	}

	filteredIndexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, filteredIndexBytes)
	filteredIndexDesc.Annotations = indexDesc.Annotations

	fmt.Printf("Pushing filtered index: %s\n", filteredIndexDesc.Digest)

	if err := dst.Push(ctx, filteredIndexDesc, bytes.NewReader(filteredIndexBytes)); err != nil {
		return fmt.Errorf("failed to push filtered index: %w", err)
	}

	// Step 6: Tag the filtered index at destination
	if err := dst.Tag(ctx, filteredIndexDesc, dstTag); err != nil {
		return fmt.Errorf("failed to tag filtered index: %w", err)
	}

	fmt.Printf("Tagged as %s\n", dstTag)

	return nil
}

// cleanupUnwantedPlatforms removes platforms from destination that are not in the desired list
func cleanupUnwantedPlatforms(ctx context.Context, dst *remote.Repository, tag string, desiredPlatforms []ocispec.Platform) error {
	fmt.Printf("Checking for unwanted platforms in destination: %s:%s\n", dst.Reference.Repository, tag)

	// Resolve the tag
	rootDesc, err := dst.Resolve(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to resolve tag: %w", err)
	}

	// Only process multi-platform indexes
	switch rootDesc.MediaType {
	case ocispec.MediaTypeImageManifest, "application/vnd.docker.distribution.manifest.v2+json":
		fmt.Println("Destination is single-platform, no cleanup needed")
		return nil

	case ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.list.v2+json":
		// Continue to cleanup
	default:
		return fmt.Errorf("unsupported media type: %s", rootDesc.MediaType)
	}

	// Fetch current index
	indexBytes, err := content.FetchAll(ctx, dst, rootDesc)
	if err != nil {
		return fmt.Errorf("failed to fetch index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	fmt.Printf("Current destination has %d manifests\n", len(index.Manifests))

	// Filter manifests
	var keptManifests []ocispec.Descriptor
	var removedCount int
	seenDigests := make(map[string]bool)

	for _, manifest := range index.Manifests {
		digestStr := manifest.Digest.String()

		// Skip duplicates
		if seenDigests[digestStr] {
			fmt.Printf("  Skipping duplicate digest: %s\n", digestStr)
			continue
		}
		seenDigests[digestStr] = true

		// Check if platform is desired
		if manifest.Platform == nil || isPlatformDesired(manifest.Platform, desiredPlatforms) {
			keptManifests = append(keptManifests, manifest)
		} else {
			removedCount++
			fmt.Printf("  Removing unwanted: %s (%s)\n", formatPlatform(manifest.Platform), manifest.Digest)
		}
	}

	if removedCount == 0 {
		fmt.Println("No unwanted platforms found - destination is clean")
		return nil
	}

	if len(keptManifests) == 0 {
		return fmt.Errorf("cannot remove all platforms - at least one must remain")
	}

	fmt.Printf("Removing %d unwanted platform(s), keeping %d\n", removedCount, len(keptManifests))

	// Create cleaned index
	cleanedIndex := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:   ocispec.MediaTypeImageIndex,
		Manifests:   keptManifests,
		Annotations: index.Annotations,
	}

	cleanedIndexBytes, err := json.Marshal(cleanedIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned index: %w", err)
	}

	cleanedIndexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, cleanedIndexBytes)
	cleanedIndexDesc.Annotations = rootDesc.Annotations

	if err := dst.Push(ctx, cleanedIndexDesc, bytes.NewReader(cleanedIndexBytes)); err != nil {
		return fmt.Errorf("failed to push cleaned index: %w", err)
	}

	if err := dst.Tag(ctx, cleanedIndexDesc, tag); err != nil {
		return fmt.Errorf("failed to tag cleaned index: %w", err)
	}

	fmt.Printf("Updated tag to cleaned index: %s\n", cleanedIndexDesc.Digest)
	fmt.Println("Note: Old manifest blobs remain in registry until garbage collection runs")

	return nil
}

// formatPlatform creates a readable string from a platform descriptor
func formatPlatform(platform *ocispec.Platform) string {
	if platform == nil {
		return "no platform"
	}

	result := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	if platform.Variant != "" {
		result += "/" + platform.Variant
	}
	if platform.OSVersion != "" {
		result += "@" + platform.OSVersion
	}
	return result
}

// isPlatformDesired checks if a platform matches any of the desired platforms
//
// Wildcard behavior:
//   - If OSVersion is NOT specified in desired platform: matches ALL OSVersions
//     (useful for bundling all Windows versions together)
//   - If OSVersion IS specified: must match exactly (for selecting specific Windows versions)
//   - If Variant is NOT specified: matches any variant
//   - If Variant IS specified: must match exactly
func isPlatformDesired(platform *ocispec.Platform, desired []ocispec.Platform) bool {
	if platform == nil {
		return true
	}

	for _, p := range desired {
		// Match OS and Architecture
		if platform.OS != p.OS || platform.Architecture != p.Architecture {
			continue
		}

		// If desired platform specifies a variant, it must match exactly
		if p.Variant != "" && platform.Variant != p.Variant {
			continue
		}

		// If desired platform specifies OS version, it must match exactly
		if p.OSVersion != "" && platform.OSVersion != p.OSVersion {
			continue
		}

		// Match found
		return true
	}

	return false
}
