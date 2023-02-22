package datastax_astra

import (
	"context"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
)

// newAcceptanceTestEnv creates a test environment for credentials
func newAcceptanceTestEnv() (*testEnv, error) {
	ctx := context.Background()

	maxLease, _ := time.ParseDuration("60s")
	defaultLease, _ := time.ParseDuration("30s")
	conf := &logical.BackendConfig{
		System: &logical.StaticSystemView{
			DefaultLeaseTTLVal: defaultLease,
			MaxLeaseTTLVal:     maxLease,
		},
		Logger: logging.NewVaultLogger(log.Debug),
	}
	b, err := Factory(ctx, conf)
	if err != nil {
		return nil, err
	}
	return &testEnv{
		AstraToken:  envVarAstraToken,
		URL:         envVarAstraURL,
		OrgId:       envVarAstraOrgId,
		LogicalName: envVarAstraLogicalName,
		RoleName:    envVarRoleName,
		LeaseTime:   envVarLeaseTime,
		RenewalTime: envVarRenewalTime,
		RoleId:      envVarRoleId,
		Backend:     b,
		Context:     ctx,
		Storage:     &logical.InmemStorage{},
	}, nil
}

// TestAcceptanceUserToken tests a series of steps to make
// sure the token creation work correctly.
func TestAcceptanceUserToken(t *testing.T) {

	acceptanceTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("add config", acceptanceTestEnv.AddConfig)
	t.Run("add role", acceptanceTestEnv.AddUserTokenRole)
	t.Run("write user token cred", acceptanceTestEnv.WriteUserToken)
	t.Run("read user token cred", acceptanceTestEnv.ReadUserToken)
	t.Run("read user token cred", acceptanceTestEnv.ReadUserTokenUsingClientId)
	t.Run("renew user token cred", acceptanceTestEnv.RenewToken)
	t.Run("revoke user token cred", acceptanceTestEnv.RevokeToken)
	// t.Run("cleanup user tokens", acceptanceTestEnv.CleanupUserTokens)
}
