package webhook

import (
	"io/ioutil"
	"path/filepath"

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

	// Load the Git repository.
	var needsSync bool
	repo, needsSync, err = git.NewRepository(cfg.Git)
	if err != nil {
		return err
	}

	// Synchronise the repository if needed.
	if needsSync {
		if err = repo.Sync(false); err != nil {
			return err
		}
	}

	// Initialise the webhook
	hook := gitlab.New(&gitlab.Config{
		Secret: cfg.Pusher.Config.Secret,
	})
	// Register the handler
	hook.RegisterEvents(HandlePush, gitlab.PushEvents)

	// Expose the webhook
	return webhooks.Run(
		hook,
		cfg.Pusher.Config.Interface+":"+cfg.Pusher.Config.Port,
		cfg.Pusher.Config.Path,
	)
}

// HandlePush is called each time a push event is sent by GitLab on the webhook.
func HandlePush(payload interface{}, header webhooks.Header) {
	var err error

	var (
		added    = make([]string, 0)
		modified = make([]string, 0)
		removed  = make([]string, 0)
		contents = make(map[string][]byte)
	)

	// Process the payload using the right structure
	pl := payload.(gitlab.PushEventPayload)

	// Only push changes made on master to Grafana
	if pl.Ref != "refs/heads/master" {
		return
	}

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

		// Copy added files' names
		for _, addedFile := range commit.Added {
			added = append(added, addedFile)
		}

		// Copy modified files' names
		for _, modifiedFile := range commit.Modified {
			modified = append(modified, modifiedFile)
		}

		// Copy removed files' names
		for _, removedFile := range commit.Removed {
			removed = append(removed, removedFile)
		}
	}

	// Get the content of the removed files before pulling from the remote, because
	// we won't be able to access them afterwards
	if err = getFilesContents(removed, &contents, cfg); err != nil {
		return
	}

	// Synchronise the repository (i.e. pull from remote)
	if err = repo.Sync(false); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":      err,
			"repo":       cfg.Git.User + "@" + cfg.Git.URL,
			"clone_path": cfg.Git.ClonePath,
		}).Error("Failed to synchronise the Git repository with the remote")

		return
	}

	// Get the content of the added files
	if err = getFilesContents(added, &contents, cfg); err != nil {
		return
	}

	// Get the content of the modified files
	if err = getFilesContents(modified, &contents, cfg); err != nil {
		return
	}

	// Remove the ignored files from the map
	if err = common.FilterIgnored(&contents, cfg); err != nil {
		return
	}

	// Push all added and modified dashboards to Grafana
	common.PushFiles(added, contents, grafanaClient)
	common.PushFiles(modified, contents, grafanaClient)

	// If the user requested it, delete all dashboards that were removed
	// from the repository.
	if deleteRemoved {
		common.DeleteDashboards(removed, contents, grafanaClient)
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

// getFilesContents takes a slice of files' names and a map mapping a file's name
// to its content and appends to it the current content of all of the files for
// which the name appears in the slice.
// Returns an error if there was an issue reading a file.
func getFilesContents(
	filenames []string, contents *map[string][]byte, cfg *config.Config,
) (err error) {
	// Iterate over files' names
	for _, filename := range filenames {
		// Compute the file's path
		filePath := filepath.Join(cfg.Git.ClonePath, filename)
		// Read the file's content
		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}

		// Append the content to the map
		(*contents)[filename] = fileContent
	}

	return
}
