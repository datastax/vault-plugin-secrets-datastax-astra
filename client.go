package datastax_astra

import (
	"errors"

	dsAstraClient "github.com/datastax/astra-client-go/v2/astra"
)

// astraClient creates an object storing
// the client.
type astraClient struct {
	*dsAstraClient.Client
}

// newClient creates a new client to access Astra
// and exposes it for any secrets or roles to use.
func newClient(config *astraConfig) (*astraClient, error) {
	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	if config.AstraToken == "" {
		return nil, errors.New("astra token was not defined")
	}

	if config.URL == "" {
		return nil, errors.New("client URL was not defined")
	}


	return nil, nil
}

