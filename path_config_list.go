package datastax_astra

import (
    "context"
    "github.com/hashicorp/vault/sdk/framework"
    "github.com/hashicorp/vault/sdk/logical"
)

const (
    pathConfigListHelpSynopsis = `List the existing configurations in the given Astra organisation.`
    pathConfigListHelpDescription = `Configurations will be listed by their name.`
)

func pathConfigList(b *datastaxAstraBackend) *framework.Path {
    return &framework.Path{
        Pattern: "configs/?$",
        Operations: map[logical.Operation]framework.OperationHandler{
            logical.ListOperation: &framework.PathOperation{
                Callback: b.pathConfigList,
                Summary:  "List all configs.",
            },
        },
        HelpSynopsis:    pathConfigListHelpSynopsis,
        HelpDescription: pathConfigListHelpDescription,
    }
}

func (b *datastaxAstraBackend) pathConfigList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
    return b.pathObjectList(ctx, req, ConfigPathList)
}
