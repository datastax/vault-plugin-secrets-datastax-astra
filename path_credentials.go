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

	"crypto/sha256"
	"github.com/google/uuid"
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
			logical.ReadOperation:   &framework.PathOperation{Callback: b.pathCredentialsRead},
			logical.CreateOperation: &framework.PathOperation{Callback: b.pathCredentialsWrite},
			logical.UpdateOperation: &framework.PathOperation{Callback: b.debugAndTestPathCredentialsUpdate},
			logical.DeleteOperation: &framework.PathOperation{Callback: b.pathTokenDelete},
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

func calculateTokenFingerprint(token *astraToken) string {
	return calculateTokenFingerprintFromComponent(token.OrgID, token.RoleNickname, token.LogicalName)
}

func calculateTokenFingerprintFromComponent(orgId, role, logicalName string) string {
	tokenRef := fmt.Sprintf("%s:%s:%s", orgId, role, logicalName)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(tokenRef)))
}

func readToken(ctx context.Context, s logical.Storage, fingerprint string) (*astraToken, error) {
	token, err := s.Get(ctx, "token/" + fingerprint)
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

func saveToken(ctx context.Context, token *astraToken, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON("token/" + calculateTokenFingerprint(token), token)
	if err != nil {
		return err
	}
	if err = s.Put(ctx, entry); err != nil {
		return err
	}
	return nil
}

func getTokenFingerprintComponents( d *framework.FieldData) (string, string, string, error) {
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return "", "", "", errors.New("role_name not provided")
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return "", "", "", errors.New("org_id not provided")
	}
	logicalName, ok := d.GetOk("logical_name")
	if !ok {
		return "", "", "", errors.New("logical_name not provided")
	}
	return roleName.(string), orgId.(string), logicalName.(string), nil
}

func matchToken(ctx context.Context, req *logical.Request, d *framework.FieldData) (*astraToken, error) {
	// try to retrieve the token by its fingerprint first
	tokenFingerprint, ok := req.Secret.InternalData["fingerprint"]
	if !ok {
		roleName, orgId, logicalName, err := getTokenFingerprintComponents(d)
		if err == nil {
			tokenFingerprint = calculateTokenFingerprintFromComponent(orgId, roleName, logicalName)
		}
	}
	token, err := readToken(ctx, req.Storage, tokenFingerprint.(string))

	// fallback to matching by client id if no fingerprint match found
	if err != nil {
		clientId, ok := d.GetOk("client_id")

		if ok {
			var tokenList []string
			tokenList, err = listCreds(ctx, req.Storage)
			if err != nil {
				return nil, errors.New("unable to retrieve tokens from vault" + err.Error())
			}
			if len(tokenList) == 0 {
				return nil, errors.New("no tokens found in vault")
			}
			for i := 0; i < len(tokenList); i++ {
				token, err = readToken(ctx, req.Storage, tokenList[i])
				if err != nil {
					return nil, errors.New("unable to retrieve token from local store" + err.Error())
				}
				if token == nil {
					continue
				}
				if  token.ClientID == clientId {
					break
				}
			}
		}
	}
	if token == nil {
		return nil, errors.New("unable to find token")
	}
	return token, nil
}

func (b *datastaxAstraBackend) debugAndTestPathCredentialsUpdate(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	//b.lock.Lock()
	//defer b.lock.Unlock()
	b.logger.Debug("Update called")

	roleName, ok := d.GetOk("role_name")
	if !ok {
		b.logger.Debug("role_name not provided")
		return nil, errors.New("role_name not provided")
	}
	b.logger.Debug(fmt.Sprintf("role_name: %s", roleName.(string)))

	orgId, ok := d.GetOk("org_id")
	if !ok {
		b.logger.Debug("org_id not provided")
		return nil, errors.New("org_id not provided")
	}
	b.logger.Debug(fmt.Sprintf("org_id: %s", orgId.(string)))

	logicalName, ok := d.GetOk("logical_name")
	if !ok {
		b.logger.Debug("logical_name not provided")
		return nil, errors.New("logical_name not provided")
	}
	b.logger.Debug(fmt.Sprintf("logical_name: %s", logicalName.(string)))

	tok, err := readToken(ctx, req.Storage, logicalName.(string))
	if err != nil {
		b.logger.Debug(fmt.Sprintf("unable to retrieve info for logical_name: %s. %s", logicalName.(string), err.Error()))
		return nil, errors.New(err.Error())
	}
	if tok != nil {
		b.logger.Debug(fmt.Sprintf("token already exists for role: %s", logicalName.(string)))
		tokJsonData, err := json.Marshal(tok.toResponseData())
		if err != nil {
			b.logger.Debug(fmt.Sprintf("could not marshal json: %s", err.Error()))
			return nil, errors.New(err.Error())
		}
		b.logger.Debug(fmt.Sprintf("json data: %s", tokJsonData))
		return nil, errors.New("token already exists for role")
	} else {
		b.logger.Debug(fmt.Sprintf("no token found for role: %s", logicalName.(string)))
	}
	tokenId := uuid.New().String()
	newToken := astraToken {
		RoleNickname: roleName.(string),
		ClientID: tokenId,
		LogicalName: logicalName.(string),
		OrgID: orgId.(string),
	}

	err = saveToken(ctx, &newToken, req.Storage)
	if err != nil {
		b.logger.Debug(fmt.Sprintf("failed to save token. %s", err.Error()))
		return nil, err
	} else {
		b.logger.Debug(fmt.Sprintf("token saved: %s", tokenId))
	}

	time.Sleep(30 * time.Second)
	return nil, nil
}

// pathCredentialsRead reads a token from vault.
func (b *datastaxAstraBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	token, err := matchToken(ctx, req, d)

	if err != nil {
		return nil, errors.New(err.Error())
	}

	return &logical.Response{Data: token.toResponseData()}, nil
}

func (b *datastaxAstraBackend) pathCredentialsWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("Write called")
	token, err := matchToken(ctx, req, d)
	if token != nil {
		return nil, errors.New("token already exists for role")
	}
	roleName, orgId, logicalName, err := getTokenFingerprintComponents(d)
	entry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, errors.New("role does not exist. add role first")
	}
	payload := strings.NewReader(`{
    "roles": [` + `"` + entry.RoleId + `"]}`)
	conf, err := getConfig(ctx, req.Storage, orgId)
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
	var newToken *astraToken
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		msg := "error reading ioutil " + err.Error()
		return nil, errors.New(msg)
	}
	err = json.Unmarshal(body, &newToken)
	if err != nil {
		msg := " Unmarshal failed " + err.Error()
		return nil, errors.New(msg)
	}
	metadata, ok, err := d.GetOkErr("metadata")
	if err != nil {
		return logical.ErrorResponse(fmt.Sprintf("failed to parse metadata: %v", err)), nil
	}
	if ok {
		newToken.Metadata = metadata.(map[string]string)
	}
	newToken.LogicalName = logicalName
	newToken.RoleNickname = roleName
	internalData := map[string]interface{}{
		"token":    	newToken.Token,
		"metadata": 	newToken.Metadata,
		"orgId":    	newToken.OrgID,
		"fingerprint":	calculateTokenFingerprint(newToken),
	}

	err = saveToken(ctx, newToken, req.Storage)
	if err != nil {
		return nil, err
	}
	resp := b.Secret(astraTokenType).Response(newToken.toResponseData(), internalData)
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
	token, err := matchToken(ctx, req, d)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	tokenFingerprint, ok := req.Secret.InternalData["fingerprint"]
	if !ok {
		return nil, errors.New("unable to retrieve token fingerprint")
	}
	err = req.Storage.Delete(ctx, "token/" + tokenFingerprint.(string))
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
	b.reset()
	return nil, nil
}

func (b *datastaxAstraBackend) tokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	return b.pathTokenDelete(ctx, req, d)
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
	if parsedRenewalTime > parseMaxLeaseTime {
		parsedRenewalTime = parseMaxLeaseTime
	}
	resp.Secret.TTL = parsedRenewalTime
	resp.Secret.MaxTTL = parseMaxLeaseTime
	return resp, rtnErr
}

const pathCredentialsHelpSyn = `
Generate a AstraCS token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Astra CS token based on a particular role. 
`
