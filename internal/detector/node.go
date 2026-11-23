package detector

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// NodeDetector detects Node.js / TypeScript / Bun projects.
type NodeDetector struct{}

func (NodeDetector) Name() string { return "node" }

type packageJSON struct {
	Name            string            `json:"name"`
	PackageManager  string            `json:"packageManager"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      any               `json:"workspaces"`
}

func (NodeDetector) Detect(rootDir, relDir string) (*model.StackConfig, bool) {
	dir := filepath.Join(rootDir, relDir)
	pkgPath := filepath.Join(dir, "package.json")
	if !fileExists(pkgPath) {
		return nil, false
	}

	raw := readFileText(pkgPath)
	var pkg packageJSON
	_ = json.Unmarshal([]byte(raw), &pkg)

	pm := detectNodePackageManager(dir, pkg.PackageManager)
	signals := []string{"package.json", "pm:" + pm}

	stack := &model.StackConfig{
		Type:           model.StackNode,
		Directory:      relDir,
		PackageManager: pm,
		RuntimeVersion: "22",
	}

	// Detect version file (.nvmrc or .node-version).
	for _, vf := range []string{".nvmrc", ".node-version"} {
		if fileExists(filepath.Join(dir, vf)) {
			if relDir == "." {
				stack.VersionFile = vf
			} else {
				stack.VersionFile = filepath.ToSlash(filepath.Join(relDir, vf))
			}
			signals = append(signals, vf)
			break
		}
	}

	// Inspect scripts in package.json.
	if hasNonPlaceholderScript(pkg.Scripts, "lint") {
		stack.RunLint = true
		signals = append(signals, "script:lint")
	}

	if hasNonPlaceholderScript(pkg.Scripts, "typecheck") {
		stack.RunTypecheck = true
		signals = append(signals, "script:typecheck")
	} else if hasNonPlaceholderScript(pkg.Scripts, "type-check") {
		stack.RunTypecheck = true
		stack.TypecheckCmd = pm + " run type-check"
		signals = append(signals, "script:type-check")
	} else if fileExists(filepath.Join(dir, "tsconfig.json")) {
		stack.RunTypecheck = true
		execPrefix := "npx"
		switch pm {
		case "pnpm":
			execPrefix = "pnpm exec"
		case "yarn":
			execPrefix = "yarn"
		case "bun":
			execPrefix = "bunx"
		}
		if _, isNext := pkg.Dependencies["next"]; isNext {
			// Next.js 15+/16 generates LayoutProps/PageProps in .next/types via `next typegen`.
			stack.TypecheckCmd = "(" + execPrefix + " next typegen 2>/dev/null || true) && " + execPrefix + " tsc --noEmit"
			signals = append(signals, "next")
		} else {
			stack.TypecheckCmd = execPrefix + " tsc --noEmit"
		}
		signals = append(signals, "tsconfig.json")
	}

	if hasNonPlaceholderScript(pkg.Scripts, "test") {
		stack.RunTest = true
		signals = append(signals, "script:test")
	}

	if hasNonPlaceholderScript(pkg.Scripts, "build") {
		stack.RunBuild = true
		signals = append(signals, "script:build")
	}

	// Enable coverage hint if vitest/jest/c8 is present.
	for _, dep := range []string{"vitest", "jest", "c8", "@vitest/coverage-v8"} {
		if _, ok := pkg.DevDependencies[dep]; ok {
			signals = append(signals, dep)
			break
		}
	}

	stack.DetectedSignals = signals
	return stack, true
}

func detectNodePackageManager(dir, pkgManagerField string) string {
	switch {
	case fileExists(filepath.Join(dir, "bun.lockb")) || fileExists(filepath.Join(dir, "bun.lock")):
		return "bun"
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")) || fileExists(filepath.Join(dir, "pnpm-workspace.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(dir, "package-lock.json")):
		return "npm"
	}

	pmField := strings.ToLower(strings.TrimSpace(pkgManagerField))
	switch {
	case strings.HasPrefix(pmField, "bun"):
		return "bun"
	case strings.HasPrefix(pmField, "pnpm"):
		return "pnpm"
	case strings.HasPrefix(pmField, "yarn"):
		return "yarn"
	default:
		return "npm"
	}
}

func hasNonPlaceholderScript(scripts map[string]string, name string) bool {
	val, ok := scripts[name]
	if !ok {
		return false
	}
	v := strings.TrimSpace(val)
	if v == "" || strings.Contains(v, "no test specified") {
		return false
	}
	return true
}

func hasPackageJSONWorkspaces(pkgPath string) bool {
	if !fileExists(pkgPath) {
		return false
	}
	var pkg packageJSON
	if err := json.Unmarshal([]byte(readFileText(pkgPath)), &pkg); err != nil {
		return false
	}
	return pkg.Workspaces != nil
}
