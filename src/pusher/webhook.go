package main

import (
	"io/ioutil"
	"strconv"
	"strings"

	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
)

func SetupWebhook() error {
	hook := gitlab.New(&gitlab.Config{
		Secret: "mysecret",
	})
	hook.RegisterEvents(HandlePush, gitlab.PushEvents)

	return webhooks.Run(
		hook,
		*webhookInterface+":"+strconv.Itoa(*webhookPort),
		*webhookPath,
	)
}

func HandlePush(payload interface{}, header webhooks.Header) {
	pl := payload.(gitlab.PushEventPayload)

	var err error
	for _, commit := range pl.Commits {
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

}

func pushFile(filename string) error {
	filePath := *clonePath + "/" + filename
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Remove the .json part
	slug := strings.Split(filename, ".json")[0]

	return grafanaClient.UpdateDashboard(slug, fileContent)
}
