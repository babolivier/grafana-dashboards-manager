package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config is the Go representation of the configuration file. It is filled when
// parsing the said file.
type Config struct {
	Grafana GrafanaSettings `yaml:"grafana"`
	Git     GitSettings     `yaml:"git"`
	Webhook WebhookSettings `yaml:"webhook"`
}

// GrafanaSettings contains the data required to talk to the Grafana HTTP API.
type GrafanaSettings struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// GitSettings contains the data required to interact with the Git repository.
type GitSettings struct {
	URL            string `yaml:"url"`
	User           string `yaml:"user"`
	PrivateKeyPath string `yaml:"private_key"`
	ClonePath      string `yaml:"clone_path"`
}

// WebhookSettings contains the data required to setup the GitLab webhook.
// We declare the port as a string because, although it's a number, it's only
// used in a string concatenation when creating the webhook.
type WebhookSettings struct {
	Interface string `yaml:"interface"`
	Port      string `yaml:"port"`
	Path      string `yaml:"path"`
	Secret    string `yaml:"secret"`
}

func Load(filename string) (cfg *Config, err error) {
	rawCfg, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	cfg = new(Config)
	err = yaml.Unmarshal(rawCfg, cfg)
	return
}
