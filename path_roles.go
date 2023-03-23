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
	roleStoragePath 		= "role/"
	roleStorageKeyDelimiter		= ":"
	defaultTtl   			= time.Duration(24*3600) * time.Second
	defaultMaxTtl			= time.Duration(24*3600) * time.Second
)

func pathRole(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: "role",
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
				Type:        framework.TypeDurationSecond,
				Description: "Default ttl in seconds, minutes or hours for the token. If unset or set to 0, it will default to 24 hours. If this value is bigger than max_ttl, it will be clamped to the max_ttl value. Use the duration initials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
			},
			"max_ttl": {
				Type:        framework.TypeDurationSecond,
				Description: "Maximum ttl in seconds, minutes or hours for the token. If unset or set to 0, it will default to 24 hours. Use the duration initials after the number. for e.g. 5s, 5m, 5h",
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
		ExistenceCheck:  b.pathRoleExistenceCheck,
		HelpSynopsis:    pathRoleHelpSynopsis,
		HelpDescription: pathRoleHelpDescription,
	}
}

func readRoleUsingKey(ctx context.Context, s logical.Storage, roleKey string) (*astraRoleEntry, error) {
	if roleKey == "" {
		return nil, errors.New("role key is an empty string")
	}

	entry, err := s.Get(ctx, roleStoragePath+roleKey)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	role := &astraRoleEntry{}
	err = entry.DecodeJSON(role)
	if err != nil {
		return nil, errors.New("error retrieving role " + roleKey + ": " + err.Error())
	}

	return role, nil
}

func readRole(ctx context.Context, s logical.Storage, roleName, orgId string) (*astraRoleEntry, error) {
	if roleName == "" {
		return nil, errors.New("role name is an empty string")
	}
	if orgId == "" {
		return nil, errors.New("org ID is an empty string")
	}

	return readRoleUsingKey(ctx, s, orgId+roleStorageKeyDelimiter+roleName)
}

func saveRole(ctx context.Context, s logical.Storage, role *astraRoleEntry) error {
	entry, err := logical.StorageEntryJSON(roleStoragePath+role.OrgId+roleStorageKeyDelimiter+role.RoleName, role)
	if err != nil {
		return err
	}
	if entry == nil {
		return errors.New("error creating role entry " + role.RoleName + ": " + err.Error())
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
	roleNameRaw, ok := d.GetOk("role_name")
	if !ok {
		return logical.ErrorResponse("please provide a role_name argument"), nil
	}
	roleName := roleNameRaw.(string)
	orgIdRaw, ok := d.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}
	orgId := orgIdRaw.(string)

	role, err := readRole(ctx, req.Storage, roleName, orgId)
	if err != nil {
		return nil, err
	}

	createOperation := req.Operation == logical.CreateOperation

	if role == nil {
		role = &astraRoleEntry{}
		role.RoleName = roleName
		role.OrgId = orgId
	}

	roleId, ok := d.GetOk("role_id")
	if ok {
		role.RoleId = roleId.(string)
	} else if !ok && createOperation {
		return logical.ErrorResponse("please provide a role_id argument"), nil
	}

	ttl, ok := d.GetOk("ttl")
	if ok {
		if ttl.(int) > 0 {
			role.TTL = time.Duration(ttl.(int)) * time.Second
		} else {
			role.TTL = defaultTtl
			b.logger.Warn(fmt.Sprintf("A ttl value of 0 was provided; defaulting to %s", role.TTL))
		}
	} else if !ok && createOperation {
		role.TTL = defaultTtl
		b.logger.Warn(fmt.Sprintf("No ttl value provided; defaulting to %s", role.TTL))
	}

	maxTtl, ok := d.GetOk("max_ttl")
	if ok {
		if maxTtl.(int) > 0 {
			role.MaxTTL = time.Duration(maxTtl.(int)) * time.Second
		} else {
			role.MaxTTL = defaultMaxTtl
			b.logger.Warn(fmt.Sprintf("A max_ttl value of 0 was provided; defaulting to %s", role.MaxTTL))
		}
	} else if !ok && createOperation {
		role.MaxTTL = defaultMaxTtl
		b.logger.Warn(fmt.Sprintf("No max_ttl value provided; defaulting to %s", role.MaxTTL))
	}

	if role.TTL > role.MaxTTL {
		role.TTL = role.MaxTTL
		b.logger.Warn(fmt.Sprintf("The ttl value provided is greater than max_ttl (%s); setting ttl to %s", role.MaxTTL, role.TTL))
	}

	err = saveRole(ctx, req.Storage, role)
	if err != nil {
		return nil, err
	}

	b.logger.Info(fmt.Sprintf(
		"%s role %s with token settings TTL: %s and MaxTTL: %s",
		operationToStringVerb(req.Operation),
		roleName,
		role.TTL,
		role.MaxTTL))

	return nil, nil
}

// pathRolesDelete makes a request to Vault storage to delete a role
func (b *datastaxAstraBackend) pathRoleDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName, ok := d.GetOk("role_name")
	if !ok {
		return logical.ErrorResponse("please provide a role_name argument"), nil
	}
	orgId, ok := d.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}

	err := req.Storage.Delete(ctx, roleStoragePath+orgId.(string)+roleStorageKeyDelimiter+roleName.(string))
	if err != nil {
		return nil, err
	}

	b.logger.Info("Deleted role " + roleName.(string))
	return nil, nil
}

func (b *datastaxAstraBackend) pathRoleExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	// The existence check determines whether the logical.Request.Operation value is Create or Update. In this case
	//	we will skip the argument validation as it will be performed in the write path implementation.
	obj, err := req.Storage.Get(ctx, roleStoragePath + data.Get("org_id").(string) + roleStorageKeyDelimiter + data.Get("role_name").(string))
	if err != nil {
		return false, errors.New("error retrieving role from storage for existence check: " + err.Error())
	}

	return obj != nil, nil
}

const (
	pathRoleHelpSynopsis    = `Manages the Vault role for generating Astra tokens.`
	pathRoleHelpDescription = `
This path allows you to read and write roles used to generate Astra tokens.
You can configure a role to manage a token by setting the 'name' (role name)
and 'org_id' (Astra organisation id) fields.
`
)
