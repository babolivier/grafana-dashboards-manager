package main

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"config"
	"git"
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

	// Files to push are stored in a map before being pushed to the Grafana API.
	// We don't push them in the loop iterating over commits because, in the
	// case a file is successively updated by two commits pushed at the same
	// time, it would push the same file several time, which isn't an optimised
	// behaviour.
	filesToPush := make(map[string]bool)

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

		// Push all added files, except the ones describing a dashboard which
		// name starts with a the prefix specified in the configuration file.
		for _, addedFile := range commit.Added {
			ignored, err := isIgnored(addedFile)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error":    err,
					"filename": addedFile,
				}).Error("Failed to check if file is to be ignored")

				continue
			}

			if !ignored {
				logrus.WithFields(logrus.Fields{
					"filename": addedFile,
				}).Info("Setting file as file to push to Grafana")

				filesToPush[addedFile] = true
			}
		}

		// Push all modified files, except the ones describing a dashboard which
		// name starts with a the prefix specified in the configuration file.
		for _, modifiedFile := range commit.Modified {
			ignored, err := isIgnored(modifiedFile)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error":    err,
					"filename": modifiedFile,
				}).Error("Failed to check if file is to be ignored")

				continue
			}

			if !ignored {
				logrus.WithFields(logrus.Fields{
					"filename": modifiedFile,
				}).Info("Setting file as file to push to Grafana")

				filesToPush[modifiedFile] = true
			}
		}

		// TODO: Remove a dashboard when its file gets deleted?
	}

	// Push all files to the Grafana API
	for fileToPush := range filesToPush {
		if err = pushFile(fileToPush); err != nil {
			logrus.WithFields(logrus.Fields{
				"error":    err,
				"filename": fileToPush,
			}).Error("Failed push the file to Grafana")

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

// pushFile pushes the content of a given file to the Grafana API in order to
// create or update a dashboard.
// Returns an error if there was an issue reading the file or sending its content
// to the Grafana instance.
func pushFile(filename string) error {
	filePath := cfg.Git.ClonePath + "/" + filename
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Remove the .json part
	slug := strings.Split(filename, ".json")[0]

	return grafanaClient.CreateOrUpdateDashboard(slug, fileContent)
}

// isIgnored checks whether the file must be ignored, by checking if there's an
// prefix for ignored files set in the configuration file, and if the dashboard
// described in the file has a name that starts with this prefix. Returns an
// error if there was an issue reading or decoding the file.
// TODO: Optimise this part of the workflow, as all files get open twice (here
// and in pushFile)
func isIgnored(filename string) (bool, error) {
	// Always ignore versions.json
	if strings.HasSuffix(filename, "versions.json") {
		return true, nil
	}

	// If there's no prefix set, no file is ignored
	if len(cfg.Grafana.IgnorePrefix) == 0 {
		return false, nil
	}

	// Read the file's content
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}

	// Parse the file's content to find the dashboard's name
	var dashboardName struct {
		Name string `json:"title"`
	}
	if err = json.Unmarshal(fileContent, &dashboardName); err != nil {
		return false, err
	}

	// Compare the lower case dashboar name to the prefix (which has already
	// been lower cased when loading the configuration file)
	lowerCaseName := strings.ToLower(dashboardName.Name)
	if strings.HasPrefix(lowerCaseName, cfg.Grafana.IgnorePrefix) {
		return true, nil
	}

	return false, nil
}
