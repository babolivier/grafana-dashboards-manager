package main

import (
	"flag"

	"config"
	"grafana"
	"logger"
)

// Some variables need to be global to the package since we need them in the
// webhook handlers.
// TODO: Find a better way to pass it to the handlers
var (
	grafanaClient *grafana.Client
	cfg           *config.Config
	deleteRemoved = flag.Bool("delete-removed", false, "For each file removed from Git, delete the corresponding dashboard on the Grafana API")
)

func main() {
	var err error

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	logger.LogConfig()

	cfg, err = config.Load(*configFile)
	if err != nil {
		panic(err)
	}

	grafanaClient = grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)

	if err = SetupWebhook(cfg); err != nil {
		panic(err)
	}
}
