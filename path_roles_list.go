package datastax_astra

import (
	"context"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	pathRoleListHelpSynopsis    = `List the existing roles in the given Astra organisation.`
	pathRoleListHelpDescription = `Roles will be listed by their name.`
)

func pathRoleList(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: "roles/?$",
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
	return b.pathObjectList(ctx, req, RolePathList)
}
