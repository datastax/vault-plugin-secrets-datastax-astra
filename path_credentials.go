package datastax_astra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	credsPath     = "org/token"
	credsListPath = "org/tokens/?"
	pluginversion = "Vault-Plugin v1.0.0"
	defaultMaxLeaseTime = "24h"
)

// pathCredentials extends the Vault API with a `/token` endpoint for a role.
func pathCredentials(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: credsPath,
		Fields: map[string]*framework.FieldSchema{
			"org_id": {
				Type:        framework.TypeString,
				Description: "name of the org for which token is being requested",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
			"role_name": {
				Type:        framework.TypeLowerCaseString,
				Description: "name of the role for which token is being requested",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
			"logical_name": {
				Type:        framework.TypeLowerCaseString,
				Description: "Logical name to reference this token by",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
			"metadata": {
				Type:         framework.TypeKVPairs,
				Description:  "Arbitrary key=value",
				Required:     false,
				DisplayAttrs: &framework.DisplayAttributes{Sensitive: false},
			},
			"client_id": {
				Type:        framework.TypeString,
				Description: "ClientId for the token",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
			"lease_time": {
				Type:        framework.TypeString,
				Description: "leaseTime in seconds, minutes or hours for the token. If this value is bigger than max_lease_time, it will be clamped to the max_lease_time value. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
			"max_lease_time": {
				Type:        framework.TypeString,
				Description: "Maximum leaseTime in seconds, minutes or hours for the token. Defaults to 24 hours. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredentialsRead,
			},
			logical.CreateOperation: &framework.PathOperation{Callback: b.pathCredentialsWrite},
			logical.UpdateOperation: &framework.PathOperation{Callback: b.pathCredentialsWrite},
			logical.DeleteOperation: &framework.PathOperation{Callback: b.pathTokenDelete},
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

// pathCredentialsRead reads a token from vault.
func (b *datastaxAstraBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	clientId, ok := d.GetOk("client_id")
	if !ok {
		roleName, ok := d.GetOk("role_name")
		if !ok {
			return nil, errors.New("role_name not provided")
		}
		orgId, ok := d.GetOk("org_id")
		if !ok {
			return nil, errors.New("org_id not provided")
		}
		logicalName, ok := d.GetOk("logical_name")
		if !ok {
			return nil, errors.New("logical_name not provided")
		}
		tokens, err := listCreds(ctx, req.Storage)
		if err != nil {
			return nil, errors.New("no tokens found")
		}
		if len(tokens) == 0 {
			return nil, errors.New("no token found in vault")
		}
		for i := 0; i < len(tokens); i++ {
			token, err := readToken(ctx, req.Storage, tokens[i])
			if err != nil {
				return nil, errors.New("no tokens found")
			}
			if doesTokenMatch(token, orgId.(string), roleName.(string), logicalName.(string)) {
				return &logical.Response{Data: token.toResponseData()}, nil
			}
		}
		return nil, errors.New("no token found that matches criteria")
	}
	tokens, err := listCreds(ctx, req.Storage)
	if err != nil {
		return nil, errors.New("no tokens found")
	}
	if len(tokens) == 0 {
		return nil, errors.New("no token found in vault")
	}
	for i := 0; i < len(tokens); i++ {
		token, err := readToken(ctx, req.Storage, tokens[i])
		if err != nil {
			return nil, errors.New("no tokens found")
		}
		if doesTokenMatchClientId(token, clientId.(string)) {
			return &logical.Response{Data: token.toResponseData()}, nil
		}
	}
	return nil, errors.New("no token found that matches criteria client")
}

func doesTokenMatchClientId(token *astraToken, clientId string) bool {
	return token.ClientID == clientId
}

func doesTokenMatch(token *astraToken, orgId, role, logicalName string) bool {
	return token.LogicalName == logicalName && token.OrgID == orgId && token.RoleNickname == role
}

func readToken(ctx context.Context, s logical.Storage, uuid string) (*astraToken, error) {
	token, err := s.Get(ctx, "token/"+uuid)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}
	result := &astraToken{}
	if err := token.DecodeJSON(result); err != nil {
		return nil, err
	}
	return result, nil
}
func (b *datastaxAstraBackend) pathCredentialsWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return nil, errors.New("role_name not provided")
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return nil, errors.New("org_id not provided")
	}
	logicalName, ok := d.GetOk("logical_name")
	if !ok {
		return nil, errors.New("logical_name not provided")
	}
	tok, err := readToken(ctx, req.Storage, logicalName.(string))
	if tok != nil {
		return nil, errors.New("token already exists for role")
	}
	entry, err := readRole(ctx, req.Storage, roleName.(string), orgId.(string))
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.New("role does not exist. add role first")
	}

	payload := strings.NewReader(`{
    "roles": [` + `"` + entry.RoleId + `"]}`)
	conf, err := getConfig(ctx, req.Storage, orgId.(string))
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	url := conf.URL + "/v2/clientIdSecrets"
	httpReq, err := http.NewRequest(http.MethodPost, url, payload)

	if err != nil {
		msg := "error creating httpReq " + err.Error()
		return nil, errors.New(msg)
	}
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Authorization", "Bearer "+conf.AstraToken)
	httpReq.Header.Add("User-Agent", pluginversion)
	res, err := client.Do(httpReq)
	if err != nil {
		msg := "error sending request " + err.Error()
		return nil, errors.New(msg)
	}
	defer res.Body.Close()
	var token *astraToken
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		msg := "error reading ioutil " + err.Error()
		return nil, errors.New(msg)
	}
	err = json.Unmarshal(body, &token)
	if err != nil {
		msg := " Unmarshal failed " + err.Error()
		return nil, errors.New(msg)
	}
	metadata, ok, err := d.GetOkErr("metadata")
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to parse metadata: %v", err)), nil
	}
	if ok {
		token.Metadata = metadata.(map[string]string)
	}
	token.LogicalName = logicalName.(string)
	token.RoleNickname = roleName.(string)
	internalData := map[string]interface{}{
		"token":    token.Token,
		"metadata": token.Metadata,
		"orgId":    token.OrgID,
	}

	err = saveToken(ctx, token, req.Storage)
	if err != nil {
		return nil, err
	}
	resp := b.Secret(astraTokenType).Response(token.toResponseData(), internalData)
	leaseTime, ok := d.GetOk("lease_time")
	if !ok {
		return resp, nil
	}
	parseLeaseTime, _ := time.ParseDuration(leaseTime.(string))
	maxLeaseTime, ok := d.GetOk("max_lease_time")
	var rtnErr error
	if !ok {
		msg := "error getting Max Lease Time. Setting value to default of " + defaultMaxLeaseTime
		rtnErr = errors.New(msg)
		maxLeaseTime = defaultMaxLeaseTime
	}
	if maxLeaseTime == "" {
		maxLeaseTime = defaultMaxLeaseTime
	}
	parseMaxLeaseTime, _ := time.ParseDuration(maxLeaseTime.(string))
	if parseLeaseTime > parseMaxLeaseTime {
		parseLeaseTime = parseMaxLeaseTime
	}
	resp.Secret.TTL = parseLeaseTime
	resp.Secret.MaxTTL = parseMaxLeaseTime
	resp.Secret.Renewable = true
	return resp, rtnErr
}

func (b *datastaxAstraBackend) pathTokenDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return nil, errors.New("role_name not provided")
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return nil, errors.New("org_id not provided")
	}
	logicalName, ok := d.GetOk("logical_name")
	if !ok {
		return nil, errors.New("logical_name not provided")
	}
	tokens, err := listCreds(ctx, req.Storage)
	if err != nil {
		return nil, errors.New("no tokens found")
	}
	if len(tokens) == 0 {
		return nil, errors.New("no token found in vault")
	}
	for i := 0; i < len(tokens); i++ {
		token, err := readToken(ctx, req.Storage, tokens[i])
		if err != nil {
			return nil, errors.New("no tokens found")
		}
		if doesTokenMatch(token, orgId.(string), roleName.(string), logicalName.(string)) {
			err = req.Storage.Delete(ctx, "token/"+tokens[i])
			if err != nil {
				return nil, err
			}
			conf, err := getConfig(ctx, req.Storage, orgId.(string))
			if err != nil {
				return nil, err
			}
			if conf.URL != "" {
				client := &http.Client{}
				url := conf.URL + "/v2/clientIdSecrets/" + token.ClientID
				httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
				if err != nil {
					msg := "error creating httpReq " + err.Error()
					return nil, errors.New(msg)
				}
				httpReq.Header.Add("Content-Type", "application/json")
				httpReq.Header.Add("Authorization", "Bearer "+conf.AstraToken)
				httpReq.Header.Add("User-Agent", pluginversion)
				res, err := client.Do(httpReq)
				if err != nil {
					msg := "error sending request " + err.Error()
					return nil, errors.New(msg)
				}
				defer res.Body.Close()
				if res.StatusCode != http.StatusNoContent {
					return nil, errors.New("could not delete token in astra")
				}
			} else {
				return nil, errors.New("config not found")
			}
		}
	}
	b.reset()
	return nil, nil
}

func saveToken(ctx context.Context, token *astraToken, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON("token/"+token.ClientID, token)
	if err != nil {
		return err
	}
	if err = s.Put(ctx, entry); err != nil {
		return err
	}
	return nil
}

func doTokensMatch(token *astraToken, tokentoRevoke string) bool {
	return token.Token == tokentoRevoke
}

func (b *datastaxAstraBackend) tokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	tokenRaw, ok := req.Secret.InternalData["token"]
	if !ok {
		return nil, errors.New("token not retrived")
	}

	tokens, err := listCreds(ctx, req.Storage)
	if err != nil {
		return nil, errors.New("no tokens found")
	}

	for i := 0; i < len(tokens); i++ {
		token, err := readToken(ctx, req.Storage, tokens[i])
		if err != nil {
			return nil, errors.New("no tokens found")
		}
		if doTokensMatch(token, tokenRaw.(string)) {
			err = req.Storage.Delete(ctx, "token/"+tokens[i])
			if err != nil {
				return nil, err
			}
			conf, err := getConfig(ctx, req.Storage, token.OrgID)
			if err != nil {
				return nil, err
			}
			if conf.URL != "" {
				client := &http.Client{}
				url := conf.URL + "/v2/clientIdSecrets/" + token.ClientID
				httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
				if err != nil {
					msg := "error creating httpReq " + err.Error()
					return nil, errors.New(msg)
				}
				httpReq.Header.Add("Content-Type", "application/json")
				httpReq.Header.Add("Authorization", "Bearer "+conf.AstraToken)
				httpReq.Header.Add("User-Agent", pluginversion)
				res, err := client.Do(httpReq)
				if err != nil {
					msg := "error sending request " + err.Error()
					return nil, errors.New(msg)
				}
				defer res.Body.Close()
				if res.StatusCode != http.StatusNoContent {
					return nil, errors.New("could not delete token in astra")
				}
			} else {
				return nil, errors.New("config not found")
			}
		}
	}
	b.reset()
	return nil, nil
}

func (b *datastaxAstraBackend) tokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	resp := &logical.Response{Secret: req.Secret}
	uuid := req.Secret.InternalData["orgId"]
	configData, err := getConfig(ctx, req.Storage, uuid.(string))
	if err != nil {
		resp.Secret.TTL = 24 * time.Hour
		return resp, errors.New("error getting config data. lease time set to 24h")
	}
	renewalTime := configData.DefaultLeaseRenewTime
	if renewalTime == "" {
		resp.Secret.TTL = 24 * time.Hour
		return resp, nil
	}
	parsedRenewalTime, err := time.ParseDuration(renewalTime)
	if err != nil {
		resp.Secret.TTL = 24 * time.Hour
		return resp, errors.New("error parsing default lease time. lease time set to 24h")
	}
	resp.Secret.TTL = parsedRenewalTime
	return resp, nil
}

const pathCredentialsHelpSyn = `
Generate a AstraCS token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Astra CS token based on a particular role. 
`
