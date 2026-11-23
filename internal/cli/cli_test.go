package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestEndToEndUserConfigInitSyncDoctorAndBulk(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "user-config.yaml")
	t.Setenv("CICD_CONFIG", cfgPath)
	t.Setenv("CICD_SOURCE", "")

	// 1. Configure per-user default owner.
	out, err := execCLI(t, "config", "set", "owner", "jane-dev")
	if err != nil {
		t.Fatalf("config set owner failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "jane-dev/cicd-automate@v1") {
		t.Fatalf("expected effective source in output, got:\n%s", out)
	}

	// 2. Create a workspace with two projects: Go repo and Node+Docker repo.
	projectsDir := filepath.Join(tmp, "projects")
	goProj := filepath.Join(projectsDir, "my-go-svc")
	nodeProj := filepath.Join(projectsDir, "my-web-app")
	if err := os.MkdirAll(goProj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nodeProj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goProj, "go.mod"), []byte("module example.com/svc\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeProj, "package.json"), []byte(`{"name":"web","scripts":{"lint":"eslint .","test":"vitest","build":"vite build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeProj, "Dockerfile"), []byte("FROM node:22-alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Init goProj using per-user default owner.
	out, err = execCLI(t, "init", "--yes", goProj)
	if err != nil {
		t.Fatalf("init goProj failed: %v\n%s", err, out)
	}
	goCI, err := os.ReadFile(filepath.Join(goProj, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read go ci.yml: %v", err)
	}
	if !strings.Contains(string(goCI), "jane-dev/cicd-automate/.github/workflows/reusable-go.yml@v1") {
		t.Fatalf("expected per-user owner in go ci.yml, got:\n%s", string(goCI))
	}

	// 4. Init nodeProj with a PER-PROJECT owner/source override!
	out, err = execCLI(t, "init", "--yes", "--source", "acme-corp/shared-ci@v1", nodeProj)
	if err != nil {
		t.Fatalf("init nodeProj with override failed: %v\n%s", err, out)
	}
	nodeCI, err := os.ReadFile(filepath.Join(nodeProj, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read node ci.yml: %v", err)
	}
	if !strings.Contains(string(nodeCI), "acme-corp/shared-ci/.github/workflows/reusable-node.yml@v1") {
		t.Fatalf("expected per-project override in node ci.yml, got:\n%s", string(nodeCI))
	}

	// 5. Sync nodeProj with --ref v2: preserves its per-project acme-corp/shared-ci owner while upgrading ref!
	out, err = execCLI(t, "sync", "--ref", "v2", nodeProj)
	if err != nil {
		t.Fatalf("sync nodeProj failed: %v\n%s", err, out)
	}
	nodeCIv2, _ := os.ReadFile(filepath.Join(nodeProj, ".github", "workflows", "ci.yml"))
	if !strings.Contains(string(nodeCIv2), "acme-corp/shared-ci/.github/workflows/reusable-node.yml@v2") {
		t.Fatalf("expected upgraded @v2 with preserved per-project source, got:\n%s", string(nodeCIv2))
	}

	// 6. Run doctor --strict on both projects.
	for _, p := range []string{goProj, nodeProj} {
		out, err = execCLI(t, "doctor", "--strict", p)
		if err != nil {
			t.Fatalf("doctor --strict failed for %s: %v\n%s", p, err, out)
		}
	}

	// 7. Add a third project with no CI and test `cicd bulk --init` + `cicd bulk --sync`.
	pyProj := filepath.Join(projectsDir, "my-py-tool")
	if err := os.MkdirAll(pyProj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pyProj, "pyproject.toml"), []byte("[project]\nname='tool'\n[tool.uv]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err = execCLI(t, "bulk", "--init", projectsDir)
	if err != nil {
		t.Fatalf("bulk --init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Initialized 1 project(s)") {
		t.Fatalf("expected 1 initialized project in bulk output, got:\n%s", out)
	}
	pyCI, err := os.ReadFile(filepath.Join(pyProj, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("expected pyProj ci.yml created by bulk --init: %v", err)
	}
	if !strings.Contains(string(pyCI), "jane-dev/cicd-automate/.github/workflows/reusable-python.yml@v1") {
		t.Fatalf("unexpected pyProj ci.yml:\n%s", string(pyCI))
	}
}
