package main

import (
	"flag"

	"config"
	"grafana"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
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
