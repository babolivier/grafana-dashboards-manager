package grafana

import (
	"encoding/json"
	"fmt"
)

// dbSearchResponse represents an element of the response to a dashboard search
// query
type dbSearchResponse struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	URI     string   `json:"uri"`
	Type    string   `json:"type"`
	Tags    []string `json:"tags"`
	Starred bool     `json:"isStarred"`
}

// dbCreateOrUpdateRequest represents the request sent to create or update a
// dashboard
type dbCreateOrUpdateRequest struct {
	Dashboard rawJSON `json:"dashboard"`
	Overwrite bool    `json:"overwrite"`
}

// dbCreateOrUpdateResponse represents the response sent by the Grafana API to
// a dashboard creation or update. All fields described from the Grafana
// documentation aren't located in this structure because there are some we
// don't need.
type dbCreateOrUpdateResponse struct {
	Status  string `json:"success"`
	Version int    `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// Dashboard represents a Grafana dashboard, with its JSON definition, slug and
// current version.
type Dashboard struct {
	RawJSON []byte
	Slug    string
	Version int
}

// UnmarshalJSON tells the JSON parser how to unmarshal JSON data into an
// instance of the Dashboard structure.
// Returns an error if there was an issue unmarshalling the JSON.
func (d *Dashboard) UnmarshalJSON(b []byte) (err error) {
	// Define the structure of what we want to parse
	var body struct {
		Dashboard rawJSON `json:"dashboard"`
		Meta      struct {
			Slug    string `json:"slug"`
			Version int    `json:"version"`
		} `json:"meta"`
	}

	// Unmarshal the JSON into the newly defined structure
	if err = json.Unmarshal(b, &body); err != nil {
		return
	}
	// Define all fields with their corresponding value.
	d.Slug = body.Meta.Slug
	d.Version = body.Meta.Version
	d.RawJSON = body.Dashboard

	return
}

// GetDashboardsURIs requests the Grafana API for the list of all dashboards,
// then returns the dashboards' URIs. An URI will look like "db/[dashboard slug]".
// Returns an error if there was an issue requesting the URIs or parsing the
// response body.
func (c *Client) GetDashboardsURIs() (URIs []string, err error) {
	resp, err := c.request("GET", "search", nil)
	if err != nil {
		return
	}

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

// GetDashboard requests the Grafana API for a dashboard identified by a given
// URI (using the same format as GetDashboardsURIs).
// Returns the dashboard as an instance of the Dashboard structure.
// Returns an error if there was an issue requesting the dashboard or parsing
// the response body.
func (c *Client) GetDashboard(URI string) (db *Dashboard, err error) {
	body, err := c.request("GET", "dashboards/"+URI, nil)
	if err != nil {
		return
	}

	db = new(Dashboard)
	err = json.Unmarshal(body, db)
	return
}

// CreateOrUpdateDashboard takes a given JSON content (as []byte) and create the
// dashboard if it doesn't exist on the Grafana instance, else updates the
// existing one. The Grafana API decides whether to create or update based on the
// "id" attribute in the dashboard's JSON: If it's unkown or null, it's a
// creation, else it's an update.
// The slug is only used for error reporting, and can differ from the
// dashboard's actual slug in its JSON content. However, it's recommended to use
// the same in both places.
// Returns an error if there was an issue generating the request body, performing
// the request or decoding the response's body.
func (c *Client) CreateOrUpdateDashboard(slug string, contentJSON []byte) (err error) {
	reqBody := dbCreateOrUpdateRequest{
		Dashboard: rawJSON(contentJSON),
		Overwrite: true,
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

	var respBody dbCreateOrUpdateResponse
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
