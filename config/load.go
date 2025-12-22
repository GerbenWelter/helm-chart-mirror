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
	DestinationRegistry   string       `yaml:"destinationRegistry"`
	DestinationRepository string       `yaml:"destinationRepository"`
	TmpDir                string       `yaml:"tmpDir"`
}

var OCICredentials *credentials.DynamicStore

func LoadConfig() Config {
	config := Config{
		DestinationRegistry:   "",
		DestinationRepository: "",
		TmpDir:                "/tmp",
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

	if config.DestinationRegistry == "" || config.DestinationRepository == "" {
		log.Fatalln("ERROR: mirror registry and mirror repository need to be configured!")
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
