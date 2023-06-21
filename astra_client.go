package datastax_astra

import (
	"errors"
	"io"
	"net/http"
	"strings"

	dsAstraClient "github.com/datastax/astra-client-go/v2/astra"
)

const (
	secretsPath   = "/v2/clientIdSecrets"
	pluginversion = "Vault-Plugin v2.0.0"
)

// astraClient creates an object storing
// the client.
type astraClient struct {
	*dsAstraClient.Client
	url   string
	token string
}

func makeHttpRequest(method, url, payload, astraToken string) (*http.Response, error) {
	client := &http.Client{}
	var body io.Reader
	if payload != "" {
		body = strings.NewReader(payload)
	} else {
		body = nil
	}
	httpReq, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, errors.New("error creating httpReq " + err.Error())
	}

	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Authorization", "Bearer "+astraToken)
	httpReq.Header.Add("User-Agent", pluginversion)
	return client.Do(httpReq)
}

func (ac *astraClient) createToken(payload string) ([]byte, error) {
	url := ac.url + secretsPath
	res, err := makeHttpRequest(http.MethodPost, url, payload, ac.token)
	if err != nil {
		return nil, errors.New("error sending request " + err.Error())
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.New("error reading ioutil " + err.Error())
	}

	return body, nil
}

func (ac *astraClient) deleteToken(clientId string) error {
	url := ac.url + secretsPath + "/" + clientId
	res, err := makeHttpRequest(http.MethodDelete, url, "", ac.token)
	if err != nil {
		return errors.New("error sending request " + err.Error())
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return errors.New("unable to delete token in astra; " + res.Status)
	}

	return nil
}

// newClient creates a new client to access Astra
// and exposes it for any secrets or roles to use.
func newClient(config *astraConfig) (*astraClient, error) {
	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	if config.AstraToken == "" {
		return nil, errors.New("astra token was not defined")
	}

	if config.URL == "" {
		return nil, errors.New("client URL was not defined")
	}

	newAstraClient := astraClient{}
	newAstraClient.url = config.URL
	newAstraClient.token = config.AstraToken

	return &newAstraClient, nil
}
