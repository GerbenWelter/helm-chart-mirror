package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

var repoPathRegex = regexp.MustCompile(`^[^/].*[^/]$`)

type Chart struct {
	Name                     string           `yaml:"name"`
	Version                  string           `yaml:"version"`
	TemplateConfigurations   []map[string]any `yaml:"templateConfigurations"`
	AdditionalImageResources []string         `yaml:"additionalImageResources"`
	Platforms                []string         `yaml:"platforms"`
}

type Repository struct {
	Name                     string   `yaml:"name"`
	Source                   string   `yaml:"source"`
	Charts                   []Chart  `yaml:"charts"`
	AdditionalImageResources []string `yaml:"additionalImageResources"`
}

type Config struct {
	KubernetesVersion            string       `yaml:"kubernetesVersion"`
	Repositories                 []Repository `yaml:"repositories"`
	OverridePlatform             string       `yaml:"overridePlatform"`
	AllPlatforms                 bool         `yaml:"allPlatforms"`
	DestinationRegistry          string       `yaml:"destinationRegistry"`
	ChartDestinationRepository   string       `yaml:"chartDestinationRepository"`
	ImageDestinationRepository   string       `yaml:"imageDestinationRepository"`
	IncludeOriginalImageRegistry bool         `yaml:"includeOriginalImageRegistry"`
	TmpDir                       string       `yaml:"tmpDir"`
	AdditionalImageResources     []string     `yaml:"additionalImageResources"`
}

var OCICredentials *credentials.DynamicStore

func LoadConfig() Config {
	config := Config{
		DestinationRegistry:          "",
		ChartDestinationRepository:   "",
		ImageDestinationRepository:   "",
		IncludeOriginalImageRegistry: true,
		TmpDir:                       "/tmp",
	}

	configFilePath := "/etc/helm-chart-mirror/config.yaml"
	envValue, exists := os.LookupEnv("HELM_CHART_MIRROR_CONFIG")
	if exists {
		configFilePath = envValue
	}
	log.Printf("INFO: loading config from '%s'\n", configFilePath)

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Fatalf("ERROR: unable to read config file '%s'\n", configFilePath)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err = decoder.Decode(&config); err != nil {
		log.Fatalf("ERROR: unable to parse config (%s)", err)
	}

	err = ValidateAndDefaultConfig(&config)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}

	return config
}

func validateRepoPath(name, value string) error {
	value = strings.TrimSpace(value)

	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}

	if !repoPathRegex.MatchString(value) {
		return fmt.Errorf(
			"%s must not start or end with '/' (got %q)",
			name, value,
		)
	}

	return nil
}

func ValidateAndDefaultConfig(config *Config) error {
	if config.DestinationRegistry == "" {
		return fmt.Errorf("mirror registry needs to be configured!")
	}

	if config.AllPlatforms && config.OverridePlatform != "" {
		return fmt.Errorf("overridePlatform and allPlatforms are mutually exclusive!")
	}

	// Defaulting
	if config.ChartDestinationRepository == "" {
		config.ChartDestinationRepository = "charts"
	} else {
		if err := validateRepoPath(
			"chartDestinationRepository",
			config.ChartDestinationRepository,
		); err != nil {
			return err
		}
	}

	// Optional field
	if config.ImageDestinationRepository != "" {
		if err := validateRepoPath(
			"imageDestinationRepository",
			config.ImageDestinationRepository,
		); err != nil {
			return err
		}
	}

	return nil
}

func LoadOCICredentials() {
	credentialsFilePath := "/etc/helm-chart-mirror/auth.json"
	envValue, exists := os.LookupEnv("HELM_CHART_MIRROR_OCI_CREDENTIALS")
	if exists {
		credentialsFilePath = envValue
	}
	log.Printf("INFO: loading registry credentials from '%s'\n", credentialsFilePath)
	storeOptions := credentials.StoreOptions{}
	creds, err := credentials.NewStore(credentialsFilePath, storeOptions)
	if err != nil {
		log.Fatalf("ERROR: unable to load OCI credentials from file '%s': %v", credentialsFilePath, err)
	}

	OCICredentials = creds
}
