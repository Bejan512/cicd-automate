package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	DefaultRepoName        = "cicd-automate"
	DefaultRef             = "v1"
	DefaultDockerRegistry  = "ghcr.io"
	DefaultReleaseStrategy = "release-please"
)

// UserConfig stores per-user defaults so any developer or organization can point
// cicd-automate at their own reusable workflows repository while allowing per-project overrides.
type UserConfig struct {
	Owner                  string             `yaml:"owner,omitempty"`
	Repo                   string             `yaml:"repo,omitempty"`
	Ref                    string             `yaml:"ref,omitempty"`
	DefaultMode            model.WorkflowMode `yaml:"default_mode,omitempty"`
	DefaultDockerRegistry  string             `yaml:"default_docker_registry,omitempty"`
	DefaultReleaseStrategy string             `yaml:"default_release_strategy,omitempty"`
}

// ConfigPath returns the path to the per-user configuration file.
// Respects CICD_CONFIG (explicit file path) and XDG_CONFIG_HOME.
func ConfigPath() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("CICD_CONFIG")); explicit != "" {
		return explicit, nil
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "cicd-automate", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cicd-automate", "config.yaml"), nil
}

// LoadUserConfig reads the per-user config file. If it does not exist yet, returns defaults.
func LoadUserConfig() (UserConfig, error) {
	cfg := UserConfig{
		Repo:                   DefaultRepoName,
		Ref:                    DefaultRef,
		DefaultMode:            model.ModeReusable,
		DefaultDockerRegistry:  DefaultDockerRegistry,
		DefaultReleaseStrategy: DefaultReleaseStrategy,
	}

	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read user config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse user config %s: %w", path, err)
	}

	if cfg.Repo == "" {
		cfg.Repo = DefaultRepoName
	}
	if cfg.Ref == "" {
		cfg.Ref = DefaultRef
	}
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = model.ModeReusable
	}
	if cfg.DefaultDockerRegistry == "" {
		cfg.DefaultDockerRegistry = DefaultDockerRegistry
	}
	if cfg.DefaultReleaseStrategy == "" {
		cfg.DefaultReleaseStrategy = DefaultReleaseStrategy
	}

	return cfg, nil
}

// SaveUserConfig persists the per-user config file with 0600 permissions.
func SaveUserConfig(cfg UserConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal user config: %w", err)
	}

	header := "# cicd-automate per-user configuration\n# Override per project with --source or in .github/cicd.yml\n"
	if err := os.WriteFile(path, append([]byte(header), data...), 0o600); err != nil {
		return fmt.Errorf("write user config %s: %w", path, err)
	}
	return nil
}

// SourceRef returns the user's configured default SourceRef if Owner is set.
func (c UserConfig) SourceRef() (model.SourceRef, bool) {
	if strings.TrimSpace(c.Owner) == "" {
		return model.SourceRef{}, false
	}
	repo := c.Repo
	if repo == "" {
		repo = DefaultRepoName
	}
	ref := c.Ref
	if ref == "" {
		ref = DefaultRef
	}
	return model.SourceRef{
		Owner: strings.TrimSpace(c.Owner),
		Repo:  strings.TrimSpace(repo),
		Ref:   strings.TrimSpace(ref),
	}, true
}

// ResolveOptions configures how ResolveSource determines the active reusable workflow source.
type ResolveOptions struct {
	CLISource      string     // --source flag (highest precedence)
	CLIOwner       string     // --owner flag
	CLIRef         string     // --ref flag (overrides ref on resolved source)
	ProjectSource  string     // source field from .github/cicd.yml (per-project override)
	EnvSource      string     // CICD_SOURCE env var (if empty, read from os.Getenv)
	UserCfg        UserConfig // per-user config
	FallbackOwner  string     // optional fallback (e.g. git remote owner or gh user)
}

// ResolvedSource describes the chosen SourceRef and where it came from.
type ResolvedSource struct {
	Ref        model.SourceRef
	Origin     string // "flag", "project", "env", "user-config", "detected"
}

// ResolveSource resolves the workflow source according to precedence:
// 1. CLI --source / --owner flags
// 2. Per-project manifest (.github/cicd.yml)
// 3. Environment variable CICD_SOURCE
// 4. Per-user config (~/.config/cicd-automate/config.yaml)
// 5. Fallback owner (e.g. git remote / gh user)
func ResolveSource(opts ResolveOptions) (ResolvedSource, error) {
	var res ResolvedSource
	var found bool

	repoDefault := opts.UserCfg.Repo
	if repoDefault == "" {
		repoDefault = DefaultRepoName
	}
	refDefault := opts.UserCfg.Ref
	if refDefault == "" {
		refDefault = DefaultRef
	}

	switch {
	case strings.TrimSpace(opts.CLISource) != "":
		parsed, err := model.ParseSourceRef(opts.CLISource)
		if err != nil {
			return ResolvedSource{}, err
		}
		res = ResolvedSource{Ref: parsed, Origin: "flag"}
		found = true

	case strings.TrimSpace(opts.CLIOwner) != "":
		res = ResolvedSource{
			Ref: model.SourceRef{
				Owner: strings.TrimSpace(opts.CLIOwner),
				Repo:  repoDefault,
				Ref:   refDefault,
			},
			Origin: "flag",
		}
		found = true

	case strings.TrimSpace(opts.ProjectSource) != "":
		parsed, err := model.ParseSourceRef(opts.ProjectSource)
		if err != nil {
			return ResolvedSource{}, fmt.Errorf("invalid project manifest source: %w", err)
		}
		res = ResolvedSource{Ref: parsed, Origin: "project"}
		found = true

	default:
		envVal := opts.EnvSource
		if envVal == "" {
			envVal = strings.TrimSpace(os.Getenv("CICD_SOURCE"))
		}
		if envVal != "" {
			parsed, err := model.ParseSourceRef(envVal)
			if err != nil {
				return ResolvedSource{}, fmt.Errorf("invalid CICD_SOURCE: %w", err)
			}
			res = ResolvedSource{Ref: parsed, Origin: "env"}
			found = true
		} else if src, ok := opts.UserCfg.SourceRef(); ok {
			res = ResolvedSource{Ref: src, Origin: "user-config"}
			found = true
		} else if strings.TrimSpace(opts.FallbackOwner) != "" {
			res = ResolvedSource{
				Ref: model.SourceRef{
					Owner: strings.TrimSpace(opts.FallbackOwner),
					Repo:  repoDefault,
					Ref:   refDefault,
				},
				Origin: "detected",
			}
			found = true
		}
	}

	if !found {
		return ResolvedSource{}, fmt.Errorf("no workflow source configured: run `cicd config set owner <github-username>` or pass `--source <owner>/<repo>@<ref>`")
	}

	if strings.TrimSpace(opts.CLIRef) != "" {
		res.Ref.Ref = strings.TrimSpace(opts.CLIRef)
	}

	return res, nil
}
