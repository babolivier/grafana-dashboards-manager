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

type diffVersion struct {
	oldVersion int
	newVersion int
}

func Pull(client *grafana.Client) error {
	dv := make(map[string]diffVersion)

	dbVersions, err := getDashboardsVersions()
	if err != nil {
		return err
	}

	repo, err := git.Sync(*repoURL, *clonePath, *privateKeyPath)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

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
			if err = addDashboardChangesToRepo(
				dashboard, version, w, &dv,
			); err != nil {
				return err
			}
		}
	}

	status, err := w.Status()
	if err != nil {
		return err
	}

	if !status.IsClean() {
		if err = commitNewVersions(dbVersions, dv, w); err != nil {
			return err
		}
	}

	if err = git.Push(repo, *privateKeyPath); err != nil {
		return err
	}

	return nil
}

func addDashboardChangesToRepo(
	dashboard *grafana.Dashboard, oldVersion int, worktree *gogit.Worktree,
	dv *map[string]diffVersion,
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

	(*dv)[dashboard.Slug] = diffVersion{
		oldVersion: oldVersion,
		newVersion: dashboard.Version,
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
