package grafana

import (
	"encoding/json"
)

type dbSearchResponse struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	URI     string   `json:"uri"`
	Type    string   `json:"type"`
	Tags    []string `json:"tags"`
	Starred bool     `json:"isStarred"`
}

func (c *Client) GetDashboardsURIs() (URIs []string, err error) {
	resp, err := c.request("GET", "search", nil)

	var respBody []dbSearchResponse
	if err = json.Unmarshal(resp, &respBody); err != nil {
		return
	}

	URIs = make([]string, 0)
	for _, db := range respBody {
		URIs = append(URIs, db.URI)
	}

	return
}

func (c *Client) GetDashboardJSON(URI string) ([]byte, error) {
	return c.request("GET", URI, nil)
}
