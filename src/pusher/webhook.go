package main

import (
	"io/ioutil"
	"strings"

	"config"
	"git"
	"grafana/helpers"
	puller "puller"

	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
)

// SetupWebhook creates and exposes a GitLab webhook using a given configuration.
// Returns an error if the webhook couldn't be set up.
func SetupWebhook(cfg *config.Config) error {
	hook := gitlab.New(&gitlab.Config{
		Secret: cfg.Webhook.Secret,
	})
	hook.RegisterEvents(HandlePush, gitlab.PushEvents)

	return webhooks.Run(
		hook,
		cfg.Webhook.Interface+":"+cfg.Webhook.Port,
		cfg.Webhook.Path,
	)
}

// HandlePush is called each time a push event is sent by GitLab on the webhook.
func HandlePush(payload interface{}, header webhooks.Header) {
	var err error

	// Process the payload using the right structure
	pl := payload.(gitlab.PushEventPayload)

	// Only push changes made on master to Grafana
	if pl.Ref != "refs/heads/master" {
		return
	}

	// Clone or pull the repository
	if _, err = git.Sync(cfg.Git); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":      err,
			"repo":       cfg.Git.User + "@" + cfg.Git.URL,
			"clone_path": cfg.Git.ClonePath,
		}).Error("Failed to synchronise the Git repository with the remote")

		return
	}

	// Files to push and their contents are stored in a map before being pushed
	// to the Grafana API. We don't push them in the loop iterating over commits
	// because, in the case a file is successively updated by two commits pushed
	// at the same time, it would push the same file several time, which isn't
	// an optimised behaviour.
	filesToPush := make(map[string][]byte)

	// Iterate over the commits descriptions from the payload
	for _, commit := range pl.Commits {
		// We don't want to process commits made by the puller
		if commit.Author.Email == cfg.Git.CommitsAuthor.Email {
			logrus.WithFields(logrus.Fields{
				"hash":          commit.ID,
				"author_email":  commit.Author.Email,
				"manager_email": cfg.Git.CommitsAuthor.Email,
			}).Info("Commit was made by the manager, skipping")

			continue
		}

		// Set all added files to be pushed, except the ones describing a
		// dashboard which name starts with a the prefix specified in the
		// configuration file.
		for _, addedFile := range commit.Added {
			if err = prepareForPush(addedFile, &filesToPush); err != nil {
				logrus.WithFields(logrus.Fields{
					"error":    err,
					"filename": addedFile,
				}).Error("Failed to prepare file for push")

				continue
			}
		}

		// Set all modified files to be pushed, except the ones describing a
		// dashboard which name starts with a the prefix specified in the
		// configuration file.
		for _, modifiedFile := range commit.Modified {
			if err = prepareForPush(modifiedFile, &filesToPush); err != nil {
				logrus.WithFields(logrus.Fields{
					"error":    err,
					"filename": modifiedFile,
				}).Error("Failed to prepare file for push")

				continue
			}
		}

		// If a file describing a dashboard gets removed from the Git repository,
		// delete the corresponding dashboard on Grafana, but only if the user
		// mentionned they want to do so with the correct command line flag.
		if *deleteRemoved {
			for _, removedFile := range commit.Removed {
				if err = deleteDashboard(removedFile); err != nil {
					logrus.WithFields(logrus.Fields{
						"error":    err,
						"filename": removedFile,
					}).Error("Failed to delete the dashboard")
				}

				continue
			}
		}
	}

	// Push all files to the Grafana API
	for fileToPush, fileContent := range filesToPush {
		if err = grafanaClient.CreateOrUpdateDashboard(fileContent); err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": fileToPush,
			}).Error("Failed to push the file to Grafana")

			continue
		}
	}

	// Grafana will auto-update the version number after we pushed the new
	// dashboards, so we use the puller mechanic to pull the updated numbers and
	// commit them in the git repo.
	if err = puller.PullGrafanaAndCommit(grafanaClient, cfg); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":      err,
			"repo":       cfg.Git.User + "@" + cfg.Git.URL,
			"clone_path": cfg.Git.ClonePath,
		}).Error("Call to puller returned an error")
	}
}

// prepareForPush reads the file containing the JSON representation of a
// dashboard, checks if the dashboard is set to be ignored, and if not appends
// its content to a map, which will be later iterated over to push the contents
// it contains to the Grafana API.
// Returns an error if there was an issue reading the file or checking if the
// dashboard it represents is to be ignored.
func prepareForPush(
	filename string, filesToPush *map[string][]byte,
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
	ignored, err := isIgnored(fileContent)
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

// deleteDashboard reads the dashboard described in a given file and, if the file
// isn't set to be ignored, delete the corresponding dashboard from Grafana.
// Returns an error if there was an issue reading the file's content, checking
// if the dashboard is to be ignored, computing its slug or deleting it from
// Grafana.
func deleteDashboard(filename string) (err error) {
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
	ignored, err := isIgnored(fileContent)
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
		err = grafanaClient.DeleteDashboard(slug)
	}

	return
}

// isIgnored checks whether the file must be ignored, by checking if there's an
// prefix for ignored files set in the configuration file, and if the dashboard
// described in the file has a name that starts with this prefix. Returns an
// error if there was an issue reading or decoding the file.
func isIgnored(dashboardJSON []byte) (bool, error) {
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
