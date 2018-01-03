package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"git"
	"grafana"

	gogit "gopkg.in/src-d/go-git.v4"
)

// diffVersion represents a dashboard version diff.
type diffVersion struct {
	oldVersion int
	newVersion int
}

// PullGrafanaAndCommit pulls all the dashboards from Grafana then commits each
// of them to Git except for those that have a newer or equal version number
// already versionned in the repo
func PullGrafanaAndCommit(
	client *grafana.Client,
	repoURL string, clonePath string, privateKeyPath string,
) error {
	// Clone or pull the repo
	repo, err := git.Sync(repoURL, clonePath, privateKeyPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Get URIs for all known dashboards
	uris, err := client.GetDashboardsURIs()
	if err != nil {
		return err
	}

	dv := make(map[string]diffVersion)

	// Load versions
	dbVersions, err := getDashboardsVersions()
	if err != nil {
		return err
	}

	// Iterate over the dashboards URIs
	for _, uri := range uris {
		// Retrieve the dashboard JSON
		dashboard, err := client.GetDashboard(uri)
		if err != nil {
			return err
		}

		// Check if there's a version for this dashboard in the data loaded from
		// the "versions.json" file. If there's a version and it's older (lower
		// version number) than the version we just retrieved from the Grafana
		// API, or if there's no known version (ok will be false), write the
		// changes in the repo and add the modified file to the git index.
		version, ok := dbVersions[dashboard.Slug]
		if !ok || dashboard.Version > version {
			if err = addDashboardChangesToRepo(dashboard, w); err != nil {
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

	status, err := w.Status()
	if err != nil {
		return err
	}

	// Check if there's uncommited changes, and if that's the case, commit them.
	if !status.IsClean() {
		if err = commitNewVersions(dbVersions, dv, w); err != nil {
			return err
		}
	}

	// Push the changes (we don't do it in the if clause above in case there are
	// pending commits in the local repo that haven't been pushed yet).
	if err = git.Push(repo, privateKeyPath); err != nil {
		return err
	}

	return nil
}

// addDashboardChangesToRepo writes a dashboard content in a file, then adds the
// file to the git index so it can be comitted afterwards.
// Returns an error if there was an issue with either of the steps.
func addDashboardChangesToRepo(
	dashboard *grafana.Dashboard, worktree *gogit.Worktree,
) error {
	slugExt := dashboard.Slug + ".json"
	if err := rewriteFile(
		*clonePath+"/"+slugExt,
		dashboard.RawJSON,
	); err != nil {
		return err
	}

	if _, err := worktree.Add(slugExt); err != nil {
		return err
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
