package config

import (
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/sirupsen/logrus"
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
	BaseURL      string `yaml:"base_url"`
	APIKey       string `yaml:"api_key"`
	IgnorePrefix string `yaml:"ignore_prefix,omitempty"`
}

// GitSettings contains the data required to interact with the Git repository.
type GitSettings struct {
	URL            string              `yaml:"url"`
	User           string              `yaml:"user"`
	PrivateKeyPath string              `yaml:"private_key"`
	ClonePath      string              `yaml:"clone_path"`
	CommitsAuthor  CommitsAuthorConfig `yaml:"commits_author"`
}

// CommitsAuthorConfig contains the configuration (name + email address) to use
// when commiting to Git.
type CommitsAuthorConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
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

// Load opens a given configuration file and parses it into an instance of the
// Config structure.
// Returns an error if there was an issue whith reading or parsing the file.
func Load(filename string) (cfg *Config, err error) {
	rawCfg, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"config_file": filename,
	}).Info("Loading configuration")

	cfg = new(Config)
	err = yaml.Unmarshal(rawCfg, cfg)
	cfg.Grafana.IgnorePrefix = strings.ToLower(cfg.Grafana.IgnorePrefix)
	return
}
