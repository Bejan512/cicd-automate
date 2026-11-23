package detector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

func writeTestFile(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func TestDetectNodeProject(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{
  "name": "web-app",
  "scripts": { "lint": "eslint .", "test": "vitest run", "build": "vite build" },
  "devDependencies": { "vitest": "^2.0.0" }
}`)
	writeTestFile(t, dir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	writeTestFile(t, dir, "tsconfig.json", "{}")
	writeTestFile(t, dir, ".nvmrc", "22\n")
	writeTestFile(t, dir, "vercel.json", "{}")

	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("DetectProject error: %v", err)
	}
	if len(res.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d: %+v", len(res.Stacks), res.Stacks)
	}
	s := res.Stacks[0]
	if s.Type != model.StackNode || s.PackageManager != "pnpm" || !s.RunLint || !s.RunTypecheck || !s.RunTest || !s.RunBuild {
		t.Fatalf("unexpected node stack: %+v", s)
	}
	if s.VersionFile != ".nvmrc" {
		t.Fatalf("expected .nvmrc, got %q", s.VersionFile)
	}
	if !res.Deploy.Enabled || res.Deploy.Provider != "vercel" {
		t.Fatalf("expected vercel deploy, got %+v", res.Deploy)
	}
}

func TestDetectPythonProject(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pyproject.toml", `
[project]
name = "ml-service"
[tool.uv]
dev-dependencies = ["pytest", "ruff", "mypy"]
`)
	writeTestFile(t, dir, "uv.lock", "")
	writeTestFile(t, dir, "Dockerfile", "FROM python:3.12-slim\n")
	writeTestFile(t, dir, "fly.toml", "app = 'ml-service'\n")

	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("DetectProject error: %v", err)
	}
	if len(res.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(res.Stacks))
	}
	s := res.Stacks[0]
	if s.Type != model.StackPython || s.PackageManager != "uv" || !s.RunLint || !s.RunFormatCheck || !s.RunTypecheck || !s.RunTest {
		t.Fatalf("unexpected python stack: %+v", s)
	}
	if !res.Docker.Enabled || res.Docker.Dockerfile != "Dockerfile" {
		t.Fatalf("expected Docker detected, got %+v", res.Docker)
	}
	if !res.Deploy.Enabled || res.Deploy.Provider != "fly" {
		t.Fatalf("expected fly deploy, got %+v", res.Deploy)
	}
}

func TestDetectPolyglotMonorepo(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "frontend/package.json", `{"name":"web","scripts":{"build":"next build","test":"jest"}}`)
	writeTestFile(t, dir, "services/api/go.mod", "module example.com/api\n\ngo 1.24\n")
	writeTestFile(t, dir, "services/worker/Cargo.toml", "[package]\nname = \"worker\"\nversion = \"0.1.0\"\n")

	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("DetectProject error: %v", err)
	}
	if !res.IsMonorepo {
		t.Fatal("expected IsMonorepo to be true")
	}
	if len(res.Stacks) != 3 {
		t.Fatalf("expected 3 stacks (node, go, rust), got %d: %+v", len(res.Stacks), res.Stacks)
	}
}

func TestDetectGenericMakefileFallback(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "Makefile", "lint:\n\techo lint\ntest:\n\techo test\nbuild:\n\techo build\n")

	res, err := DetectProject(dir)
	if err != nil {
		t.Fatalf("DetectProject error: %v", err)
	}
	if len(res.Stacks) != 1 || res.Stacks[0].Type != model.StackGeneric {
		t.Fatalf("expected 1 generic stack, got %+v", res.Stacks)
	}
	s := res.Stacks[0]
	if s.LintCommand != "make lint" || s.TestCommand != "make test" || s.BuildCommand != "make build" {
		t.Fatalf("unexpected make commands: %+v", s)
	}
}
