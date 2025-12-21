package config

import (
	"bytes"
	"log"
	"os"

	"gopkg.in/yaml.v3"
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
	DestinationRegistry   string
	DestinationRepository string
	DockerUsername        string
	DockerPassword        string
	TmpDir                string
}

func LoadConfig() Config {
	dockerUsername, exists := os.LookupEnv("DOCKER_IO_USERNAME")
	if !exists {
		log.Println("WARNING: docker.io username is not configured. This may result in HTTP 429 (rate limiting) errors.")
	}

	dockerPassword, exists := os.LookupEnv("DOCKER_IO_PASSWORD")
	if !exists {
		log.Println("WARNING: docker.io password is not configured. This may result in HTTP 429 (rate limiting) errors.")
	}

	registry, exists := os.LookupEnv("HELM_CHART_MIRROR_REGISTRY")
	if !exists {
		log.Fatal("ERROR: helm-chart-mirror password is not specified!")
	}

	registryBaseRepository, exists := os.LookupEnv("HELM_CHART_MIRROR_BASE_REPO")
	if !exists {
		log.Fatal("ERROR: helm-chart-mirror base repository is not specified!")
	}

	tmpDir := "/tmp"
	envValue, exists := os.LookupEnv("HELM_CHART_MIRROR_TMPDIR")
	if exists {
		tmpDir = envValue
	}

	config := Config{
		DestinationRegistry:   registry,
		DestinationRepository: registryBaseRepository,
		DockerUsername:        dockerUsername,
		DockerPassword:        dockerPassword,
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

	// repositoryConfig := &RepositoryConfig{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err = decoder.Decode(&config); err != nil {
		log.Fatalf("ERROR: unable to parse config (%s)", err)
	}
	return config
}
