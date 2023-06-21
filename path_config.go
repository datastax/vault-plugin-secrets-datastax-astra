package datastax_astra

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	configStoragePath = "config/"
)

func pathConfig(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: "config",
		Fields: map[string]*framework.FieldSchema{
			"url": {
				Type:        framework.TypeString,
				Description: "The url to access astra",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "url",
					Sensitive: false,
				},
			},
			"astra_token": {
				Type:        framework.TypeString,
				Description: "token",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "AstraToken",
					Sensitive: true,
				},
			},
			"org_id": {
				Type:        framework.TypeString,
				Description: "UUID of organization",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "org_id",
					Sensitive: false,
				},
			},
			"logical_name": {
				Type:        framework.TypeString,
				Description: "logical name of config",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "logical_name",
					Sensitive: false,
				},
			},
			"caller_mode": {
				Type:        framework.TypeString,
				Description: "what type of client will be calling the API. Valid values are 'standard' (default) and 'sidecar'",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "caller_mode",
					Sensitive: false,
				},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
			},
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
			},
		},
		ExistenceCheck:  b.pathConfigExistenceCheck,
		HelpSynopsis:    pathConfigHelpSynopsis,
		HelpDescription: pathConfigHelpDescription,
	}
}

// pathConfigRead reads the configuration and outputs information.
func (b *datastaxAstraBackend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgId := data.Get("org_id").(string)
	logicalName := data.Get("logical_name").(string)
	if logicalName != "" && orgId == "" {
		configs, err := listConfig(ctx, req.Storage)
		if err != nil {
			return nil, errors.New("error retrieving configs: " + err.Error())
		}
		if len(configs) == 0 {
			return nil, errors.New("no configs found")
		}
		for _, orgId := range configs {
			m, err := readConfig(ctx, req.Storage, orgId)
			if err != nil {
				return nil, errors.New("error retrieving config for org ID " + orgId + ": " + err.Error())
			}
			if m == nil {
				return nil, errors.New("unable to find config for org ID " + orgId)
			}
			if m.LogicalName == logicalName {
				return &logical.Response{
					Data: m.ToResponseData(),
				}, nil
			}
		}
		return nil, errors.New("unable to find config for logical name " + logicalName)
	}
	if orgId != "" {
		config, err := readConfig(ctx, req.Storage, orgId)
		if err != nil {
			return nil, err
		}
		if config == nil {
			return nil, errors.New("unable to find config for org ID " + orgId)
		}
		return &logical.Response{
			Data: config.ToResponseData(),
		}, nil
	}
	return logical.ErrorResponse("please provide an org_id or a logical_name argument"), nil
}

// pathConfigWrite updates the configuration for the backend
func (b *datastaxAstraBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgIdRaw, ok := data.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}
	orgId := orgIdRaw.(string)

	config, err := readConfig(ctx, req.Storage, orgId)
	if err != nil {
		return nil, err
	}

	createOperation := req.Operation == logical.CreateOperation

	if config == nil {
		if !createOperation {
			return nil, errors.New("unable to find config for org ID " + orgId + " during update operation")
		}
		config = &astraConfig{}
		config.OrgId = orgId
	}

	token, ok := data.GetOk("astra_token")
	if ok {
		config.AstraToken = token.(string)
	} else if !ok && createOperation {
		return logical.ErrorResponse("please provide an astra_token argument"), nil
	}
	url, ok := data.GetOk("url")
	if ok {
		config.URL = url.(string)
	} else if !ok && createOperation {
		return logical.ErrorResponse("please provide a url argument"), nil
	}
	logicalName, ok := data.GetOk("logical_name")
	if ok {
		config.LogicalName = logicalName.(string)
	}
	callerModeRaw, ok := data.GetOk("caller_mode")
	if ok {
		callerMode := getCallerModeFromString(callerModeRaw.(string))
		if callerMode == UndefinedCallerMode {
			return logical.ErrorResponse("unrecognised caller_mode argument; valid values are 'standard' or 'sidecar'"), nil
		}
		config.CallerMode = callerMode
	} else if !ok && createOperation {
		config.CallerMode = StandardCallerMode
	}

	err = saveConfig(ctx, config, req.Storage)
	if err != nil {
		return nil, err
	}

	b.logger.Info(fmt.Sprintf(
		"%s config for org ID %s running in %s caller mode",
		operationToStringVerb(req.Operation),
		config.OrgId,
		config.CallerMode.String()))
	// reset the client so the next invocation will pick up the new configuration
	b.reset()
	return nil, nil
}

func readConfig(ctx context.Context, s logical.Storage, orgId string) (*astraConfig, error) {
	if orgId == "" {
		return nil, errors.New("org ID is an empty string")
	}

	entry, err := s.Get(ctx, configStoragePath+orgId)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	config := &astraConfig{}
	err = entry.DecodeJSON(config)
	if err != nil {
		return nil, errors.New("error retrieving config for org ID " + orgId + ": " + err.Error())
	}

	return config, nil
}

func saveConfig(ctx context.Context, config *astraConfig, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON(configStoragePath+config.OrgId, config)
	if err != nil {
		return err
	}
	if entry == nil {
		return errors.New("error creating config entry for org ID " + config.OrgId + ": " + err.Error())
	}
	if err = s.Put(ctx, entry); err != nil {
		return err
	}
	return nil
}

// pathConfigDelete removes the configuration for the backend
func (b *datastaxAstraBackend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgId, ok := data.GetOk("org_id")
	if !ok {
		return logical.ErrorResponse("please provide an org_id argument"), nil
	}
	err := req.Storage.Delete(ctx, configStoragePath+orgId.(string))
	if err != nil {
		return nil, err
	}
	b.logger.Info("Deleted config for org ID " + orgId.(string))
	b.reset()
	return nil, nil
}

func listConfig(ctx context.Context, s logical.Storage) ([]string, error) {
	objList, err := s.List(ctx, configStoragePath)
	if err != nil {
		return nil, errors.New("error loading config list")
	}
	if len(objList) == 0 {
		return nil, errors.New("no configs found")
	}
	return objList, nil
}

func (b *datastaxAstraBackend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	// The existence check determines whether the logical.Request.Operation value is Create or Update. In this case
	//	we will skip the argument validation as it will be performed in the write path implementation.
	obj, err := req.Storage.Get(ctx, configStoragePath + data.Get("org_id").(string))
	if err != nil {
		return false, errors.New("error retrieving config from storage for existence check: " + err.Error())
	}

	return obj != nil, nil
}

// pathConfigHelpSynopsis summarizes the help text for the configuration
const pathConfigHelpSynopsis = `Configure the Datastax Astra backend.`

// pathConfigHelpDescription describes the help text for the configuration
const pathConfigHelpDescription = `
The Datastax Astra secret backend requires credentials for managing
tokens issued to users working with an astra organization.

You must create a default token and specify the 
Astra endpoint before using this secrets backend.`
