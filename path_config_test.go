package datastax_astra

import (
	"context"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
)

const (
	astra_token  = "AstraCS:KYZjAKoNQaQNEmrgJznTNiqZ:99df3d5dc4c3c25eac761ddad9630fc885e3b243bedb8f3b9f8c9b3ad6846b0e"
	url          = "https://api.astra.datastax.com"
	org_id       = "03acd0a6-1451-4827-b206-81ad1099f1a1"
	logical_name = "org1"
	renewal_time = "5h"
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
			"renewal_time": renewal_time,
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

	return nil
}
