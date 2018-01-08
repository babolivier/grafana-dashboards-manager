package main

import (
	"io/ioutil"
	"strings"

	"config"
	"git"
	puller "puller"

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

	// Clone or pull the repository
	if _, err = git.Sync(cfg.Git); err != nil {
		panic(err)
	}

	// Only push changes made on master to Grafana
	if pl.Ref != "refs/heads/master" {
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
			continue
		}

		// Push all added files
		for _, addedFile := range commit.Added {
			filesToPush[addedFile] = true
		}

		// Push all modified files
		for _, modifiedFile := range commit.Modified {
			filesToPush[modifiedFile] = true
		}

		// TODO: Remove a dashboard when its file gets deleted?
	}

	// Push all files to the Grafana API
	for fileToPush := range filesToPush {
		if err = pushFile(fileToPush); err != nil {
			panic(err)
		}
	}

	// Grafana will auto-update the version number after we pushed the new
	// dashboards, so we use the puller mechanic to pull the updated numbers and
	// commit them in the git repo.
	if err = puller.PullGrafanaAndCommit(grafanaClient, cfg); err != nil {
		panic(err)
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
