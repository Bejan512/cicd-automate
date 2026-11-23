package detector

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// GoDetector detects Go modules.
type GoDetector struct{}

func (GoDetector) Name() string { return "go" }

var goVersionRe = regexp.MustCompile(`(?m)^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)

func (GoDetector) Detect(rootDir, relDir string) (*model.StackConfig, bool) {
	dir := filepath.Join(rootDir, relDir)
	goModPath := filepath.Join(dir, "go.mod")
	if !fileExists(goModPath) {
		return nil, false
	}

	signals := []string{"go.mod"}
	goModText := readFileText(goModPath)
	goVer := ""
	if m := goVersionRe.FindStringSubmatch(goModText); len(m) == 2 {
		goVer = m[1]
		signals = append(signals, "go:"+goVer)
	}

	versionFile := "go.mod"
	if relDir != "." {
		versionFile = filepath.ToSlash(filepath.Join(relDir, "go.mod"))
	}

	hasLintCfg := fileExists(filepath.Join(dir, ".golangci.yml")) ||
		fileExists(filepath.Join(dir, ".golangci.yaml")) ||
		fileExists(filepath.Join(dir, ".golangci.toml"))

	if hasLintCfg {
		signals = append(signals, ".golangci.yml")
	}

	stack := &model.StackConfig{
		Type:            model.StackGo,
		Directory:       relDir,
		RuntimeVersion:  goVer,
		VersionFile:     versionFile,
		RunLint:         hasLintCfg,
		RunTest:         true,
		RunBuild:        true,
		BuildCommand:    "go build ./...",
		DetectedSignals: signals,
	}

	// If cmd/<name> exists, note it in signals.
	if entries, err := filepath.Glob(filepath.Join(dir, "cmd", "*", "main.go")); err == nil && len(entries) > 0 {
		signals = append(signals, "cmd-binary")
		stack.DetectedSignals = signals
	}

	_ = strings.TrimSpace(goModText)
	return stack, true
}
