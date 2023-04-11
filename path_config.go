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
	configStoragePath = "config/"
	configsListPath   = "configs/?"
)

// astraConfig includes the minimum configuration
// required to instantiate a new astra client.
type astraConfig struct {
	AstraToken            	string 			`json:"astra_token"`
	URL                   	string 			`json:"url"`
	OrgId                 	string 			`json:"org_id"`
	LogicalName           	string 			`json:"logical_name"`
	DefaultLeaseRenewTime	time.Duration	`json:"renewal_time"`
}

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
			"renewal_time": {
				Type:        framework.TypeString,
				Description: "Default lease time in seconds, minutes or hours for renew operation. Use the duration intials after the number. for e.g. 5s, 5m, 5h",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "renewal_time",
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

func pathConfigList(b *datastaxAstraBackend) *framework.Path {
	return &framework.Path{
		Pattern: configsListPath,
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathConfigList,
				Summary:  "List all configs.",
			},
		},
	}
}

func (b *datastaxAstraBackend) pathConfigList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	matches := map[string]interface{}{}
	objList, err := req.Storage.List(ctx, "config/")
	if err != nil {
		return nil, errors.New("failed to load config list")
	}
	if len(objList) == 0 {
		return nil, errors.New("no objects found")
	}
	for _, key := range objList {
		fmt.Println(key)
		cid := "config/" + key
		m, err := getConfig(ctx, req.Storage, key)
		if err != nil {
			return nil, errors.New("failed to get config for org in list")
		}
		matches[cid] = m.AstraToken
	}
	var keys []string
	for k := range matches {
		keys = append(keys, k)
	}
	return logical.ListResponseWithInfo(keys, matches), nil
}

// pathConfigRead reads the configuration and outputs information.
func (b *datastaxAstraBackend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgId := data.Get("org_id").(string)
	logicalName := data.Get("logical_name").(string)
	if logicalName != "" && orgId == "" {
		configs, err := listConfig(ctx, req.Storage)
		if err != nil {
			return nil, errors.New("error getting configs")
		}
		if len(configs) == 0 {
			return nil, errors.New("no configs found")
		}
		for _, orgId := range configs {
			m, err := getConfig(ctx, req.Storage, orgId)
			if err != nil {
				return nil, errors.New("failed to get config for org in list")
			}
			if m.LogicalName == logicalName {
				return &logical.Response{
					Data: map[string]interface{}{
						"url":          m.URL,
						"org_id":       m.OrgId,
						"logical_name": m.LogicalName,
						"renewal_time": m.DefaultLeaseRenewTime,
					},
				}, nil
			}
		}
		return nil, errors.New("no config found for logical_name = " + logicalName)
	}
	if orgId != "" {
		config, err := getConfig(ctx, req.Storage, orgId)
		if err != nil {
			return nil, err
		}
		if config == nil {
			return nil, errors.New("config does not exist for org")
		}
		return &logical.Response{
			Data: map[string]interface{}{
				"url":          config.URL,
				"org_id":       config.OrgId,
				"logical_name": config.LogicalName,
				"renewal_time": config.DefaultLeaseRenewTime,
			},
		}, nil
	}
	return nil, errors.New("please provide org_id or logical_name")
}

// pathConfigWrite updates the configuration for the backend
func (b *datastaxAstraBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgId, ok := data.GetOk("org_id")
	if !ok {
		return nil, errors.New("org_id not present")
	}
	config, err := getConfig(ctx, req.Storage, orgId.(string))
	if err != nil {
		return nil, err
	}

	createOperation := (req.Operation == logical.CreateOperation)

	if config == nil {
		if !createOperation {
			return nil, errors.New("unable to find config during update operation")
		}
		config = &astraConfig{}
		config.OrgId = orgId.(string)
	}

	token, ok := data.GetOk("astra_token")
	if ok {
		config.AstraToken = token.(string)
	} else if !ok && createOperation {
		return nil, errors.New("astra_token not present")
	}
	url, ok := data.GetOk("url")
	if ok {
		config.URL = url.(string)
	} else if !ok && createOperation {
		return nil, errors.New("url not present")
	}
	logicalName, ok := data.GetOk("logical_name")
	if ok {
		config.LogicalName = logicalName.(string)
	} else if !ok && createOperation {
		return nil, errors.New("logical_name not present")
	}
	renewalTime, ok := data.GetOk("renewal_time")
	if !ok && createOperation {
		renewalTime = data.Get("renewal_time")
	}
	if renewalTime == "" {
		renewalTime = "0"
	}
	parseRenewalTime, _ := time.ParseDuration(renewalTime.(string))
	config.DefaultLeaseRenewTime = time.Duration(parseRenewalTime.Seconds()) * time.Second

	err = saveConfig(ctx, config, req.Storage)
	if err != nil {
		return nil, err
	}
	// reset the client so the next invocation will pick up the new configuration
	b.reset()
	return nil, nil
}

func saveConfig(ctx context.Context, config *astraConfig, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON(configStoragePath+config.OrgId, config)
	if err != nil {
		return err
	}
	if err = s.Put(ctx, entry); err != nil {
		return err
	}
	return nil
}

// pathConfigDelete removes the configuration for the backend
func (b *datastaxAstraBackend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	orgId := data.Get("org_id").(string)
	if orgId == "" {
		return nil, errors.New("invalid org_id")
	}
	err := req.Storage.Delete(ctx, configStoragePath+orgId)
	if err != nil {
		return nil, err
	}
	b.reset()
	return nil, nil
}

func getConfig(ctx context.Context, s logical.Storage, orgId string) (*astraConfig, error) {
	entry, err := s.Get(ctx, configStoragePath+orgId)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	config := astraConfig{}
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, fmt.Errorf("error reading root configuration: %w", err)
	}

	// return the config, we are done
	return &config, nil
}

func listConfig(ctx context.Context, s logical.Storage) ([]string, error) {
	objList, err := s.List(ctx, configStoragePath)
	if err != nil {
		return nil, errors.New("failed to load config list")
	}
	if len(objList) == 0 {
		return nil, errors.New("no configs found")
	}
	return objList, nil
}

func (b *datastaxAstraBackend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	orgId := data.Get("org_id").(string)
	if orgId == "" {
		return false, errors.New("invalid org_id")
	}
	out, err := req.Storage.Get(ctx, configStoragePath+orgId)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}

	return out != nil, nil
}

// pathConfigHelpSynopsis summarizes the help text for the configuration
const pathConfigHelpSynopsis = `Configure the Datastax Astra backend.`

// pathConfigHelpDescription describes the help text for the configuration
const pathConfigHelpDescription = `
The Datastax Astra secret backend requires credentials for managing
tokens issued to users working with an astra organization.

You must create a default token and specify the 
Astra endpoint before using this secrets backend.`
