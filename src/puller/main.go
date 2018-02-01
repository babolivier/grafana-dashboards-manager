package main

import (
	"flag"

	"config"
	"grafana"
	"logger"

	"github.com/sirupsen/logrus"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	logger.LogConfig()

	cfg, err := config.Load(*configFile)
	if err != nil {
		logrus.Panic(err)
	}

	client := grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)
	if err := PullGrafanaAndCommit(client, cfg); err != nil {
		logrus.Panic(err)
	}
}
