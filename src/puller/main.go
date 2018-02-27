package main

import (
	"flag"

	"config"
	"grafana"
	"logger"

	"github.com/sirupsen/logrus"
)

func main() {
	// Define this flag in the main function because else it would cause a
	// conflict with the one in the pusher.
	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	// Load the logger's configuration.
	logger.LogConfig()

	// Load the configuration.
	cfg, err := config.Load(*configFile)
	if err != nil {
		logrus.Panic(err)
	}

	// Tell the user which sync mode we use.
	var syncMode string
	if cfg.Git != nil {
		syncMode = "git"
	} else {
		syncMode = "simple"
	}

	logrus.WithFields(logrus.Fields{
		"sync_mode": syncMode,
	}).Info("Sync mode set")

	// Initialise the Grafana API client.
	client := grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)
	// Run the puller.
	if err := PullGrafanaAndCommit(client, cfg); err != nil {
		logrus.Panic(err)
	}
}
