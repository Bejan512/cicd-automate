package generator

import (
	"strings"
	"testing"

	"github.com/cicd-automate/cicd-automate/internal/model"
	"gopkg.in/yaml.v3"
)

func assertValidYAML(t *testing.T, name, content string) {
	t.Helper()
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("%s produced invalid YAML: %v\n---\n%s", name, err, content)
	}
}

func TestGenerateReusableMonorepoFullPipeline(t *testing.T) {
	m := model.Manifest{
		Version:       1,
		Source:        "acme/cicd-automate@v1",
		Mode:          model.ModeReusable,
		DefaultBranch: "main",
		Monorepo:      true,
		Stacks: []model.StackConfig{
			{Name: "web-node", Type: model.StackNode, Directory: "web", PackageManager: "pnpm", RunLint: true, RunTypecheck: true, RunTest: true, RunBuild: true},
			{Name: "api-go", Type: model.StackGo, Directory: "api", VersionFile: "api/go.mod", RunTest: true, RunBuild: true},
			{Name: "ml-python", Type: model.StackPython, Directory: "ml", PackageManager: "uv", RunLint: true, RunTest: true},
		},
		Docker:     model.DockerConfig{Enabled: true, Context: ".", Dockerfile: "Dockerfile", Registry: "ghcr.io", ScanTrivy: true},
		Deploy:     model.DeployConfig{Enabled: true, Provider: "vercel", Environment: "production", Directory: "web"},
		Release:    model.ReleaseConfig{Enabled: true, Strategy: "release-please"},
		Dependabot: model.DependabotConfig{Enabled: true, Interval: "weekly"},
	}

	src, err := model.ParseSourceRef(m.Source)
	if err != nil {
		t.Fatalf("ParseSourceRef: %v", err)
	}

	files, err := Generate(m, src, Options{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(files) != 6 {
		t.Fatalf("expected 6 generated files (manifest, ci, docker, deploy, release, dependabot), got %d", len(files))
	}

	for _, f := range files {
		assertValidYAML(t, f.RelPath, f.Content)
		if f.RelPath == ".github/workflows/ci.yml" {
			if !strings.Contains(f.Content, "dorny/paths-filter@v3") {
				t.Errorf("expected path filter in monorepo ci.yml")
			}
			if !strings.Contains(f.Content, "acme/cicd-automate/.github/workflows/reusable-node.yml@v1") {
				t.Errorf("expected reusable-node.yml ref in ci.yml")
			}
			if !strings.Contains(f.Content, "acme/cicd-automate/.github/workflows/reusable-go.yml@v1") {
				t.Errorf("expected reusable-go.yml ref in ci.yml")
			}
		}
	}
}

func TestGenerateStandaloneMode(t *testing.T) {
	m := model.Manifest{
		Version:       1,
		Mode:          model.ModeStandalone,
		DefaultBranch: "main",
		Stacks: []model.StackConfig{
			{Name: "rust", Type: model.StackRust, Directory: ".", RuntimeVersion: "stable", RunFormatCheck: true, RunLint: true, RunTest: true, RunBuild: true},
		},
		Docker:  model.DockerConfig{Enabled: true, Registry: "docker.io"},
		Deploy:  model.DeployConfig{Enabled: true, Provider: "fly"},
		Release: model.ReleaseConfig{Enabled: true, Strategy: "tag-release"},
	}

	files, err := Generate(m, model.SourceRef{}, Options{})
	if err != nil {
		t.Fatalf("Generate standalone error: %v", err)
	}

	for _, f := range files {
		assertValidYAML(t, f.RelPath, f.Content)
		if strings.Contains(f.Content, "reusable-") {
			t.Errorf("standalone file %s should not reference reusable workflows", f.RelPath)
		}
	}
}
