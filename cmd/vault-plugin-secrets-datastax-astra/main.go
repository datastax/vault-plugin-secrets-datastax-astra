package main

import (
	datastax_astra "github.com/datastax/vault-plugin-secrets-datastax-astra"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"
	"os"
)

func main() {
	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	tlsConfig := apiClientMeta.GetTLSConfig()
	tlsProviderFunc := api.VaultPluginTLSProvider(tlsConfig)

	err := plugin.Serve(&plugin.ServeOpts{
		BackendFactoryFunc: datastax_astra.Factory,
		TLSProviderFunc:    tlsProviderFunc,
	})
	if err != nil {
		logger := datastax_astra.NewLogger()
		logger.Error("plugin shutting down", "error", err)
		os.Exit(1)
	}
}
