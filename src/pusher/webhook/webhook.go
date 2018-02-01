package webhook

import (
	"config"
	"git"
	"grafana"
	puller "puller"
	"pusher/common"

	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
)

// Some variables need to be global to the package since we need them in the
// webhook handlers.
var (
	grafanaClient *grafana.Client
	cfg           *config.Config
	deleteRemoved bool
	repo          *git.Repository
)

// Setup creates and exposes a GitLab webhook using a given configuration.
// Returns an error if the webhook couldn't be set up.
func Setup(conf *config.Config, client *grafana.Client, delRemoved bool) (err error) {
	cfg = conf
	grafanaClient = client
	deleteRemoved = delRemoved

	repo, _, err = git.NewRepository(cfg.Git)
	if err != nil {
		return err
	}

	hook := gitlab.New(&gitlab.Config{
		Secret: cfg.Pusher.Config.Secret,
	})
	hook.RegisterEvents(HandlePush, gitlab.PushEvents)

	return webhooks.Run(
		hook,
		cfg.Pusher.Config.Interface+":"+cfg.Pusher.Config.Port,
		cfg.Pusher.Config.Path,
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
	if err = repo.Sync(false); err != nil {
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
			if err = common.PrepareForPush(
				addedFile, &filesToPush, cfg,
			); err != nil {
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
			if err = common.PrepareForPush(
				modifiedFile, &filesToPush, cfg,
			); err != nil {
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
		if deleteRemoved {
			for _, removedFile := range commit.Removed {
				if err = common.DeleteDashboard(
					removedFile, grafanaClient, cfg,
				); err != nil {
					logrus.WithFields(logrus.Fields{
						"error":    err,
						"filename": removedFile,
					}).Error("Failed to delete the dashboard")
				}
			}
		}
	}

	common.PushFiles(filesToPush, grafanaClient)

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
