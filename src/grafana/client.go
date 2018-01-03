package grafana

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type Client struct {
	BaseURL    string
	APIKey     string
	httpClient *http.Client
}

func NewClient(baseURL string, apiKey string) (c *Client) {
	if strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL[:len(baseURL)-1]
	}

	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		httpClient: new(http.Client),
	}
}

func (c *Client) request(method string, endpoint string, body []byte) ([]byte, error) {
	url := c.BaseURL + "/api/" + endpoint

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	authHeader := fmt.Sprintf("Bearer %s", c.APIKey)
	req.Header.Add("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			err = fmt.Errorf("%s not found (404)", url)
		} else {
			err = newHttpUnknownError(resp.StatusCode)
		}
	}

	return respBody, err
}

type httpUnkownError struct {
	StatusCode int
}

func newHttpUnknownError(statusCode int) *httpUnkownError {
	return &httpUnkownError{
		StatusCode: statusCode,
	}
}

func (e *httpUnkownError) Error() string {
	return fmt.Sprintf("Unknown HTTP error: %d", e.StatusCode)
}
