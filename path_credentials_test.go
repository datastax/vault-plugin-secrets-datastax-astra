package datastax_astra

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	mockLocalServerPort = "8080"
)

type HttpResponse struct {
	ClientID    string   `json:"clientId"`
	Secret      string   `json:"secret"`
	Token       string   `json:"token"`
	OrgID       string   `json:"orgId"`
	Roles       []string `json:"roles"`
	GeneratedOn string   `json:"generatedOn"`
}

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
		Logger: logging.NewVaultLogger(hclog.Debug),
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
		TTL:         envVarTTL,
		MaxTTL:      envVarMaxTTL,
		RoleId:      envVarRoleId,
		CallerMode:  envVarCallerMode,
		Backend:     b,
		Context:     ctx,
		Storage:     &logical.InmemStorage{},
	}, nil
}

func clientIdSecretsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		tokenResponse := HttpResponse{
			ClientID:    "test_client_id",
			Secret:      "test_secret",
			Token:       "test_token",
			OrgID:       envVarAstraOrgId,
			Roles:       []string{envVarRoleId},
			GeneratedOn: time.Now().Format(time.RFC3339),
		}

		tokenResponseJson, err := json.Marshal(tokenResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(tokenResponseJson)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case "DELETE":
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "not found"}`))
	}
}

// TestAcceptanceUserToken tests a series of steps to make
// sure the token creation work correctly.
func TestAcceptanceUserToken(t *testing.T) {
	http.HandleFunc("/v2/clientIdSecrets", clientIdSecretsHandler)
	http.HandleFunc("/v2/clientIdSecrets/", clientIdSecretsHandler)

	mockServer := &http.Server{
		Addr: ":" + mockLocalServerPort,
	}

	// Start a local server to mock the Astra API
	go func() {
		log.Println("Listening on localhost:" + mockLocalServerPort)
		err := mockServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()

	acceptanceTestEnv, err := newAcceptanceTestEnv()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("add config", acceptanceTestEnv.AddConfig)
	t.Run("add role", acceptanceTestEnv.AddUserTokenRole)
	t.Run("write user token cred", acceptanceTestEnv.WriteUserToken)
	t.Run("read user token cred", acceptanceTestEnv.ReadUserToken)
	if acceptanceTestEnv.CallerMode == "standard" {
		t.Run("read user token cred", acceptanceTestEnv.ReadUserTokenUsingClientId)
	}
	t.Run("renew user token cred", acceptanceTestEnv.RenewToken)
	t.Run("revoke user token cred", acceptanceTestEnv.RevokeToken)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server after testing is done
	err = mockServer.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Server shutdown failed: %s", err)
	}
}
