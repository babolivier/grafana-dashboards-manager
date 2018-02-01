package common

import (
	"strings"

	"config"
	"grafana"
	"grafana/helpers"

	"github.com/sirupsen/logrus"
)

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
