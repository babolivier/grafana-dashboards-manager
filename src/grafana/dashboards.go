package grafana

import (
	"encoding/json"
	"fmt"
)

type dbSearchResponse struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	URI     string   `json:"uri"`
	Type    string   `json:"type"`
	Tags    []string `json:"tags"`
	Starred bool     `json:"isStarred"`
}

type dbUpdateRequest struct {
	Dashboard rawJSON `json:"dashboard"`
	Overwrite bool    `json:"overwrite"`
}

type dbUpdateResponse struct {
	Status  string `json:"success"`
	Version int    `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

type Dashboard struct {
	RawJSON []byte
	Slug    string
	Version int
}

func (d *Dashboard) UnmarshalJSON(b []byte) (err error) {
	var body struct {
		Dashboard rawJSON `json:"dashboard"`
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
	d.RawJSON = body.Dashboard

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

func (c *Client) UpdateDashboard(slug string, contentJSON []byte) (err error) {
	reqBody := dbUpdateRequest{
		Dashboard: rawJSON(contentJSON),
		Overwrite: false,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return
	}

	var httpError *httpUnkownError
	var isHttpUnknownError bool
	respBodyJSON, err := c.request("POST", "dashboards/db", reqBodyJSON)
	if err != nil {
		httpError, isHttpUnknownError = err.(*httpUnkownError)
		// We process httpUnkownError errors below, after we decoded the body
		if !isHttpUnknownError {
			return
		}
	}

	var respBody dbUpdateResponse
	if err = json.Unmarshal(respBodyJSON, &respBody); err != nil {
		return
	}

	if respBody.Status != "success" && isHttpUnknownError {
		return fmt.Errorf(
			"Failed to update dashboard %s (%d %s): %s",
			slug, httpError.StatusCode, respBody.Status, respBody.Message,
		)
	}

	return
}
