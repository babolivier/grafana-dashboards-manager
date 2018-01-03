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

type Dashboard struct {
	RawJSON []byte
	Slug    string
	Version int
}

func (d *Dashboard) UnmarshalJSON(b []byte) (err error) {
	var body struct {
		Dashboard interface{} `json:"dashboard"`
		Meta      struct {
			Slug    string `json:"slug"`
			Version int    `json:"version"`
		} `json:"meta"`
	}

	if err = json.Unmarshal(b, &body); err != nil {
		return
	}
	d.Slug = body.Meta.Slug
	d.Version = body.Meta.Version
	d.RawJSON, err = json.Marshal(body.Dashboard)

	return
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

func (c *Client) GetDashboard(URI string) (db *Dashboard, err error) {
	body, err := c.request("GET", "dashboards/"+URI, nil)
	if err != nil {
		return
	}

	db = new(Dashboard)
	err = json.Unmarshal(body, db)
	return
}
