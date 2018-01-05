package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// getDashboardsVersions reads the "versions.json" file at the root of the git
// repository and returns its content as a map.
// If the file doesn't exist, returns an empty map.
// Return an error if there was an issue looking for the file (except when the
// file doesn't exist), reading it or formatting its content into a map.
func getDashboardsVersions(clonePath string) (versions map[string]int, err error) {
	versions = make(map[string]int)

	filename := clonePath + "/versions.json"

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

// writeVersions updates or creates the "versions.json" file at the root of the
// git repository. It takes as parameter a map of versions computed by
// getDashboardsVersions and a map linking a dashboard slug to an instance of
// diffVersion instance, and uses them both to compute an updated map of
// versions that it will convert to JSON, indent and write down into the
// "versions.json" file.
// Returns an error if there was an issue when conerting to JSON, indenting or
// writing on disk.
func writeVersions(
	versions map[string]int, dv map[string]diffVersion, clonePath string,
) (err error) {
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

	filename := clonePath + "/versions.json"
	return rewriteFile(filename, indentedJSON)
}

// commitNewVersions creates a git commit from updated dashboard files (that
// have previously been added to the git index) and an updated "versions.json"
// file that it creates (with writeVersions) and add to the index.
// Returns an error if there was an issue when creating the "versions.json"
// file, adding it to the index or creating the commit.
func commitNewVersions(
	versions map[string]int, dv map[string]diffVersion, worktree *gogit.Worktree,
	clonePath string,
) (err error) {
	if err = writeVersions(versions, dv, clonePath); err != nil {
		return err
	}

	if _, err = worktree.Add("versions.json"); err != nil {
		return err
	}

	_, err = worktree.Commit(getCommitMessage(dv), &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Grafana Dashboard Manager",
			Email: "grafana@cozycloud.cc",
			When:  time.Now(),
		},
	})

	return
}

// getCommitMessage creates a commit message that summarises the version updates
// included in the commit.
func getCommitMessage(dv map[string]diffVersion) string {
	message := "Updated dashboards\n"

	for slug, diff := range dv {
		message += fmt.Sprintf(
			"%s: %d => %d\n", slug, diff.oldVersion, diff.newVersion,
		)
	}

	return message
}
