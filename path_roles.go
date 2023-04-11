package datastax_astra

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
	"time"
)

const (
	rolePath      = "role"
	defaultMaxTtl = time.Duration(24 * 3600) * time.Second
)

func pathRole(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: rolePath,
		Fields: map[string]*framework.FieldSchema{
			"role_name": {
				Type:        framework.TypeLowerCaseString,
				Description: "The name of the role as it should appear in Vault.",
			},
			"role_id": {
				Type:        framework.TypeString,
				Description: "UUID of the role as seen in Astra.",
			},
			"org_id": {
				Type:        framework.TypeString,
				Description: "UUID of the organization in Astra.",
			},
			"ttl": {
				Type:        framework.TypeString,
				Description: "Default ttl in seconds, minutes or hours for the token. If unset or set to 0, it will default to 24 hours. If this value is bigger than max_ttl, it will be clamped to the max_ttl value. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
			"max_ttl": {
				Type:        framework.TypeString,
				Description: "Maximum ttl in seconds, minutes or hours for the token. If unset or set to 0, it will default to 24 hours. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRoleWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRoleRead,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRoleDelete,
			},
		},
		HelpSynopsis:    pathRoleHelpSynopsis,
		HelpDescription: pathRoleHelpDescription,
	}
}

func readRole(ctx context.Context, s logical.Storage, roleName, orgId string) (*astraRoleEntry, error) {
	if roleName == "" {
		return nil, fmt.Errorf("missing role name")
	}
	if orgId == "" {
		return nil, fmt.Errorf("missing org id")
	}

	role, err := s.Get(ctx, "role/"+orgId+"-"+roleName)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}

	result := &astraRoleEntry{}
	err = role.DecodeJSON(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func saveRole(ctx context.Context, s logical.Storage, roleName, orgId string, role *astraRoleEntry) error {
	entry, err := logical.StorageEntryJSON("role/"+orgId+"-"+roleName, role)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("failed to create storage entry for role")
	}
	err = s.Put(ctx, entry)
	if err != nil {
		return err
	}
	return nil
}

// pathRolesRead makes a request to Vault storage to read a role and return response data
func (b *datastaxAstraBackend) pathRoleRead(ctx context.Context,
	req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("pathRoleRead called")
	role, err := readRole(ctx, req.Storage, d.Get("role_name").(string), d.Get("org_id").(string))
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: role.ToResponseData(),
	}, nil
}

// pathRolesWrite makes a request to Vault storage to update a role based on the attributes passed to the role configuration
func (b *datastaxAstraBackend) pathRoleWrite(ctx context.Context,
	req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("pathRoleWrite called")
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return logical.ErrorResponse("missing role name"), nil
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("missing org id"), nil
	}

	role, err := readRole(ctx, req.Storage, roleName.(string), orgId.(string))
	if err != nil {
		return nil, err
	}
	if role == nil {
		role = &astraRoleEntry{}
		role.RoleName = roleName.(string)
		role.OrgId = orgId.(string)
	}

	createOperation := (req.Operation == logical.CreateOperation)

	roleId, ok := d.GetOk("role_id")
	if ok {
		role.RoleId = roleId.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing role id in role")
	}

	ttl, ok := d.GetOk("ttl")
	if !ok && createOperation {
		ttl = d.Get("ttl")
	}
	parseTtl, _ := time.ParseDuration(ttl.(string))
	role.TTL = time.Duration(parseTtl.Seconds()) * time.Second
	b.logger.Debug(fmt.Sprintf("pathRoleWrite - setting TTL to: %s", role.TTL))

	maxTtl, ok := d.GetOk("max_ttl")
	if !ok && createOperation {
		maxTtl = d.Get("max_ttl")
	}
	parseMaxTtl, _ := time.ParseDuration(maxTtl.(string))
	role.MaxTTL = time.Duration(parseMaxTtl.Seconds()) * time.Second
	b.logger.Debug(fmt.Sprintf("pathRoleWrite - setting MaxTTL to: %s", role.MaxTTL))

	if role.MaxTTL == 0 {
		role.MaxTTL = defaultMaxTtl
	}

	if role.TTL > role.MaxTTL {
		role.TTL = role.MaxTTL
	}

	err = saveRole(ctx, req.Storage, roleName.(string), orgId.(string), role)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// pathRolesDelete makes a request to Vault storage to delete a role
func (b *datastaxAstraBackend) pathRoleDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	b.logger.Debug("pathRoleDelete called")
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return nil, errors.New("role name not provided")
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return nil, errors.New("org_id not provided")
	}

	if err := req.Storage.Delete(ctx, "role/"+orgId.(string)+"-"+roleName.(string)); err != nil {
		return nil, err
	}
	return nil, nil
}

const (
	pathRoleHelpSynopsis    = `Manages the Vault role for generating Astra tokens.`
	pathRoleHelpDescription = `
This path allows you to read and write roles used to generate Astra tokens.
You can configure a role to manage a token by setting the 'name' (role name)
and 'org_id' (Astra organisation id) fields.
`
)
