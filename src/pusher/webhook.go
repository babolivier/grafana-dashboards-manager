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

func HandlePush(payload interface{}, header webhooks.Header) {
	var err error

	pl := payload.(gitlab.PushEventPayload)

	if _, err = git.Sync(cfg.Git); err != nil {
		panic(err)
	}

	for _, commit := range pl.Commits {
		// We don't want to process commits made by the puller
		if commit.Author.Email == cfg.Git.CommitsAuthor.Email {
			continue
		}

		for _, addedFile := range commit.Added {
			if err = pushFile(addedFile); err != nil {
				panic(err)
			}
		}

		for _, modifiedFile := range commit.Modified {
			if err = pushFile(modifiedFile); err != nil {
				panic(err)
			}
		}

		// TODO: Remove a dashboard when its file gets deleted?
	}

	// Grafana will auto-update the version number after we pushed the new
	// dashboards, so we use the puller mechanic to pull the updated numbers and
	// commit them in the git repo.
	if err = puller.PullGrafanaAndCommit(grafanaClient, cfg); err != nil {
		panic(err)
	}
}

func pushFile(filename string) error {
	filePath := cfg.Git.ClonePath + "/" + filename
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Remove the .json part
	slug := strings.Split(filename, ".json")[0]

	return grafanaClient.UpdateDashboard(slug, fileContent)
}
