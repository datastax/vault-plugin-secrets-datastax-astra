package datastax_astra

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

const (
	astra_token  = "AstraCS:Th!s1safAk3T0K3n"
	url          = "http://localhost:" + mockLocalServerPort
	org_id       = "testOrgId"
	logical_name = "testLogicalName"
	caller_mode  = "sidecar"
)

// TestConfig mocks the creation, read, update, and delete
// of the backend configuration for astra.
func TestConfig(t *testing.T) {
	b, reqStorage := getTestBackend(t)

	t.Run("Test Configuration", func(t *testing.T) {
		err := testConfigCreate(t, b, reqStorage, map[string]interface{}{
			"astra_token":  astra_token,
			"url":          url,
			"org_id":       org_id,
			"logical_name": logical_name,
			"caller_mode":  caller_mode,
		})

		assert.NoError(t, err)

		err = testConfigRead(t, b, reqStorage, map[string]interface{}{
			"org_id":       org_id,
			"logical_name": logical_name,
		})

		assert.NoError(t, err)

		err = testConfigRead(t, b, reqStorage, map[string]interface{}{
			"org_id":       org_id,
			"logical_name": logical_name,
		})

		assert.NoError(t, err)

		err = testConfigDelete(t, b, reqStorage, map[string]interface{}{
			"org_id": org_id,
		})

		assert.NoError(t, err)
	})
}

func testConfigDelete(t *testing.T, b logical.Backend, s logical.Storage, d map[string]interface{}) error {
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "config",
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return err
	}

	if resp != nil && resp.IsError() {
		return resp.Error()
	}
	return nil
}

func testConfigCreate(t *testing.T, b logical.Backend, s logical.Storage, d map[string]interface{}) error {
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config",
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return err
	}

	if resp != nil && resp.IsError() {
		return resp.Error()
	}
	return nil
}

func testConfigRead(t *testing.T, b logical.Backend, s logical.Storage, d map[string]interface{}) error {
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config",
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return err
	}

	if resp != nil && resp.IsError() {
		return resp.Error()
	}

	require.NotNil(t, resp.Data["url"])
	require.NotNil(t, resp.Data["org_id"])
	require.NotNil(t, resp.Data["logical_name"])
	require.NotNil(t, resp.Data["caller_mode"])
	expectedResp := map[string]interface{}{
		"url":    			url,
		"org_id": 			org_id,
		"logical_name": 	logical_name,
		"caller_mode": 		caller_mode,
	}
	require.Equal(t, expectedResp, resp.Data)

	return nil
}
