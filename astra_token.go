package datastax_astra

import (
	"github.com/hashicorp/vault/sdk/framework"
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
	Metadata     map[string]string `json:"metadata"`
	LogicalName  string            `json:"logicalName"`
	RoleNickname string            `json:"roleNickname"`
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
		},
	}
}

func (token *astraToken) toResponseData() map[string]interface{} {
	respData := map[string]interface{}{
		"token":       token.Token,
		"clientId":    token.ClientID,
		"orgId":       token.OrgID,
		"metadata":    token.Metadata,
		"generatedOn": token.GeneratedOn,
		"logicalName": token.LogicalName,
		"roleName":    token.RoleNickname,
	}
	return respData
}
