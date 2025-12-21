package config

import (
	"bytes"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

type Chart struct {
	Name                   string           `yaml:"name"`
	Version                string           `yaml:"version"`
	TemplateConfigurations []map[string]any `yaml:"templateConfigurations"`
	DestinationRegistry    string           `yaml:"destinationRegistry"`
	DestinationRepository  string           `yaml:"destinationRepository"`
}

type Repository struct {
	Name   string  `yaml:"name"`
	Source string  `yaml:"source"`
	Charts []Chart `yaml:"charts"`
}

type Config struct {
	KubernetesVersion     string       `yaml:"kubernetesVersion"`
	Repositories          []Repository `yaml:"repositories"`
	OverridePlatform      string       `yaml:"overridePlatform"`
	DestinationRegistry   string
	DestinationRepository string
	TmpDir                string
}

var OCICredentials *credentials.DynamicStore

func LoadConfig() Config {
	mirrorRegistry, exists := os.LookupEnv("HELM_CHART_MIRROR_REGISTRY")
	if !exists {
		log.Fatal("ERROR: helm-chart-mirror password is not specified!")
	}

	mirrorRepository, exists := os.LookupEnv("HELM_CHART_MIRROR_BASE_REPO")
	if !exists {
		log.Fatal("ERROR: helm-chart-mirror base repository is not specified!")
	}

	tmpDir := "/tmp"
	envValue, exists := os.LookupEnv("HELM_CHART_MIRROR_TMPDIR")
	if exists {
		tmpDir = envValue
	}

	config := Config{
		DestinationRegistry:   mirrorRegistry,
		DestinationRepository: mirrorRepository,
		TmpDir:                tmpDir,
	}

	configFilePath := "/etc/helm-chart-mirror/config.yaml"
	envValue, exists = os.LookupEnv("HELM_CHART_MIRROR_CONFIG")
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
	return config
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
