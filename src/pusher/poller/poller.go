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

	previousCommit := latestCommit

	for {
		addedOrModified := make([]string, 0)
		removed := make([]string, 0)

		if err = repo.Sync(true); err != nil {
			return
		}

		latestCommit, err = repo.GetLatestCommit()
		if err != nil {
			return
		}

		if previousCommit.Hash.String() != latestCommit.Hash.String() {
			lineCounts, err := git.GetFilesLineCountsAtCommit(previousCommit)
			if err != nil {
				return err
			}

			deltas, err := repo.LineCountsDeltasIgnoreManagerCommits(previousCommit, latestCommit)
			if err != nil {
				return err
			}

			for file, delta := range deltas {
				if delta == 0 {
					continue
				}

				if delta > 0 {
					addedOrModified = append(addedOrModified, file)
				} else if delta < 0 {
					if -delta < lineCounts[file] {
						addedOrModified = append(addedOrModified, file)
					} else {
						removed = append(removed, file)
					}
				}
			}
		}

		filesToPush := make(map[string][]byte)
		for _, filename := range addedOrModified {
			if err = common.PrepareForPush(filename, &filesToPush, cfg); err != nil {
				return err
			}
		}

		common.PushFiles(filesToPush, client)

		if delRemoved {
			for _, removedFile := range removed {
				if err = common.DeleteDashboard(
					removedFile, client, cfg,
				); err != nil {
					logrus.WithFields(logrus.Fields{
						"error":    err,
						"filename": removedFile,
					}).Error("Failed to delete the dashboard")
				}
			}
		}

		previousCommit = latestCommit
		time.Sleep(time.Duration(cfg.Pusher.Config.Interval) * time.Second)
	}
}
