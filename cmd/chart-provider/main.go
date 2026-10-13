package main

import (
	chartprovider "example.com/helm-release-poc/internal/provider"
	"github.com/go-semantic-release/semantic-release/v2/pkg/plugin"
	"github.com/go-semantic-release/semantic-release/v2/pkg/provider"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		Provider: func() provider.Provider {
			return &chartprovider.GitHubProvider{}
		},
	})
}
