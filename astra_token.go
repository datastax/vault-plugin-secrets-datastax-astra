package datastax_astra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	astraTokenType = "astra_token"
)

// astraToken defines a secret for the astra token
type astraToken struct {
	ClientID     string            `json:"clientId"`
	Secret       string            `json:"secret"`
	OrgID        string            `json:"orgId"`
	Roles        []string          `json:"roles"`
	Token        string            `json:"token"`
	GeneratedOn  string            `json:"generatedOn"`
	LogicalName  string            `json:"logicalName"`
	Metadata     map[string]string `json:"metadata"`
}

// astraToken defines a secret to store for a given role
// and how it should be revoked or renewed.
func (b *datastaxAstraBackend) astraToken() *framework.Secret {
	return &framework.Secret{
		Type: astraTokenType,
		Fields: map[string]*framework.FieldSchema{
			"token": {
				Type:        framework.TypeString,
				Description: "Astra Token",
			},
			"clientId": {
				Type:        framework.TypeString,
				Description: "Client Id associated with the Astra Token",
			},
			"orgId": {
				Type:        framework.TypeString,
				Description: "The organisation Id that that Astra Token belongs to",
			},
		},
		Renew: b.tokenRenew,
		Revoke: b.tokenRevoke,
	}
}

func (token *astraToken) toResponseData() map[string]interface{} {
	return map[string]interface{}{
		"clientId":    token.ClientID,
		"orgId":       token.OrgID,
		"token":       token.Token,
		"generatedOn": token.GeneratedOn,
		"logicalName": token.LogicalName,
		"metadata":    token.Metadata,
	}
}


func createToken(c *astraClient, roleId, logicalName string, metadata map[string]string) (*astraToken, error) {
	response, err := c.createToken(`{"roles": [` + `"` + roleId + `"]}`)
	if err != nil {
		return nil, fmt.Errorf("error creating Astra token: %w", err)
	}

	var newToken *astraToken
	err = json.Unmarshal(response, &newToken)
	if err != nil {
		msg := " Unmarshal failed " + err.Error()
		return nil, errors.New(msg)
	}
	newToken.LogicalName = logicalName
	newToken.Metadata = metadata

	return newToken, nil
}

func deleteToken(c *astraClient, clientId string) error {
	err := c.deleteToken(clientId)

	if err != nil {
		return err
	}

	return nil
}

func (b *datastaxAstraBackend) tokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("tokenRenew called")
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

	configData, err := getConfig(ctx, req.Storage, orgId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving config for orgId %s: %w", orgId, err)
	}
	if configData == nil {
		return nil, fmt.Errorf("error retrieving config data for orgId %s: config is nil", orgId)
	}

	roleEntry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role %s in orgId %s : %w", roleName, orgId, err)
	}
	if roleEntry == nil {
		return nil, fmt.Errorf("error retrieving role %s in orgId %s: role is nil", roleName, orgId)
	}

	resp := &logical.Response{Secret: req.Secret}

	var newTTL time.Duration
	if configData.DefaultLeaseRenewTime > 0 {
		if configData.DefaultLeaseRenewTime > roleEntry.MaxTTL {
			newTTL = roleEntry.MaxTTL
		} else {
			newTTL = configData.DefaultLeaseRenewTime
		}
	} else {
		newTTL = roleEntry.TTL
	}
	if roleEntry.TTL > 0 {
		resp.Secret.TTL = newTTL
		b.logger.Debug(fmt.Sprintf("tokenRenew - set logical.Response.Secret.TTL to: %s", newTTL))
	}
	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
		b.logger.Debug(fmt.Sprintf("tokenRenew - set logical.Response.Secret.MaxTTL to: %s", roleEntry.MaxTTL))
	}

	return resp, nil
}


func (b *datastaxAstraBackend) tokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("tokenRevoke called")
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

	b.logger.Debug(fmt.Sprintf("tokenRevoke - deleting token: %s", clientId))
	if err := deleteToken(client, clientId); err != nil {
		return nil, fmt.Errorf("error revoking user token: %w", err)
	}
	return nil, nil
}
