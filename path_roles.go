package datastax_astra

import (
	"context"
	"errors"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	rolePath      = "role"
	rolesListPath = "roles/?"
)

func pathRole(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: rolePath,
		Fields: map[string]*framework.FieldSchema{
			"role": {
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

func (b *datastaxAstraBackend) operationRoleExistenceCheck(ctx context.Context,
	req *logical.Request, data *framework.FieldData) (bool, error) {
	entry, err := readRole(ctx, req.Storage, data.Get("role").(string), data.Get("org_id").(string))
	if err != nil {
		return false, err
	}
	return entry != nil, nil
}

func readRole(ctx context.Context, s logical.Storage, roleName, orgId string) (*roleEntry, error) {
	role, err := s.Get(ctx, "role/"+orgId+"-"+roleName)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	result := &roleEntry{}
	if err := role.DecodeJSON(result); err != nil {
		return nil, err
	}

	return result, nil
}

func saveRole(ctx context.Context, role *roleEntry, s logical.Storage, roleName, orgId string) error {
	entry, err := logical.StorageEntryJSON("role/"+orgId+"-"+roleName, role)
	if err != nil {
		return err
	}
	if err = s.Put(ctx, entry); err != nil {
		return err
	}
	return nil
}

// pathRolesRead makes a request to Vault storage to read a role and return response data
func (b *datastaxAstraBackend) pathRoleRead(ctx context.Context,
	req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	role, err := readRole(ctx, req.Storage, data.Get("role").(string), data.Get("org_id").(string))
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
	req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get("role").(string)
	orgId := data.Get("org_id").(string)
	roleId := data.Get("role_id").(string)
	role := &roleEntry{
		RoleId: roleId,
		OrgId:  orgId,
		Name:   roleName,
	}
	err := saveRole(ctx, role, req.Storage, roleName, orgId)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// pathRolesDelete makes a request to Vault storage to delete a role
func (b *datastaxAstraBackend) pathRoleDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName, ok := d.GetOk("role")
	if !ok {
		return nil, errors.New("role name not provided")
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return nil, errors.New("role name not provided")
	}

	if err := req.Storage.Delete(ctx, "role/"+orgId.(string)+"-"+roleName.(string)); err != nil {
		return nil, err
	}
	return nil, nil
}

//TODO fix below
const (
	pathRoleHelpSynopsis    = `Manages the Vault role for generating Astra tokens.`
	pathRoleHelpDescription = `
This path allows you to read and write roles used to generate Astra tokens.
You can configure a role to manage a token by setting the role_name field.
`
)
