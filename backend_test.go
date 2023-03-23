package datastax_astra

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Fill in the config details to test.
const (
	envVarAstraToken       = "AstraCS:Th!s1safAk3T0K3n"
	envVarAstraOrgId       = "TestOrgId"
	envVarAstraLogicalName = "TestLogicalName"
	envVarAstraURL         = "http://localhost:" + mockLocalServerPort
	envVarRoleName         = "TestRoleName"
	envVarTTL              = 3600
	envVarMaxTTL           = 36000
	envVarRoleId           = "TestRoleId"
	envVarCallerMode       = "standard"
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
	TTL         time.Duration
	MaxTTL      time.Duration
	RoleId      string
	CallerMode  string
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
			"caller_mode":  e.CallerMode,
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
			"role_id":   e.RoleId,
			"role_name": e.RoleName,
			"org_id":    e.OrgId,
			"ttl":       e.TTL * time.Second,
			"max_ttl":   e.MaxTTL * time.Second,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}

func (e *testEnv) WriteUserToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"org_id":       e.OrgId,
			"logical_name": e.LogicalName,
			"role_name":    e.RoleName,
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
	if envVarCallerMode == "standard" {
		expectedResp := map[string]interface{}{
			"clientId":    e.response.Data["clientId"],
			"generatedOn": e.response.Data["generatedOn"],
			"logicalName": "testlogicalname",
			"metadata":    map[string]string{},
			"orgId":       e.OrgId,
			"roleName":    "testrolename",
			"token":       e.response.Data["token"],
		}
		require.Equal(t, expectedResp, resp.Data)
	}
	require.NotNil(t, resp)
	require.Nil(t, err)
}

func (e *testEnv) ReadUserTokenUsingClientId(t *testing.T) {
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"client_id": e.response.Data["clientId"],
			"org_id":    e.OrgId,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp.Data["clientId"])
	require.NotNil(t, resp.Data["token"])
	require.NotNil(t, resp.Data["orgId"])
	require.NotNil(t, resp.Data["logicalName"])
	if envVarCallerMode == "standard" {
		expectedResp := map[string]interface{}{
			"clientId":    e.response.Data["clientId"],
			"generatedOn": e.response.Data["generatedOn"],
			"logicalName": "testlogicalname",
			"metadata":    map[string]string{},
			"orgId":       e.OrgId,
			"roleName":    "testrolename",
			"token":       e.response.Data["token"],
		}
		require.Equal(t, expectedResp, resp.Data)
	}
	require.NotNil(t, resp)
	require.Nil(t, err)
}

func (e *testEnv) RenewToken(t *testing.T) {
	req := &logical.Request{
		Operation: logical.RenewOperation,
		Path:      "org/token",
		Storage:   e.Storage,
		Secret:    e.response.Secret,
		Data: map[string]interface{}{
			"orgId":    e.OrgId,
			"roleName": e.RoleName,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.NotNil(t, resp)
	require.Equal(t, resp.Secret.TTL, e.TTL*time.Second)
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
			"orgId":    e.OrgId,
			"roleName": e.RoleName,
		},
	}
	resp, err := e.Backend.HandleRequest(e.Context, req)
	require.Nil(t, resp)
	require.Nil(t, err)
}
