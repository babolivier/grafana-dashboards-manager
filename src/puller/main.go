package main

import (
	"flag"

	"config"
	"grafana"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to the configuration file")
)

func main() {
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		panic(err)
	}

	client := grafana.NewClient(cfg.Grafana.BaseURL, cfg.Grafana.APIKey)
	if err := PullGrafanaAndCommit(client, cfg); err != nil {
		panic(err)
	}
}
