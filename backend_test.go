package datastax_astra

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

// Fill in the config details to test.
const (
	envVarAstraToken       = ""
	envVarAstraOrgId       = ""
	envVarAstraLogicalName = ""
	envVarAstraURL         = ""
	envVarRoleName         = ""
	envVarLeaseTime        = ""
	envVarRenewalTime      = ""
)

// getTestBackend will help you construct a test backend object.
func getTestBackend(tb testing.TB) (*datastaxAstraBackend, logical.Storage) {
	tb.Helper()

	config := logical.TestBackendConfig()
	config.StorageView = new(logical.InmemStorage)
	config.Logger = hclog.NewNullLogger()
	config.System = logical.TestSystemView()

	b, err := Factory(context.Background(), config)
	if err != nil {
		tb.Fatal(err)
	}

	return b.(*datastaxAstraBackend), config.StorageView
}

// testEnv creates an object to store and track testing environment
// resources
type testEnv struct {
	AstraToken  string
	URL         string
	OrgId       string
	LogicalName string
	RoleName    string
	LeaseTime   string
	RenewalTime string
	response    *logical.Response

	Backend logical.Backend
	Context context.Context
	Storage logical.Storage

	// SecretToken tracks the API token, for checking rotations
	SecretToken string

	// Tokens tracks the generated tokens, to make sure we clean up
	Tokens []string
}

func (e *testEnv) AddConfig(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"astra_token":  e.AstraToken,
			"url":          e.URL,
			"org_id":       e.OrgId,
			"logical_name": e.LogicalName,
			"renewal_time": e.RenewalTime,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

func (e *testEnv) AddUserTokenRole(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "role",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"role":   e.RoleName,
			"org_id": e.OrgId,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

func (e *testEnv) WriteUserToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"org_id":       e.OrgId,
			"logical_name": e.LogicalName,
			"role_name":    e.RoleName,
			"lease_time":   e.LeaseTime,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Data["clientId"])
	require.NotEmpty(t, resp.Data["token"])
	require.NotNil(t, resp.Data["orgId"])
	require.NotNil(t, resp.Data["logicalName"])
	require.Nil(t, err)

	if t, ok := resp.Data["token"]; ok {
		e.Tokens = append(e.Tokens, t.(string))
	}

	e.response = resp

}

// Read user token using role_name, org_id and logical_name
func (e *testEnv) ReadUserToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"org_id":       e.OrgId,
			"logical_name": e.LogicalName,
			"role_name":    e.RoleName,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp.Data["clientId"])
	require.NotNil(t, resp.Data["token"])
	require.NotNil(t, resp.Data["orgId"])
	require.NotNil(t, resp.Data["logicalName"])
	expectedResp := map[string]interface{}{"clientId": e.response.Data["clientId"], "generatedOn": e.response.Data["generatedOn"], "logicalName": "org1", "metadata": map[string]string(nil), "orgId": "03acd0a6-1451-4827-b206-81ad1099f1a1", "roleName": "r_w_user", "token": e.response.Data["token"]}
	require.NotNil(t, resp)
	require.Equal(t, expectedResp, resp.Data)
	require.Nil(t, err)
}

func (e *testEnv) ReadUserTokenUsingClientId(t *testing.T) {
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"client_id": e.response.Data["clientId"],
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp.Data["clientId"])
	require.NotNil(t, resp.Data["token"])
	require.NotNil(t, resp.Data["orgId"])
	require.NotNil(t, resp.Data["logicalName"])
	expectedResp := map[string]interface{}{"clientId": e.response.Data["clientId"], "generatedOn": e.response.Data["generatedOn"], "logicalName": "org1", "metadata": map[string]string(nil), "orgId": "03acd0a6-1451-4827-b206-81ad1099f1a1", "roleName": "r_w_user", "token": e.response.Data["token"]}
	require.NotNil(t, resp)
	require.Equal(t, expectedResp, resp.Data)
	require.Nil(t, err)
}

func (e *testEnv) RenewToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.RenewOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Secret:    e.response.Secret,
		Data: map[string]interface{}{
			"orgId": e.OrgId,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp)
	parsedRenewalTime, _ := time.ParseDuration(e.RenewalTime)
	require.Equal(t, resp.Secret.TTL, parsedRenewalTime)
	require.Equal(t, e.response.Secret.LeaseID, resp.Secret.LeaseID)
	require.Nil(t, err)
}

func (e *testEnv) RevokeToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.RevokeOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Secret:    e.response.Secret,
		Data: map[string]interface{}{
			"orgId": e.OrgId,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

// CleanupUserTokens removes the tokens
// when the test completes.
func (e *testEnv) CleanupUserTokens(t *testing.T) {
	if len(e.Tokens) == 0 {
		t.Fatalf("expected 2 tokens, got: %d", len(e.Tokens))
	}

	req := &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"role_name":    e.RoleName,
			"org_id":       e.OrgId,
			"logical_name": e.LogicalName,
		},
	}

	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)

}
