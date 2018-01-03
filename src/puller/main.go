package main

import (
	"flag"
	"os"

	"grafana"
)

var (
	grafanaURL     = flag.String("grafana-url", "", "Base URL of the Grafana instance")
	grafanaAPIKey  = flag.String("api-key", "", "API key to use in authenticated requests")
	clonePath      = flag.String("clone-path", "/tmp/grafana-dashboards", "Path to directory where the repo will be cloned")
	repoURL        = flag.String("git-repo", "", "SSH URL for the Git repository, without the user part")
	privateKeyPath = flag.String("private-key", "", "Path to the private key used to talk with the Git remote")
)

func main() {
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

	client := grafana.NewClient(*grafanaURL, *grafanaAPIKey)
	if err := Pull(client); err != nil {
		panic(err)
	}
}
