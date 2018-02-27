package common

import (
	"strings"

	"config"
	"grafana"
	"grafana/helpers"

	"github.com/sirupsen/logrus"
)

// FilterIgnored takes a map mapping files' names to their contents and remove
// all the files that are supposed to be ignored by the dashboard manager.
// An ignored file is either named "versions.json" or describing a dashboard
// which slug starts with a given prefix.
// Returns an error if the slug couldn't be tested against the prefix.
func FilterIgnored(
	filesToPush *map[string][]byte, cfg *config.Config,
) (err error) {
	for filename, content := range *filesToPush {
		// Don't set versions.json to be pushed
		if strings.HasSuffix(filename, "versions.json") {
			delete(*filesToPush, filename)
			continue
		}

		// Check if dashboard is ignored
		ignored, err := isIgnored(content, cfg)
		if err != nil {
			return err
		}

		if ignored {
			delete(*filesToPush, filename)
		}
	}

	return
}

// PushFiles takes a slice of files' names and a map mapping a file's name to its
// content, and iterates over the first slice. For each file name, it will push
// to Grafana the content from the map that matches the name, as a creation or
// an update of an existing dashboard.
// Logs any errors encountered during an iteration, but doesn't return until all
// creation and/or update requests have been performed.
func PushFiles(filenames []string, contents map[string][]byte, client *grafana.Client) {
	// Push all files to the Grafana API
	for _, filename := range filenames {
		if err := client.CreateOrUpdateDashboard(contents[filename]); err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": filename,
			}).Error("Failed to push the file to Grafana")
		}
	}
}

// DeleteDashboards takes a slice of files' names and a map mapping a file's name
// to its content, and iterates over the first slice. For each file name, extract
// a dashboard's slug from the content, in the map, that matches the name, and
// will use it to send a deletion request to the Grafana API.
// Logs any errors encountered during an iteration, but doesn't return until all
// deletion requests have been performed.
func DeleteDashboards(filenames []string, contents map[string][]byte, client *grafana.Client) {
	for _, filename := range filenames {
		// Retrieve dashboard slug because we need it in the deletion request.
		slug, err := helpers.GetDashboardSlug(contents[filename])
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": filename,
			}).Error("Failed to compute the dahsboard's slug")
		}

		if err := client.DeleteDashboard(slug); err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": filename,
				"slug":     slug,
			}).Error("Failed to remove the dashboard from Grafana")
		}
	}
}

// isIgnored checks whether the file must be ignored, by checking if there's an
// prefix for ignored files set in the configuration file, and if the dashboard
// described in the file has a name that starts with this prefix. Returns an
// error if there was an issue reading or decoding the file.
func isIgnored(dashboardJSON []byte, cfg *config.Config) (bool, error) {
	// If there's no prefix set, no file is ignored
	if len(cfg.Grafana.IgnorePrefix) == 0 {
		return false, nil
	}

	// Parse the file's content to extract its slug
	slug, err := helpers.GetDashboardSlug(dashboardJSON)
	if err != nil {
		return false, err
	}

	// Compare the slug against the prefix
	if strings.HasPrefix(slug, cfg.Grafana.IgnorePrefix) {
		return true, nil
	}

	return false, nil
}
