package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"config"
	"git"
	"grafana"

	"github.com/sirupsen/logrus"
	gogit "gopkg.in/src-d/go-git.v4"
)

// diffVersion represents a dashboard version diff.
type diffVersion struct {
	oldVersion int
	newVersion int
}

// PullGrafanaAndCommit pulls all the dashboards from Grafana except the ones
// which name starts with "test", then commits each of them to Git except for
// those that have a newer or equal version number already versionned in the
// repo.
func PullGrafanaAndCommit(client *grafana.Client, cfg *config.Config) (err error) {
	var repo *git.Repository
	var w *gogit.Worktree
	var syncPath string

	// Only do Git stuff if there's a configuration for that. On "simple sync"
	// mode, we don't need do do any versioning.
	// We need to set syncPath accordingly, though, because we use it later.
	if cfg.Git != nil {
		syncPath = cfg.Git.ClonePath

		// Clone or pull the repo
		repo, _, err = git.NewRepository(cfg.Git)
		if err != nil {
			return err
		}

		if err = repo.Sync(false); err != nil {
			return err
		}

		w, err = repo.Repo.Worktree()
		if err != nil {
			return err
		}
	} else {
		syncPath = cfg.SimpleSync.SyncPath
	}

	// Get URIs for all known dashboards
	logrus.Info("Getting dashboard URIs")
	uris, err := client.GetDashboardsURIs()
	if err != nil {
		return err
	}

	dv := make(map[string]diffVersion)

	// Load versions
	logrus.Info("Getting local dashboard versions")
	dbVersions, err := getDashboardsVersions(syncPath)
	if err != nil {
		return err
	}

	// Iterate over the dashboards URIs
	for _, uri := range uris {
		logrus.WithFields(logrus.Fields{
			"uri": uri,
		}).Info("Retrieving dashboard")

		// Retrieve the dashboard JSON
		dashboard, err := client.GetDashboard(uri)
		if err != nil {
			return err
		}

		if len(cfg.Grafana.IgnorePrefix) > 0 {
			if strings.HasPrefix(dashboard.Slug, cfg.Grafana.IgnorePrefix) {
				logrus.WithFields(logrus.Fields{
					"uri":    uri,
					"name":   dashboard.Name,
					"prefix": cfg.Grafana.IgnorePrefix,
				}).Info("Dashboard name starts with specified prefix, skipping")

				continue
			}
		}

		// Check if there's a version for this dashboard in the data loaded from
		// the "versions.json" file. If there's a version and it's older (lower
		// version number) than the version we just retrieved from the Grafana
		// API, or if there's no known version (ok will be false), write the
		// changes in the repo and add the modified file to the git index.
		version, ok := dbVersions[dashboard.Slug]
		if !ok || dashboard.Version > version {
			logrus.WithFields(logrus.Fields{
				"uri":           uri,
				"name":          dashboard.Name,
				"local_version": version,
				"new_version":   dashboard.Version,
			}).Info("Grafana has a newer version, updating")

			if err = addDashboardChangesToRepo(
				dashboard, syncPath, w,
			); err != nil {
				return err
			}

			// We don't need to check for the value of ok because if ok is false
			// version will be initialised to the 0-value of the int type, which
			// is 0, so the previous version number will be considered to be 0,
			// which is the behaviour we want.
			dv[dashboard.Slug] = diffVersion{
				oldVersion: version,
				newVersion: dashboard.Version,
			}
		}
	}

	// Only do Git stuff if there's a configuration for that. On "simple sync"
	// mode, we don't need do do any versioning.
	if cfg.Git != nil {
		var status gogit.Status
		status, err = w.Status()
		if err != nil {
			return err
		}

		// Check if there's uncommited changes, and if that's the case, commit
		// them.
		if !status.IsClean() {
			logrus.Info("Comitting changes")

			if err = commitNewVersions(dbVersions, dv, w, cfg); err != nil {
				return err
			}
		}

		// Push the changes (we don't do it in the if clause above in case there
		// are pending commits in the local repo that haven't been pushed yet).
		if err = repo.Push(); err != nil {
			return err
		}
	} else {
		// If we're on simple sync mode, write versions and don't do anything
		// else.
		if err = writeVersions(dbVersions, dv, syncPath); err != nil {
			return err
		}
	}

	return nil
}

// addDashboardChangesToRepo writes a dashboard content in a file, then adds the
// file to the git index so it can be comitted afterwards.
// Returns an error if there was an issue with either of the steps.
func addDashboardChangesToRepo(
	dashboard *grafana.Dashboard, clonePath string, worktree *gogit.Worktree,
) error {
	slugExt := dashboard.Slug + ".json"
	if err := rewriteFile(clonePath+"/"+slugExt, dashboard.RawJSON); err != nil {
		return err
	}

	// If worktree is nil, it means that it hasn't been initialised, which means
	// the sync mode is "simple sync" and not Git.
	if worktree != nil {
		if _, err := worktree.Add(slugExt); err != nil {
			return err
		}
	}

	return nil
}

// rewriteFile removes a given file and re-creates it with a new content. The
// content is provided as JSON, and is then indented before being written down.
// We need the whole "remove then recreate" thing because, if the file already
// exists, ioutil.WriteFile will append the content to it. However, we want to
// replace the oldest version with another (so git can diff it), so we re-create
// the file with the changed content.
// Returns an error if there was an issue when removing or writing the file, or
// indenting the JSON content.
func rewriteFile(filename string, content []byte) error {
	if err := os.Remove(filename); err != nil {
		pe, ok := err.(*os.PathError)
		if !ok || pe.Err.Error() != "no such file or directory" {
			return err
		}
	}

	indentedContent, err := indent(content)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, indentedContent, 0644)
}

// indent indents a given JSON content with tabs.
// We need to indent the content as the Grafana API returns a one-lined JSON
// string, which isn't great to work with.
// Returns an error if there was an issue with the process.
func indent(srcJSON []byte) (indentedJSON []byte, err error) {
	buf := bytes.NewBuffer(nil)
	if err = json.Indent(buf, srcJSON, "", "\t"); err != nil {
		return
	}

	indentedJSON, err = ioutil.ReadAll(buf)
	return
}
