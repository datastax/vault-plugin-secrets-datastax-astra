package datastax_astra

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	credsPath     = "org/token"
	credsListPath = "org/tokens/?"
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
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   	b.pathCredentialsRead,
			logical.UpdateOperation:	b.pathCredentialsRead,
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

func (b *datastaxAstraBackend) createToken(ctx context.Context, s logical.Storage, roleEntry *astraRoleEntry) (*astraToken, error) {
	b.logger.Debug("createToken called")
	client, err := b.getClient(ctx, s, roleEntry.OrgId)
	if err != nil {
		return nil, err
	}

	var token *astraToken

	token, err = createToken(client, roleEntry.RoleId, "", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Astra token: %w", err)
	}

	if token == nil {
		return nil, errors.New("error creating Astra token")
	}

	b.logger.Debug(fmt.Sprintf("createToken - token generated: %+v", token.toResponseData()))
	return token, nil
}

func (b *datastaxAstraBackend) readToken(ctx context.Context, req *logical.Request, roleEntry *astraRoleEntry) (*logical.Response, error) {
	b.logger.Debug("readToken called")
	token, err := b.createToken(ctx, req.Storage, roleEntry)
	if err != nil {
		return nil, err
	}

	resp := b.Secret(astraTokenType).Response(
		token.toResponseData(),
		map[string]interface{}{
			"token": token.Token,
			"orgId": token.OrgID,
			"clientId": token.ClientID,
			"roleName": roleEntry.RoleName,
	})

	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
		b.logger.Debug(fmt.Sprintf("readToken - set logical.Response.Secret.TTL to: %d", roleEntry.TTL))
	}

	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
		b.logger.Debug(fmt.Sprintf("readToken - set logical.Response.Secret.MaxTTL to: %d", roleEntry.MaxTTL))
	}

	return resp, nil
}

// pathCredentialsRead reads a token from vault.
func (b *datastaxAstraBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("pathCredentialsRead called")
	roleName := d.Get("role_name").(string)
	orgId := d.Get("org_id").(string)

	roleEntry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}
	if roleEntry == nil {
		return nil, errors.New("error retrieving role: role is nil")
	}
	if roleEntry.RoleId == "" {
		return nil, nil
	}

	b.logger.Debug(fmt.Sprintf("pathCredentialsRead - found role: %s (%s)", roleEntry.RoleName, roleEntry.RoleId))

	return b.readToken(ctx, req, roleEntry)
}

const pathCredentialsHelpSyn = `
Generate a AstraCS token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Astra CS token based on a particular role. 
`
