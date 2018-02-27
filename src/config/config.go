package config

import (
	"errors"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

var (
	ErrPusherInvalidSyncMode   = errors.New("Invalid sync mode in the pusher settings")
	ErrPusherConfigNotMatching = errors.New("The pusher config doesn't match with the one expected from the pusher sync mode")
	ErrNoSyncSettings          = errors.New("At least one of the simple_sync or the git settings must be set")
)

// Config is the Go representation of the configuration file. It is filled when
// parsing the said file.
type Config struct {
	Grafana    GrafanaSettings     `yaml:"grafana"`
	SimpleSync *SimpleSyncSettings `yaml:"simple_sync,omitempty"`
	Git        *GitSettings        `yaml:"git,omitempty"`
	Pusher     *PusherSettings     `yaml:"pusher,omitempty"`
}

// GrafanaSettings contains the data required to talk to the Grafana HTTP API.
type GrafanaSettings struct {
	BaseURL      string `yaml:"base_url"`
	APIKey       string `yaml:"api_key"`
	IgnorePrefix string `yaml:"ignore_prefix,omitempty"`
}

// SimpleSyncSettings contains minimal data on the synchronisation process. It is
// expected to be found if there is no Git settings.
// If both simple sync settings and Git settings are found, the Git settings
// will be used.
type SimpleSyncSettings struct {
	SyncPath string `yaml:"sync_path"`
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

// PusherConfig contains the data required to setup either the GitLab webhook or
// the poller.
// When using the GitLab webhook, we declare the port as a string because,
// although it's a number, it's only used in a string concatenation when
// creating the webhook.
type PusherConfig struct {
	Interface string `yaml:"interface,omitempty"`
	Port      string `yaml:"port,omitempty"`
	Path      string `yaml:"path,omitempty"`
	Secret    string `yaml:"secret,omitempty"`
	Interval  int64  `yaml:"interval,omitempty"`
}

// PusherSettings contains the settings to configure the Git->Grafana pusher.
type PusherSettings struct {
	Mode   string       `yaml:"sync_mode"`
	Config PusherConfig `yaml:"config"`
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
	if err = yaml.Unmarshal(rawCfg, cfg); err != nil {
		return
	}

	// Check if at least one settings group exists for synchronisation settings.
	if cfg.Git == nil && cfg.SimpleSync == nil {
		err = ErrNoSyncSettings
		return
	}

	// Since we always compare the prefix against a slug, we need to make sure
	// the prefix is a slug itself.
	cfg.Grafana.IgnorePrefix = slug.Make(cfg.Grafana.IgnorePrefix)
	// Make sure the pusher's config is valid, as the parser can't do it.
	err = validatePusherSettings(cfg.Pusher)
	return
}

// validatePusherSettings checks the pusher config against the one expected from
// looking at its sync mode.
// Returns an error if the sync mode isn't in the allowed modes, or if at least
// one of the fields expected to hold a non-zero-value holds the zero-value for
// its type.
func validatePusherSettings(cfg *PusherSettings) error {
	config := cfg.Config
	var configValid bool
	switch cfg.Mode {
	case "webhook":
		configValid = len(config.Interface) > 0 && len(config.Port) > 0 &&
			len(config.Path) > 0 && len(config.Secret) > 0
		break
	case "git-pull":
		configValid = config.Interval > 0
		break
	default:
		return ErrPusherInvalidSyncMode
	}

	if !configValid {
		return ErrPusherConfigNotMatching
	}

	return nil
}
