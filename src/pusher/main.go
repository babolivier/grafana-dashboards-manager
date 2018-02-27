package main

import (
	"flag"
	"os"

	"config"
	"grafana"
	"logger"
	"pusher/poller"
	"pusher/webhook"

	"github.com/sirupsen/logrus"
)

var (
	deleteRemoved = flag.Bool("delete-removed", false, "For each file removed from Git, delete the corresponding dashboard on the Grafana API")
)

func main() {
	var err error

	// Define this flag in the main function because else it would cause a
	// conflict with the one in the puller.
	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	// Load the logger's configuration.
	logger.LogConfig()

	// Load the configuration.
	cfg, err := config.Load(*configFile)
	if err != nil {
		logrus.Panic(err)
	}

	if cfg.Git == nil || cfg.Pusher == nil {
		logrus.Info("The git configuration or the pusher configuration (or both) is not defined in the configuration file. The pusher cannot start unless both are defined.")
		os.Exit(0)
	}

	// Initialise the Grafana API client.
	grafanaClient := grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)

	// Set up either a webhook or a poller depending on the mode specified in the
	// configuration file.
	switch cfg.Pusher.Mode {
	case "webhook":
		err = webhook.Setup(cfg, grafanaClient, *deleteRemoved)
		break
	case "git-pull":
		err = poller.Setup(cfg, grafanaClient, *deleteRemoved)
	}

	if err != nil {
		logrus.Panic(err)
	}
}
