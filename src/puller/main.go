package main

import (
	"bytes"
	"encoding/json"
	"flag"
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

var (
	grafanaURL     = flag.String("grafana-url", "", "Base URL of the Grafana instance")
	grafanaAPIKey  = flag.String("api-key", "", "API key to use in authenticated requests")
	clonePath      = flag.String("clone-path", "/tmp/grafana-dashboards", "Path to directory where the repo will be cloned")
	repoURL        = flag.String("git-repo", "", "SSH URL for the Git repository, without the user part")
	privateKeyPath = flag.String("private-key", "", "Path to the private key used to talk with the Git remote")
)

func main() {
	dv := make(map[string]diffVersion)

	flag.Parse()

	if *grafanaURL == "" {
		println("Error: No Grafana URL provided")
		flag.Usage()
		os.Exit(1)
	}
	if *grafanaAPIKey == "" {
		println("Error: No Grafana API key provided")
		flag.Usage()
		os.Exit(1)
	}
	if *repoURL == "" {
		println("Error: No Git repository provided")
		flag.Usage()
		os.Exit(1)
	}
	if *privateKeyPath == "" {
		println("Error: No private key provided")
		flag.Usage()
		os.Exit(1)
	}

	dbVersions, err := getDashboardsVersions()
	if err != nil {
		panic(err)
	}

	repo, err := git.Sync(
		*repoURL,
		*clonePath,
		*privateKeyPath,
	)
	if err != nil {
		panic(err)
	}

	w, err := repo.Worktree()
	if err != nil {
		panic(err)
	}

	client := grafana.NewClient(*grafanaURL, *grafanaAPIKey)
	uris, err := client.GetDashboardsURIs()
	if err != nil {
		panic(err)
	}

	for _, uri := range uris {
		dashboard, err := client.GetDashboard(uri)
		if err != nil {
			panic(err)
		}

		version, ok := dbVersions[dashboard.Slug]
		if !ok || dashboard.Version > version {
			slugExt := dashboard.Slug + ".json"
			if err = rewriteFile(
				*clonePath+"/"+slugExt,
				dashboard.RawJSON,
			); err != nil {
				panic(err)
			}

			if _, err = w.Add(slugExt); err != nil {
				panic(err)
			}

			dv[dashboard.Slug] = diffVersion{
				oldVersion: version,
				newVersion: dashboard.Version,
			}
		}
	}

	status, err := w.Status()
	if err != nil {
		panic(err)
	}

	if !status.IsClean() {
		if err = writeVersions(dbVersions, dv); err != nil {
			panic(err)
		}

		if _, err = w.Add("versions.json"); err != nil {
			panic(err)
		}

		if _, err = git.Commit(getCommitMessage(dv), w); err != nil {
			panic(err)
		}

	}

	if err = git.Push(repo, *privateKeyPath); err != nil {
		panic(err)
	}
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
