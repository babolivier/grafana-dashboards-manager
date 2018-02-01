package main

import (
	"flag"

	"config"
	"grafana"
	"logger"
	"pusher/webhook"

	"github.com/sirupsen/logrus"
)

var (
	deleteRemoved = flag.Bool("delete-removed", false, "For each file removed from Git, delete the corresponding dashboard on the Grafana API")
)

func main() {
	var err error

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	logger.LogConfig()

	cfg, err := config.Load(*configFile)
	if err != nil {
		logrus.Panic(err)
	}

	grafanaClient := grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)

	if err = webhook.Setup(cfg, grafanaClient, *deleteRemoved); err != nil {
		logrus.Panic(err)
	}
}
