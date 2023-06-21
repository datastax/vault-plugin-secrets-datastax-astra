package datastax_astra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	astraTokenType = "astra_token"
)

// astraToken defines a secret for the astra token
type astraToken struct {
	ClientID    string            `json:"clientId"`
	Secret      string            `json:"secret"`
	OrgID       string            `json:"orgId"`
	RoleName    string            `json:"roleName"`
	Roles       []string          `json:"roles"`
	Token       string            `json:"token"`
	GeneratedOn string            `json:"generatedOn"`
	LogicalName string            `json:"logicalName"`
	Metadata    map[string]string `json:"metadata"`
}

// astraToken defines a secret to store for a given role
// and how it should be revoked or renewed.
func (b *datastaxAstraBackend) astraToken() *framework.Secret {
	return &framework.Secret{
		Type: astraTokenType,
		Fields: map[string]*framework.FieldSchema{
			// The Fields mapping has been intentionally left empty. This is because the
			// schema fields are never used during the token renewal or revocation.
		},
		Renew:  b.tokenRenew,
		Revoke: b.tokenRevoke,
	}
}

func (token *astraToken) ToResponseData() map[string]interface{} {
	return map[string]interface{}{
		"token":       token.Token,
		"clientId":    token.ClientID,
		"orgId":       token.OrgID,
		"roleName":    token.RoleName,
		"logicalName": token.LogicalName,
		"generatedOn": token.GeneratedOn,
		"metadata":    token.Metadata,
	}
}

func createTokenInAstra(c *astraClient, roleEntry *astraRoleEntry, logicalName string, metadata map[string]string) (*astraToken, error) {
	response, err := c.createToken(`{"roles": [` + `"` + roleEntry.RoleId + `"]}`)
	if err != nil {
		return nil, err
	}

	var newToken *astraToken
	err = json.Unmarshal(response, &newToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response '%s', caused by %w", string(response), err)
	}
	// Override the orgId with the one specified by the user so there is no confusion when it is displayed back to them
	// The orgId is only used internally by the plugin and never used when calling out to the Astra API. Astra uses only
	// the clientId to identify the token.
	newToken.OrgID = roleEntry.OrgId
	newToken.RoleName = roleEntry.RoleName
	newToken.LogicalName = logicalName
	newToken.Metadata = metadata

	return newToken, nil
}

func deleteTokenFromAstra(c *astraClient, clientId string) error {
	err := c.deleteToken(clientId)
	if err != nil {
		return err
	}

	return nil
}

func deleteTokenFromStorage(ctx context.Context, s logical.Storage, tokenId string) error {
	err := s.Delete(ctx, "token/"+tokenId)
	if err != nil {
		return err
	}

	return nil
}

func (b *datastaxAstraBackend) tokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	orgIdRaw, ok := req.Secret.InternalData["orgId"]
	if !ok {
		return nil, fmt.Errorf("secret is missing orgId internal data")
	}
	orgId := orgIdRaw.(string)
	roleNameRaw, ok := req.Secret.InternalData["roleName"]
	if !ok {
		return nil, fmt.Errorf("secret is missing role internal data")
	}
	roleName := roleNameRaw.(string)

	roleEntry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role %s: %w", roleName, err)
	}
	if roleEntry == nil {
		return nil, errors.New("unable to find role " + roleName)
	}

	resp := &logical.Response{Secret: req.Secret}

	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
	}
	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
	}

	b.logger.Info(fmt.Sprintf(
		"Renewed lease for token '%s' with TTL: %s and MaxTTL: %s ",
			req.Secret.InternalData["clientId"],
			resp.Secret.TTL,
			resp.Secret.MaxTTL))

	return resp, nil
}

func (b *datastaxAstraBackend) tokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	orgId, ok := req.Secret.InternalData["orgId"]
	if !ok {
		return nil, errors.New("failed to retrieve organisation Id related to token")
	}
	client, err := b.getClient(ctx, req.Storage, orgId.(string))
	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	clientId := ""
	clientIdRaw, ok := req.Secret.InternalData["clientId"]
	if ok {
		clientId, ok = clientIdRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for client Id in secret internal data")
		}
	}

	err = deleteTokenFromAstra(client, clientId)
	if err != nil {
		return nil, fmt.Errorf("error revoking user token: %s", err.Error())
	}

	tokenId := ""
	tokenIdRaw, ok := req.Secret.InternalData["tokenId"]
	if ok {
		tokenId, ok = tokenIdRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for client Id in secret internal data")
		}
	}
	err = deleteTokenFromStorage(ctx, req.Storage, tokenId)

	b.logger.Info(fmt.Sprintf("Revoked lease for token '%s'", clientId))
	return nil, err
}
