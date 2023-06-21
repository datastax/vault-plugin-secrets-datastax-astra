package datastax_astra

import (
	"context"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathCredentialsList(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: "org/tokens/?$",
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathCredsList,
				Summary:  "List all tokens.",
			},
		},
	}
}

func (b *datastaxAstraBackend) pathCredsList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	return b.pathObjectList(ctx, req, CredentialsPathList)
}
