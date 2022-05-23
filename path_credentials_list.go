package datastax_astra

import (
	"context"
	"errors"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathCredentialsList(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: credsListPath,
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathCredsList,
				Summary:  "List all tokens.",
			},
		},
	}
}

func (b *datastaxAstraBackend) pathCredsList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	credsList, err := listCreds(ctx, req.Storage)
	if err != nil {
		return nil, errors.New("could not find tokens. error: " + err.Error())
	}
	if len(credsList) == 0 {
		return nil, errors.New("no tokens found")
	}
	keyInfo := make(map[string]interface{})
	for i := 0; i < len(credsList); i++ {
		token, err := readToken(ctx, req.Storage, credsList[i])
		if err != nil {
			return nil, errors.New("could not list tokens. error: " + err.Error())
		}
		keyInfo[token.ClientID] = token.toResponseData()
	}
	return logical.ListResponseWithInfo(credsList, keyInfo), nil
}

func listCreds(ctx context.Context, s logical.Storage) ([]string, error) {
	objList, err := s.List(ctx, "token/")
	if err != nil {
		return nil, errors.New("failed to load tokens list")
	}
	if len(objList) == 0 {
		return nil, errors.New("no tokens found")
	}
	return objList, nil
}
