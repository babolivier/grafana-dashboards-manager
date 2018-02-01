package poller

import (
	"time"

	"config"
	"git"
	"grafana"
	"pusher/common"

	"github.com/sirupsen/logrus"
)

func Setup(cfg *config.Config, client *grafana.Client, delRemoved bool) error {
	r, needsSync, err := git.NewRepository(cfg.Git)
	if err != nil {
		return err
	}

	if needsSync {
		if err = r.Sync(false); err != nil {
			return err
		}
	}

	errs := make(chan error, 1)

	go func() {
		if err = poller(cfg, r, client, delRemoved); err != nil {
			errs <- err
			return
		}
	}()

	err = <-errs
	return err
}

func poller(
	cfg *config.Config, repo *git.Repository, client *grafana.Client,
	delRemoved bool,
) (err error) {
	latestCommit, err := repo.GetLatestCommit()
	if err != nil {
		return
	}

	filesContents, err := repo.GetFilesContentsAtCommit(latestCommit)
	if err != nil {
		return
	}

	previousCommit := latestCommit
	previousFilesContents := filesContents

	for {
		if err = repo.Sync(true); err != nil {
			return
		}

		latestCommit, err = repo.GetLatestCommit()
		if err != nil {
			return
		}

		if previousCommit.Hash.String() != latestCommit.Hash.String() {
			logrus.WithFields(logrus.Fields{
				"previous_hash": previousCommit.Hash.String(),
				"new_hash":      latestCommit.Hash.String(),
			}).Info("New commit(s) detected")

			filesContents, err = repo.GetFilesContentsAtCommit(latestCommit)
			if err != nil {
				return err
			}

			modified, removed, err := repo.GetModifiedAndRemovedFiles(previousCommit, latestCommit)
			if err != nil {
				return err
			}

			mergedContents := mergeContents(modified, removed, filesContents, previousFilesContents)
			common.PushFiles(modified, mergedContents, client)

			if delRemoved {
				common.DeleteDashboards(removed, mergedContents, client)
			}
		}

		previousCommit = latestCommit
		previousFilesContents = filesContents
		time.Sleep(time.Duration(cfg.Pusher.Config.Interval) * time.Second)
	}
}

func mergeContents(
	modified []string, removed []string,
	filesContents map[string][]byte, previousFilesContents map[string][]byte,
) (merged map[string][]byte) {
	merged = make(map[string][]byte)

	for _, modifiedFile := range modified {
		merged[modifiedFile] = filesContents[modifiedFile]
	}

	for _, removedFile := range removed {
		merged[removedFile] = previousFilesContents[removedFile]
	}

	return
}
