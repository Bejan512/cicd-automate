package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorSelfRepoWorkflows(t *testing.T) {
	// Validate the repository's own .github/workflows/*.yml files!
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	report, err := Run(repoRoot, Options{})
	if err != nil {
		t.Fatalf("doctor.Run error: %v", err)
	}
	if report.WorkflowCount < 9 {
		t.Fatalf("expected at least 9 workflows in repo root, found %d", report.WorkflowCount)
	}
	if report.HasErrors() {
		t.Fatalf("repo workflows have errors: %+v", report.Findings)
	}
	if report.HasWarnings() {
		t.Fatalf("repo workflows have warnings: %+v", report.Findings)
	}
}

func TestDoctorDetectsInvalidYAMLAndDrift(t *testing.T) {
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Bad YAML workflow
	if err := os.WriteFile(filepath.Join(wfDir, "broken.yml"), []byte("name: broken\non: [push\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Workflow with unpinned ref and custom secret
	if err := os.WriteFile(filepath.Join(wfDir, "deploy.yml"), []byte(`name: Deploy
on: [push]
jobs:
  dep:
    runs-on: ubuntu-latest
    steps:
      - uses: someorg/action@main
      - run: echo ${{ secrets.CUSTOM_API_KEY }}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Manifest without Docker, while Dockerfile exists -> drift warning!
	if err := os.WriteFile(filepath.Join(dir, ".github", "cicd.yml"), []byte("version: 1\nmode: reusable\ndefault_branch: main\nstacks: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(dir, Options{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !report.HasErrors() {
		t.Fatal("expected error for broken.yml")
	}
	if !report.HasWarnings() {
		t.Fatal("expected warnings for @main ref and Dockerfile drift")
	}
	if len(report.RequiredSecrets) != 1 || report.RequiredSecrets[0] != "CUSTOM_API_KEY" {
		t.Fatalf("expected CUSTOM_API_KEY in RequiredSecrets, got %+v", report.RequiredSecrets)
	}
}
