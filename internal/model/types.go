package model

import (
	"fmt"
	"strings"
)

// WorkflowMode controls whether generated workflows call centralized reusable workflows
// or inline all steps directly into standalone workflow files.
type WorkflowMode string

const (
	ModeReusable   WorkflowMode = "reusable"
	ModeStandalone WorkflowMode = "standalone"
)

// SourceRef identifies a GitHub repository and git ref hosting reusable workflows,
// e.g. "octocat/cicd-automate@v1".
type SourceRef struct {
	Owner string `yaml:"owner,omitempty" json:"owner"`
	Repo  string `yaml:"repo,omitempty" json:"repo"`
	Ref   string `yaml:"ref,omitempty" json:"ref"`
}

// ParseSourceRef parses strings like "owner/repo@v1" or "owner/repo" (defaulting ref to "v1").
func ParseSourceRef(raw string) (SourceRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SourceRef{}, fmt.Errorf("empty workflow source")
	}

	ref := "v1"
	repoPart := raw
	if idx := strings.Index(raw, "@"); idx != -1 {
		repoPart = raw[:idx]
		ref = strings.TrimSpace(raw[idx+1:])
		if ref == "" {
			return SourceRef{}, fmt.Errorf("invalid source %q: missing ref after '@'", raw)
		}
	}

	parts := strings.Split(repoPart, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return SourceRef{}, fmt.Errorf("invalid source %q: expected <owner>/<repo>[@<ref>]", raw)
	}

	return SourceRef{
		Owner: strings.TrimSpace(parts[0]),
		Repo:  strings.TrimSpace(parts[1]),
		Ref:   ref,
	}, nil
}

// String formats the SourceRef as "owner/repo@ref".
func (s SourceRef) String() string {
	if s.Owner == "" || s.Repo == "" {
		return ""
	}
	ref := s.Ref
	if ref == "" {
		ref = "v1"
	}
	return fmt.Sprintf("%s/%s@%s", s.Owner, s.Repo, ref)
}

// WorkflowUses returns the full GitHub Actions `uses:` path for a reusable workflow file.
// If local is true, it returns "./.github/workflows/<workflowFile>".
func (s SourceRef) WorkflowUses(workflowFile string, local bool) string {
	if local {
		return fmt.Sprintf("./.github/workflows/%s", workflowFile)
	}
	ref := s.Ref
	if ref == "" {
		ref = "v1"
	}
	return fmt.Sprintf("%s/%s/.github/workflows/%s@%s", s.Owner, s.Repo, workflowFile, ref)
}

// StackType enumerates supported project stacks.
type StackType string

const (
	StackNode    StackType = "node"
	StackPython  StackType = "python"
	StackGo      StackType = "go"
	StackRust    StackType = "rust"
	StackGeneric StackType = "generic"
)

// StackConfig describes CI configuration for one detected/configured stack in a repository.
type StackConfig struct {
	Name            string    `yaml:"name,omitempty" json:"name,omitempty"`
	Type            StackType `yaml:"type" json:"type"`
	Directory       string    `yaml:"directory" json:"directory"`
	PackageManager  string    `yaml:"package_manager,omitempty" json:"package_manager,omitempty"`
	RuntimeVersion  string    `yaml:"runtime_version,omitempty" json:"runtime_version,omitempty"`
	VersionFile     string    `yaml:"version_file,omitempty" json:"version_file,omitempty"`
	MonorepoTool    string    `yaml:"monorepo_tool,omitempty" json:"monorepo_tool,omitempty"`
	RunLint         bool      `yaml:"lint" json:"lint"`
	RunFormatCheck  bool      `yaml:"format_check,omitempty" json:"format_check,omitempty"`
	RunTypecheck    bool      `yaml:"typecheck,omitempty" json:"typecheck,omitempty"`
	RunTest         bool      `yaml:"test" json:"test"`
	RunBuild        bool      `yaml:"build" json:"build"`
	UploadCoverage  bool      `yaml:"coverage,omitempty" json:"coverage,omitempty"`
	SetupCommand    string    `yaml:"setup_command,omitempty" json:"setup_command,omitempty"`
	LintCommand     string    `yaml:"lint_command,omitempty" json:"lint_command,omitempty"`
	FormatCommand   string    `yaml:"format_command,omitempty" json:"format_command,omitempty"`
	TypecheckCmd    string    `yaml:"typecheck_command,omitempty" json:"typecheck_command,omitempty"`
	TestCommand     string    `yaml:"test_command,omitempty" json:"test_command,omitempty"`
	BuildCommand    string    `yaml:"build_command,omitempty" json:"build_command,omitempty"`
	DetectedSignals []string  `yaml:"-" json:"detected_signals,omitempty"`
}

// DockerConfig configures container image build, scan, and publish.
type DockerConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Context    string `yaml:"context,omitempty" json:"context,omitempty"`
	Dockerfile string `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`
	Registry   string `yaml:"registry,omitempty" json:"registry,omitempty"`
	ImageName  string `yaml:"image_name,omitempty" json:"image_name,omitempty"`
	Platforms  string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	ScanTrivy  bool   `yaml:"scan_trivy,omitempty" json:"scan_trivy,omitempty"`
}

// DeployConfig configures continuous deployment.
type DeployConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Provider      string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Environment   string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Directory     string `yaml:"directory,omitempty" json:"directory,omitempty"`
	ProjectName   string `yaml:"project_name,omitempty" json:"project_name,omitempty"`
	DistDir       string `yaml:"dist_dir,omitempty" json:"dist_dir,omitempty"`
	AWSRegion     string `yaml:"aws_region,omitempty" json:"aws_region,omitempty"`
	S3Bucket      string `yaml:"s3_bucket,omitempty" json:"s3_bucket,omitempty"`
	BuildCommand  string `yaml:"build_command,omitempty" json:"build_command,omitempty"`
	DeployCommand string `yaml:"deploy_command,omitempty" json:"deploy_command,omitempty"`
}

// ReleaseConfig configures automated semantic versioning, changelogs, and GitHub Releases.
type ReleaseConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Strategy    string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	ReleaseType string `yaml:"release_type,omitempty" json:"release_type,omitempty"`
}

// DependabotConfig configures .github/dependabot.yml generation.
type DependabotConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// Manifest is stored at .github/cicd.yml in each onboarded repository.
// It captures per-project overrides (such as Source) and enables clean `cicd sync`.
type Manifest struct {
	Version       int              `yaml:"version" json:"version"`
	Source        string           `yaml:"source,omitempty" json:"source,omitempty"`
	Mode          WorkflowMode     `yaml:"mode" json:"mode"`
	DefaultBranch string           `yaml:"default_branch" json:"default_branch"`
	Monorepo      bool             `yaml:"monorepo,omitempty" json:"monorepo,omitempty"`
	Stacks        []StackConfig    `yaml:"stacks" json:"stacks"`
	Docker        DockerConfig     `yaml:"docker,omitempty" json:"docker,omitempty"`
	Deploy        DeployConfig     `yaml:"deploy,omitempty" json:"deploy,omitempty"`
	Release       ReleaseConfig    `yaml:"release,omitempty" json:"release,omitempty"`
	Dependabot    DependabotConfig `yaml:"dependabot,omitempty" json:"dependabot,omitempty"`
}

// DetectionResult holds everything automatically discovered in a project directory.
type DetectionResult struct {
	ProjectDir        string        `json:"project_dir"`
	DefaultBranch     string        `json:"default_branch"`
	GitOwner          string        `json:"git_owner,omitempty"`
	GitRepo           string        `json:"git_repo,omitempty"`
	IsMonorepo        bool          `json:"is_monorepo"`
	MonorepoTool      string        `json:"monorepo_tool,omitempty"`
	Stacks            []StackConfig `json:"stacks"`
	Docker            DockerConfig  `json:"docker"`
	Deploy            DeployConfig  `json:"deploy"`
	ExistingWorkflows []string      `json:"existing_workflows,omitempty"`
	HasManifest       bool          `json:"has_manifest"`
}
