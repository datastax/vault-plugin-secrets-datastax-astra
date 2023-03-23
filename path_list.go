package datastax_astra

import (
    "context"
    "errors"
    "github.com/hashicorp/vault/sdk/logical"
    "strings"
)

type PathList int
type ToResponseDataFunc func() map[string]interface{}

const (
    ConfigPathList = iota
    RolePathList
    CredentialsPathList
)

func (pl PathList) String() string {
    switch pl {
    case ConfigPathList:
        return "config"
    case RolePathList:
        return "role"
    case CredentialsPathList:
        return "token"
    default:
        return "unknown"
    }
}

// GetObjectInformation Retrieves the object entry's ID and data. Specifically, we return the object ID, a callback
//  function to retrieve the response data, and an error to determine if we should call the callback function.
func (pl PathList) GetObjectInformation(ctx context.Context, req *logical.Request, key string) (string, ToResponseDataFunc, error) {
    switch pl {
    case ConfigPathList:
        obj, err := readConfig(ctx, req.Storage, key)
        return key, obj.ToResponseData, err
    case RolePathList:
        obj, err := readRoleUsingKey(ctx, req.Storage, key)
        // The role storage key is contains the Org ID and Role Name delimited by ":"
        //  For consistency and to avoid confusion, we return just the Role Name
        return strings.Split(key, roleStorageKeyDelimiter)[1], obj.ToResponseData, err
    case CredentialsPathList:
        obj, err := readToken(ctx, req.Storage, key)
        // Regardless of the storage key used for the token, always return the Client ID
        return obj.ClientID, obj.ToResponseData, err
    default:
        return "", nil, errors.New("response requested for unknown object type")
    }
}

func (b *datastaxAstraBackend) pathObjectList(ctx context.Context, req *logical.Request, pl PathList) (*logical.Response, error) {
    pathList := pl.String()
    objList, err := req.Storage.List(ctx, pathList+"/")
    if err != nil {
        return nil, errors.New("error loading " + pathList + " list: " + err.Error())
    }
    if len(objList) == 0 {
        return nil, errors.New("no " + pathList + "s found")
    }
    keyInfo := map[string]interface{}{}
    var keys []string
    for _, key := range objList {
        // Get the key/value (ID/Data) of the object
        objId, objResponseData, err := pl.GetObjectInformation(ctx, req, key)
        if err != nil {
            return nil, errors.New("failed to retrieve" + pathList + "information: " + err.Error())
        }
        keys = append(keys, objId)
        keyInfo[objId] = objResponseData()
    }
    return logical.ListResponseWithInfo(keys, keyInfo), nil
}