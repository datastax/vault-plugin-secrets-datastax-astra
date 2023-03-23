package datastax_astra

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

// pathCredentials extends the Vault API with a `/token` endpoint for a role.
func pathCredentials(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: "org/token",
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
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
			"logical_name": {
				Type:        framework.TypeLowerCaseString,
				Description: "Logical name to reference this token by. Ignored if running in sidecar mode",
				Required:    false,
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
				Description: "ClientId for the token. Ignored if running in sidecar mode",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Sensitive: false,
				},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredentialsRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCredentialsUpdate,
			},
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

func readTokenUsingClientId(ctx context.Context, s logical.Storage, clientId string) (*astraToken, error) {
	credsList, err := s.List(ctx, "token/")
	if err != nil {
		return nil, errors.New("failed to get token list: " + err.Error())
	}
	if len(credsList) == 0 {
		return nil, errors.New("no tokens found")
	}
	for i := 0; i < len(credsList); i++ {
		token, err := readToken(ctx, s, credsList[i])
		if err != nil {
			return nil, errors.New("unable to retrieve token information: " + err.Error())
		}

		if token.ClientID == clientId {
			return token, nil
		}
	}
	return nil, nil
}

func calculateTokenId(roleEntry *astraRoleEntry, logicalName string) string {
	tokenRef := fmt.Sprintf("%s:%s:%s", roleEntry.OrgId, roleEntry.RoleName, logicalName)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(tokenRef)))
}

func readToken(ctx context.Context, s logical.Storage, tokenId string) (*astraToken, error) {
	token, err := s.Get(ctx, "token/"+tokenId)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}
	result := &astraToken{}
	err = token.DecodeJSON(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func saveToken(ctx context.Context, s logical.Storage, token *astraToken, tokenId string) error {
	entry, err := logical.StorageEntryJSON("token/"+tokenId, token)
	if err != nil {
		return err
	}
	err = s.Put(ctx, entry)
	if err != nil {
		return err
	}
	return nil
}

func (b *datastaxAstraBackend) createToken(ctx context.Context, s logical.Storage, roleEntry *astraRoleEntry, logicalName string, metadata map[string]string) (*astraToken, error) {
	client, err := b.getClient(ctx, s, roleEntry.OrgId)
	if err != nil {
		return nil, err
	}

	var token *astraToken

	token, err = createTokenInAstra(client, roleEntry, logicalName, metadata)
	if err != nil {
		errMsg := "error creating Astra token: " + err.Error()
		b.logger.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	if token == nil {
		errMsg := "failed to create Astra token"
		b.logger.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	// If logicalName is a non-empty string we will use that along with the org ID and role name to store the token,
	//	otherwise use the token clientID. In standard mode the logicalName will be set a non-empty string, but in
	//	sidecar mode it is set to an empty string.
	var tokenId string
	if logicalName != "" {
		tokenId = calculateTokenId(roleEntry, logicalName)
	} else {
		tokenId = token.ClientID
	}

	err = saveToken(ctx, s, token, tokenId)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (b *datastaxAstraBackend) generateTokenResponse(token *astraToken, tokenId string, roleEntry *astraRoleEntry) (*logical.Response, error) {
	resp := b.Secret(astraTokenType).Response(
		token.ToResponseData(),
		map[string]interface{}{
			"orgId":    roleEntry.OrgId,
			"clientId": token.ClientID,
			"roleName": roleEntry.RoleName,
			"tokenId":  tokenId,
		})

	if roleEntry.TTL > 0 {
		resp.Secret.TTL = roleEntry.TTL
	}

	if roleEntry.MaxTTL > 0 {
		resp.Secret.MaxTTL = roleEntry.MaxTTL
	}

	resp.Secret.Renewable = true

	b.logger.Info(fmt.Sprintf("Created token '%s' with TTL: %s and MaxTTL: %s", token.ClientID, roleEntry.TTL, roleEntry.MaxTTL))
	return resp, nil
}

func (b *datastaxAstraBackend) pathCredentialsStandardMode(ctx context.Context, req *logical.Request, d *framework.FieldData, orgId string, readOnly bool) (*logical.Response, error) {
	clientIdRaw, ok := d.GetOk("client_id")
	if ok && readOnly {
		clientId := clientIdRaw.(string)
		token, err := readTokenUsingClientId(ctx, req.Storage, clientId)
		if token == nil {
			if err == nil {
				// No error means no token existed with that clientId
				return nil, errors.New("no token found with clientId " + clientId)
			} else {
				return nil, err
			}
		}
		return &logical.Response{Data: token.ToResponseData()}, nil
	}

	roleNameRaw, ok := d.GetOk("role_name")
	if !ok {
		return logical.ErrorResponse("please provide a role_name argument"), nil
	}
	roleName := roleNameRaw.(string)
	logicalNameRaw, ok := d.GetOk("logical_name")
	if !ok {
		return logical.ErrorResponse("please provide a logical_name argument"), nil
	}
	logicalName := logicalNameRaw.(string)
	// In standard mode logical_name must be set to a non-empty string value, this is
	// so we can calculate a tokenId when storing it
	if logicalName == "" {
		return nil, errors.New("logical_name is set to an empty string; a non-empty value must be provided")
	}
	roleEntry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, errors.New("error retrieving role " + roleName + ": " + err.Error())
	}
	if roleEntry == nil {
		return nil, errors.New("unable to find role " + roleName)
	}
	if roleEntry.RoleId == "" {
		return nil, nil
	}

	tokenId := calculateTokenId(roleEntry, logicalName)
	token, err := readToken(ctx, req.Storage, tokenId)
	if err != nil {
		return nil, errors.New("error attempting to retrieve token for org ID" + orgId + ", role " + roleName + ", with logical name " + logicalName + ": " + err.Error())
	}

	// Return behaviour depends on whether we are reading or writing
	if readOnly {
		if token == nil {
			return nil, errors.New("unable to find token for org ID" + orgId + ", role " + roleName + ", with logical name " + logicalName)
		}
		return &logical.Response{Data: token.ToResponseData()}, nil
	}

	// If we are writing (not readOnly), create the token and generate a secret response
	if token != nil {
		return nil, errors.New("token already exists for org ID " + orgId + ", role " + roleName + ", with logical name " + logicalName)
	}

	metadata := make(map[string]string)
	metadataRaw, ok, err := d.GetOkErr("metadata")
	if err != nil {
		return nil, errors.New("error parsing metadata: " + err.Error())
	}
	if ok {
		metadata = metadataRaw.(map[string]string)
	}

	token, err = b.createToken(ctx, req.Storage, roleEntry, logicalName, metadata)
	if err != nil {
		return nil, err
	}
	return b.generateTokenResponse(token, tokenId, roleEntry)
}

func (b *datastaxAstraBackend) pathCredentialsSidecarMode(ctx context.Context, req *logical.Request, d *framework.FieldData, orgId string) (*logical.Response, error) {
	roleNameRaw, ok := d.GetOk("role_name")
	if !ok {
		return logical.ErrorResponse("please provide a role_name argument"), nil
	}
	roleName := roleNameRaw.(string)

	roleEntry, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, errors.New("error retrieving role " + roleName + ": " + err.Error())
	}
	if roleEntry == nil {
		return nil, errors.New("unable to find role " + roleName)
	}
	if roleEntry.RoleId == "" {
		return nil, nil
	}

	metadata := make(map[string]string)
	metadataRaw, ok, err := d.GetOkErr("metadata")
	if err != nil {
		return nil, errors.New("error parsing metadata: " + err.Error())
	}
	if ok {
		metadata = metadataRaw.(map[string]string)
	}

	// In sidecar mode logical_name is ignored, so make it an empty string
	// to force storing the token using its ClientId
	token, err := b.createToken(ctx, req.Storage, roleEntry, "", metadata)
	if err != nil {
		return nil, err
	}

	return b.generateTokenResponse(token, token.ClientID, roleEntry)
}

func (b *datastaxAstraBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	orgIdRaw, ok := d.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}
	orgId := orgIdRaw.(string)
	config, err := readConfig(ctx, req.Storage, orgId)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("unable to find config for org ID" + orgId)
	}

	switch config.CallerMode {
	case StandardCallerMode:
		return b.pathCredentialsStandardMode(ctx, req, d, orgId, true)
	case SidecarCallerMode:
		return b.pathCredentialsSidecarMode(ctx, req, d, orgId)
	}
	return nil, nil
}

func (b *datastaxAstraBackend) pathCredentialsUpdate(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	orgIdRaw, ok := d.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}
	orgId := orgIdRaw.(string)
	config, err := readConfig(ctx, req.Storage, orgId)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("unable to find config for org ID" + orgId)
	}

	switch config.CallerMode {
	case StandardCallerMode:
		return b.pathCredentialsStandardMode(ctx, req, d, orgId, false)
	case SidecarCallerMode:
		return b.pathCredentialsSidecarMode(ctx, req, d, orgId)
	}
	return nil, nil
}

const pathCredentialsHelpSyn = `
Generate a AstraCS token from a specific Vault role.
`

const pathCredentialsHelpDesc = `
This path generates a Astra CS token based on a particular role. 
`
