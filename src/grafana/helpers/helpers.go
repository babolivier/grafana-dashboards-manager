package helpers

import (
	"encoding/json"

	"github.com/gosimple/slug"
)

// GetDashboardSlug reads the JSON description of a dashboard and computes a
// slug from the dashboard's title.
// Returns an error if there was an issue parsing the dashboard JSON description.
func GetDashboardSlug(dbJSONDescription []byte) (dbSlug string, err error) {
	// Parse the file's content to find the dashboard's title
	var dashboardTitle struct {
		Title string `json:"title"`
	}

	err = json.Unmarshal(dbJSONDescription, &dashboardTitle)
	// Compute the slug
	dbSlug = slug.Make(dashboardTitle.Title)
	return
}
