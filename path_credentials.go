package datastax_astra

import (
	"context"
	"crypto/sha256"
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
				Description: "leaseTime in seconds, minutes or hours for the token. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredentialsRead,
			},
			//logical.CreateOperation: &framework.PathOperation{Callback: b.pathCredentialsWrite},
			logical.UpdateOperation: &framework.PathOperation{Callback: b.pathCredentialsWrite},
			logical.DeleteOperation: &framework.PathOperation{Callback: b.pathTokenDelete},
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

// pathCredentialsRead reads a token from vault.
func (b *datastaxAstraBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("read called")
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
	fingerprint := calculateTokenFingerprintFromComponent(orgId.(string), roleName.(string), logicalName.(string))
	token, err := readToken(ctx, req.Storage, fingerprint)
	if err != nil {
		return nil, errors.New("no token found")
	}
	if doesTokenMatch(token, orgId.(string), roleName.(string), logicalName.(string)) {
		return &logical.Response{Data: token.toResponseData()}, nil
	}
	return nil, errors.New("no token found that matches criteria client")
}

func doesTokenMatchClientId(token *astraToken, clientId string) bool {
	return token.ClientID == clientId
}

func doesTokenMatch(token *astraToken, orgId, role, logicalName string) bool {
	return token.LogicalName == logicalName && token.OrgID == orgId && token.RoleNickname == role
}

func calculateTokenFingerprint(token *astraToken) string {
	return calculateTokenFingerprintFromComponent(token.OrgID, token.RoleNickname, token.LogicalName)
}
func calculateTokenFingerprintFromComponent(orgId, role, logicalName string) string {
	tokenRef := fmt.Sprintf("%s:%s:%s", orgId, role, logicalName)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(tokenRef)))
}

func readToken(ctx context.Context, s logical.Storage, fingerprint string) (*astraToken, error) {
	token, err := s.Get(ctx, "token/"+fingerprint)
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
	b.logger.Debug("write called")
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
	fingerprint := calculateTokenFingerprintFromComponent(orgId.(string), roleName.(string), logicalName.(string))
	tok, err := readToken(ctx, req.Storage, fingerprint)
	if tok != nil {
		b.logger.Debug("token exists")
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
		"token":       token.Token,
		"metadata":    token.Metadata,
		"orgId":       token.OrgID,
		"roleName":    token.RoleNickname,
		"logicalName": token.LogicalName,
	}

	err = saveToken(ctx, token, req.Storage)
	if err != nil {
		return nil, err
	}
	resp := b.Secret(astraTokenType).Response(token.toResponseData(), internalData)
	resp.Secret.TTL = entry.TTL
	resp.Secret.MaxTTL = entry.MaxTTL
	resp.Secret.Renewable = true
	b.logger.Debug("token created")
	return resp, nil
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

	fingerprint := calculateTokenFingerprintFromComponent(orgId.(string), roleName.(string), logicalName.(string))
	token, err := readToken(ctx, req.Storage, fingerprint)
	if err != nil {
		return nil, errors.New("no tokens found")
	}
	if doesTokenMatch(token, orgId.(string), roleName.(string), logicalName.(string)) {
		err = req.Storage.Delete(ctx, "token/"+fingerprint)
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
	b.reset()
	return nil, nil
}

func saveToken(ctx context.Context, token *astraToken, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON("token/"+calculateTokenFingerprint(token), token)
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
	b.logger.Debug("revoke called")
	tokenRaw, ok := req.Secret.InternalData["token"]
	if !ok {
		return nil, errors.New("token not retrived")
	}
	orgId, ok := req.Secret.InternalData["orgId"]
	if !ok {
		return nil, errors.New("orgId not retrived")
	}
	roleName, ok := req.Secret.InternalData["roleName"]
	if !ok {
		return nil, errors.New("roleName not retrived")
	}
	logicalName, ok := req.Secret.InternalData["logicalName"]
	if !ok {
		return nil, errors.New("logicalName not retrived")
	}

	fingerprint := calculateTokenFingerprintFromComponent(orgId.(string), roleName.(string), logicalName.(string))
	token, err := readToken(ctx, req.Storage, fingerprint)
	if err != nil {
		return nil, errors.New("no tokens found")
	}
	if doTokensMatch(token, tokenRaw.(string)) {
		err = req.Storage.Delete(ctx, "token/"+fingerprint)
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
	b.reset()
	return nil, nil
}

func (b *datastaxAstraBackend) tokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("renew called")
	resp := &logical.Response{Secret: req.Secret}
	orgId := req.Secret.InternalData["orgId"]
	roleName := req.Secret.InternalData["roleName"]
	entry, err := readRole(ctx, req.Storage, roleName.(string), orgId.(string))
	if err != nil {
		resp.Secret.TTL = 24 * time.Hour
		return resp, errors.New("error getting config data. lease time set to 24h")
	}

	resp.Secret.TTL = entry.TTL
	resp.Secret.MaxTTL = entry.MaxTTL
	return resp, nil
}

const pathCredentialsHelpSyn = `
Generate a AstraCS token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Astra CS token based on a particular role. 
`
