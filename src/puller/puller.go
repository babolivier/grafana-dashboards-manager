package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"git"
	"grafana"
)

type diffVersion struct {
	oldVersion int
	newVersion int
}

func Pull() error {
	dv := make(map[string]diffVersion)

	dbVersions, err := getDashboardsVersions()
	if err != nil {
		return err
	}

	repo, err := git.Sync(
		*repoURL,
		*clonePath,
		*privateKeyPath,
	)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	client := grafana.NewClient(*grafanaURL, *grafanaAPIKey)
	uris, err := client.GetDashboardsURIs()
	if err != nil {
		return err
	}

	for _, uri := range uris {
		dashboard, err := client.GetDashboard(uri)
		if err != nil {
			return err
		}

		version, ok := dbVersions[dashboard.Slug]
		if !ok || dashboard.Version > version {
			slugExt := dashboard.Slug + ".json"
			if err = rewriteFile(
				*clonePath+"/"+slugExt,
				dashboard.RawJSON,
			); err != nil {
				return err
			}

			if _, err = w.Add(slugExt); err != nil {
				return err
			}

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

	if !status.IsClean() {
		if err = writeVersions(dbVersions, dv); err != nil {
			return err
		}

		if _, err = w.Add("versions.json"); err != nil {
			return err
		}

		if _, err = git.Commit(getCommitMessage(dv), w); err != nil {
			return err
		}

	}

	if err = git.Push(repo, *privateKeyPath); err != nil {
		return err
	}

	return nil
}

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

func indent(srcJSON []byte) (indentedJSON []byte, err error) {
	buf := bytes.NewBuffer(nil)
	if err = json.Indent(buf, srcJSON, "", "\t"); err != nil {
		return
	}

	indentedJSON, err = ioutil.ReadAll(buf)
	return
}

func getCommitMessage(dv map[string]diffVersion) string {
	message := "Updated dashboards\n"

	for slug, diff := range dv {
		message += fmt.Sprintf("%s: %d => %d\n", slug, diff.oldVersion, diff.newVersion)
	}

	return message
}
