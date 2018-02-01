package common

import (
	"io/ioutil"
	"strings"

	"config"
	"grafana"
	"grafana/helpers"

	"github.com/sirupsen/logrus"
)

// PrepareForPush reads the file containing the JSON representation of a
// dashboard, checks if the dashboard is set to be ignored, and if not appends
// its content to a map, which will be later iterated over to push the contents
// it contains to the Grafana API.
// Returns an error if there was an issue reading the file or checking if the
// dashboard it represents is to be ignored.
func PrepareForPush(
	filename string, filesToPush *map[string][]byte, cfg *config.Config,
) (err error) {
	// Don't set versions.json to be pushed
	if strings.HasSuffix(filename, "versions.json") {
		return
	}

	// Read the file's content
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	// Check if dashboard is ignored
	ignored, err := isIgnored(fileContent, cfg)
	if err != nil {
		return
	}

	// Append to the list of contents to push to Grafana
	if !ignored {
		logrus.WithFields(logrus.Fields{
			"filename": filename,
		}).Info("Preparing file to be pushed to Grafana")

		(*filesToPush)[filename] = fileContent
	}

	return
}

func PushFiles(filesToPush map[string][]byte, client *grafana.Client) {
	// Push all files to the Grafana API
	for fileToPush, fileContent := range filesToPush {
		if err := client.CreateOrUpdateDashboard(fileContent); err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": fileToPush,
			}).Error("Failed to push the file to Grafana")
		}
	}
}

// DeleteDashboard reads the dashboard described in a given file and, if the file
// isn't set to be ignored, delete the corresponding dashboard from Grafana.
// Returns an error if there was an issue reading the file's content, checking
// if the dashboard is to be ignored, computing its slug or deleting it from
// Grafana.
func DeleteDashboard(
	filename string, client *grafana.Client, cfg *config.Config,
) (err error) {
	// Don't delete versions.json
	if strings.HasSuffix(filename, "versions.json") {
		return
	}

	// Read the file's content
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	// Check if dashboard is ignored
	ignored, err := isIgnored(fileContent, cfg)
	if err != nil {
		return
	}

	if !ignored {
		// Retrieve dashboard slug because we need it in the deletion request.
		var slug string
		slug, err = helpers.GetDashboardSlug(fileContent)
		if err != nil {
			return
		}

		// Delete the dashboard
		err = client.DeleteDashboard(slug)
	}

	return
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
