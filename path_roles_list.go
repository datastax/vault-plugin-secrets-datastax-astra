package datastax_astra

import (
	"context"
	"errors"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	pathRoleListHelpSynopsis    = `List the existing roles in the given Astra organisation`
	pathRoleListHelpDescription = `Roles will be listed by their name.`
	rolesListPath = "roles/?$"

)

func pathRoleList(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: rolesListPath,
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathRolesList,
				Summary:  "List all roles.",
			},
		},
		HelpSynopsis:    pathRoleListHelpSynopsis,
		HelpDescription: pathRoleListHelpDescription,
	}
}

func (b *datastaxAstraBackend) pathRolesList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	entry, err := listOrgRoles(ctx, req.Storage)
		if err != nil {
			return nil, errors.New("no roles found " + err.Error())
		}
	if len(entry) == 0 {
		return nil, errors.New("no roles found")
	}
return logical.ListResponse(entry), nil
}

func listOrgRoles(ctx context.Context, s logical.Storage)([]string, error) {
	objList, err := s.List(ctx, "role/")
	if err != nil {
		return nil, errors.New("failed to load role list")
	}
	if len(objList) == 0 {
		return nil, errors.New("no roles found")
	}
	return objList, nil
}
