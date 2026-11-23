package config

import (
	"path/filepath"
	"testing"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

func TestSaveAndLoadUserConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("CICD_CONFIG", cfgPath)

	// Missing config returns defaults with empty owner.
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	if cfg.Owner != "" || cfg.Repo != DefaultRepoName || cfg.Ref != DefaultRef {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	cfg.Owner = "alice"
	cfg.Ref = "v2"
	cfg.DefaultMode = model.ModeStandalone
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}

	loaded, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() reload error = %v", err)
	}
	if loaded.Owner != "alice" || loaded.Ref != "v2" || loaded.DefaultMode != model.ModeStandalone {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestResolveSourcePrecedence(t *testing.T) {
	t.Setenv("CICD_SOURCE", "")

	userCfg := UserConfig{Owner: "user-default", Repo: "cicd-automate", Ref: "v1"}

	// 1. User config used when no overrides exist.
	res, err := ResolveSource(ResolveOptions{
		UserCfg:       userCfg,
		FallbackOwner: "git-fallback",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Origin != "user-config" || res.Ref.String() != "user-default/cicd-automate@v1" {
		t.Fatalf("got %+v, want user-config user-default/cicd-automate@v1", res)
	}

	// 2. Env overrides user config.
	res, err = ResolveSource(ResolveOptions{
		EnvSource: "env-org/ci@v3",
		UserCfg:   userCfg,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Origin != "env" || res.Ref.String() != "env-org/ci@v3" {
		t.Fatalf("got %+v, want env env-org/ci@v3", res)
	}

	// 3. Per-project manifest overrides env and user config.
	res, err = ResolveSource(ResolveOptions{
		ProjectSource: "client-org/custom-ci@v2",
		EnvSource:     "env-org/ci@v3",
		UserCfg:       userCfg,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Origin != "project" || res.Ref.String() != "client-org/custom-ci@v2" {
		t.Fatalf("got %+v, want project client-org/custom-ci@v2", res)
	}

	// 4. CLI flag overrides everything, and CLIRef overrides ref.
	res, err = ResolveSource(ResolveOptions{
		CLISource:     "cli-org/workflows@v1",
		CLIRef:        "v4",
		ProjectSource: "client-org/custom-ci@v2",
		UserCfg:       userCfg,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Origin != "flag" || res.Ref.String() != "cli-org/workflows@v4" {
		t.Fatalf("got %+v, want flag cli-org/workflows@v4", res)
	}

	// 5. No config and no fallback returns actionable error.
	_, err = ResolveSource(ResolveOptions{UserCfg: UserConfig{}})
	if err == nil {
		t.Fatal("expected error when no source is configured")
	}
}
