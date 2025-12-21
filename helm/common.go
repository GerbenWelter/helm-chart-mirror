package helm

import (
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
