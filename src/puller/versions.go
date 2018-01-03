package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"git"

	gogit "gopkg.in/src-d/go-git.v4"
)

func getDashboardsVersions() (versions map[string]int, err error) {
	versions = make(map[string]int)

	filename := *clonePath + "/versions.json"

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		return versions, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &versions)
	return
}

func writeVersions(versions map[string]int, dv map[string]diffVersion) (err error) {
	for slug, diff := range dv {
		versions[slug] = diff.newVersion
	}

	rawJSON, err := json.Marshal(versions)
	if err != nil {
		return
	}

	indentedJSON, err := indent(rawJSON)
	if err != nil {
		return
	}

	filename := *clonePath + "/versions.json"
	return rewriteFile(filename, indentedJSON)
}

func commitNewVersions(
	versions map[string]int, dv map[string]diffVersion, worktree *gogit.Worktree,
) (err error) {
	if err = writeVersions(versions, dv); err != nil {
		return err
	}

	if _, err = worktree.Add("versions.json"); err != nil {
		return err
	}

	_, err = git.Commit(getCommitMessage(dv), worktree)
	return
}

func getCommitMessage(dv map[string]diffVersion) string {
	message := "Updated dashboards\n"

	for slug, diff := range dv {
		message += fmt.Sprintf("%s: %d => %d\n", slug, diff.oldVersion, diff.newVersion)
	}

	return message
}
