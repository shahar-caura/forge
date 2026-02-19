package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration with YAML unmarshaling from strings like "45m".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

// Config is the top-level forge configuration.
type Config struct {
	VCS      VCSConfig      `yaml:"vcs"`
	Tracker  TrackerConfig  `yaml:"tracker"`
	Notifier NotifierConfig `yaml:"notifier"`
	Agent    AgentConfig    `yaml:"agent"`
	Worktree WorktreeConfig `yaml:"worktree"`
	State    StateConfig    `yaml:"state"`
	CR       CRConfig       `yaml:"cr"`
	Editor   EditorConfig   `yaml:"editor"`
}

// CRConfig controls the code review feedback loop.
type CRConfig struct {
	Enabled        bool     `yaml:"enabled"`
	PollTimeout    Duration `yaml:"poll_timeout"`
	PollInterval   Duration `yaml:"poll_interval"`
	CommentPattern string   `yaml:"comment_pattern"`
	FixStrategy    string   `yaml:"fix_strategy"`
}

type StateConfig struct {
	Retention Duration `yaml:"retention"` // default 7 days (168h)
}

type VCSConfig struct {
	Provider   string `yaml:"provider"`
	Repo       string `yaml:"repo"`
	BaseBranch string `yaml:"base_branch"`
}

type TrackerConfig struct {
	Provider string `yaml:"provider"`
	Project  string `yaml:"project"`
	BaseURL  string `yaml:"base_url"`
	Email    string `yaml:"email"`
	Token    string `yaml:"token"`
	BoardID  string `yaml:"board_id"`
}

type NotifierConfig struct {
	Provider   string `yaml:"provider"`
	WebhookURL string `yaml:"webhook_url"`
}

type AgentConfig struct {
	Provider string   `yaml:"provider"`
	Timeout  Duration `yaml:"timeout"`
}

type WorktreeConfig struct {
	CreateCmd string `yaml:"create_cmd"`
	RemoveCmd string `yaml:"remove_cmd"`
	Cleanup   bool   `yaml:"cleanup"`
}

type EditorConfig struct {
	Enabled bool   `yaml:"enabled"`
	Command string `yaml:"command"`
}

const (
	defaultTimeout      = 45 * time.Minute
	defaultRetention    = 7 * 24 * time.Hour // 168h
	defaultPollTimeout  = 5 * time.Minute
	defaultPollInterval = 15 * time.Second
)

// Load reads, expands env vars, parses, and validates a forge config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Agent.Timeout.Duration == 0 {
		cfg.Agent.Timeout.Duration = defaultTimeout
	}
	if cfg.State.Retention.Duration == 0 {
		cfg.State.Retention.Duration = defaultRetention
	}
	if cfg.CR.Enabled {
		if cfg.CR.PollTimeout.Duration == 0 {
			cfg.CR.PollTimeout.Duration = defaultPollTimeout
		}
		if cfg.CR.PollInterval.Duration == 0 {
			cfg.CR.PollInterval.Duration = defaultPollInterval
		}
		if cfg.CR.FixStrategy == "" {
			cfg.CR.FixStrategy = "amend"
		}
	}

	if cfg.Editor.Command == "" {
		cfg.Editor.Command = "code"
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	var errs []error

	if cfg.VCS.Provider == "" {
		errs = append(errs, errors.New("vcs.provider is required"))
	}
	if cfg.VCS.Repo == "" {
		errs = append(errs, errors.New("vcs.repo is required"))
	}
	if cfg.VCS.BaseBranch == "" {
		errs = append(errs, errors.New("vcs.base_branch is required"))
	}
	if cfg.Agent.Provider == "" {
		errs = append(errs, errors.New("agent.provider is required"))
	}
	if cfg.Agent.Timeout.Duration <= 0 {
		errs = append(errs, errors.New("agent.timeout must be positive"))
	}
	if cfg.Worktree.CreateCmd == "" {
		errs = append(errs, errors.New("worktree.create_cmd is required"))
	}

	// Only validate tracker fields when provider is set.
	if cfg.Tracker.Provider != "" {
		if cfg.Tracker.Project == "" {
			errs = append(errs, errors.New("tracker.project is required when tracker.provider is set"))
		}
		if cfg.Tracker.BaseURL == "" {
			errs = append(errs, errors.New("tracker.base_url is required when tracker.provider is set"))
		}
		if cfg.Tracker.Email == "" {
			errs = append(errs, errors.New("tracker.email is required when tracker.provider is set"))
		}
		if cfg.Tracker.Token == "" {
			errs = append(errs, errors.New("tracker.token is required when tracker.provider is set"))
		}
	}

	// Only validate notifier fields when provider is set.
	if cfg.Notifier.Provider != "" {
		if cfg.Notifier.WebhookURL == "" {
			errs = append(errs, errors.New("notifier.webhook_url is required when notifier.provider is set"))
		}
	}

	// Only validate CR fields when enabled.
	if cfg.CR.Enabled {
		if cfg.CR.CommentPattern == "" {
			errs = append(errs, errors.New("cr.comment_pattern is required when cr.enabled is true"))
		}
		switch cfg.CR.FixStrategy {
		case "amend", "new-commit":
			// valid
		default:
			errs = append(errs, fmt.Errorf("cr.fix_strategy must be \"amend\" or \"new-commit\", got %q", cfg.CR.FixStrategy))
		}
	}

	return errors.Join(errs...)
}
