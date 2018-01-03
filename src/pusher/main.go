package main

import (
	"flag"
	"os"

	"grafana"
)

// The Grafana API client needs to be global to the package since we need it in
// the webhook handlers
// TODO: Find a better way to pass it to the handlers
var grafanaClient *grafana.Client

var (
	grafanaURL       = flag.String("grafana-url", "", "Base URL of the Grafana instance")
	grafanaAPIKey    = flag.String("api-key", "", "API key to use in authenticated requests")
	clonePath        = flag.String("clone-path", "/tmp/grafana-dashboards", "Path to directory where the repo will be cloned")
	repoURL          = flag.String("git-repo", "", "SSH URL for the Git repository, without the user part")
	privateKeyPath   = flag.String("private-key", "", "Path to the private key used to talk with the Git remote")
	webhookInterface = flag.String("webhook-interface", "127.0.0.1", "Interface on which the GitLab webhook will be exposed")
	webhookPort      = flag.Int("webhook-port", 8080, "Port on which the GitLab webhook will be exposed")
	webhookPath      = flag.String("webhook-path", "/gitlab-webhook", "Path at which GitLab should send payloads to the webhook")
	webhookSecret    = flag.String("webhook-secret", "", "Secret GitLab will use to send payloads to the webhook")
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
	if *webhookSecret == "" {
		println("Error: No webhook secret provided")
		flag.Usage()
		os.Exit(1)
	}

	grafanaClient = grafana.NewClient(*grafanaURL, *grafanaAPIKey)

	if err := SetupWebhook(); err != nil {
		panic(err)
	}
}
