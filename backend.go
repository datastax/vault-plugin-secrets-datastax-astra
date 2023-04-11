package datastax_astra

import (
	"context"
	"strings"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

// Factory returns a new backend as logical.Backend
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := backend()
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}
	return b, nil
}

// datastaxAstraBackend defines an object that
// extends the Vault backend and stores the
// target API's client.
type datastaxAstraBackend struct {
	*framework.Backend
	lock   sync.RWMutex
	client *astraClient
	logger log.Logger
}

// backend defines the target API backend
// for Vault. It must include each path
// and the secrets it will store.
func backend() *datastaxAstraBackend {
	var b = datastaxAstraBackend{}
	b.logger = logging.NewVaultLogger(log.Debug)
	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(backendHelp),
		PathsSpecial: &logical.Paths{
			LocalStorage: []string{
				framework.WALPrefix,
			},
			SealWrapStorage: []string{
				"config",
				"org/token",
				"role",
			},
		},
		Paths: framework.PathAppend(
			[]*framework.Path{
				pathConfig(&b),
				pathCredentials(&b),
				pathRole(&b),
				pathRoleList(&b),
				pathConfigList(&b),
			},
		),
		Secrets: []*framework.Secret{
			b.astraToken(),
		},
		BackendType: logical.TypeLogical,
		Invalidate:  b.invalidate,
	}
	return &b
}

// reset clears any client configuration for a new
// backend to be configured
func (b *datastaxAstraBackend) reset() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.client = nil
}

// invalidate clears an existing client configuration in
// the backend
func (b *datastaxAstraBackend) invalidate(ctx context.Context, key string) {
	if key == "config" {
		b.reset()
	}
}

func (b *datastaxAstraBackend) getClient(ctx context.Context, s logical.Storage, orgId string) (*astraClient, error) {
	b.lock.RLock()
	unlockFunc := b.lock.RUnlock
	defer func() { unlockFunc() }()

	if b.client != nil {
		return b.client, nil
	}

	b.lock.RUnlock()
	b.lock.Lock()
	unlockFunc = b.lock.Unlock

	config, err := getConfig(ctx, s, orgId)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = new(astraConfig)
	}

	b.client, err = newClient(config)
	if err != nil {
		return nil, err
	}

	return b.client, nil
}

// backendHelp should contain help information for the backend
const backendHelp = `
The Astra secrets backend dynamically generates user tokens.
After mounting this backend, credentials to manage Astra tokens
must be configured with the "config/" endpoints.`
