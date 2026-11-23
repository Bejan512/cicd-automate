package detector

import (
	"path/filepath"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// RustDetector detects Rust Cargo projects and workspaces.
type RustDetector struct{}

func (RustDetector) Name() string { return "rust" }

func (RustDetector) Detect(rootDir, relDir string) (*model.StackConfig, bool) {
	dir := filepath.Join(rootDir, relDir)
	cargoPath := filepath.Join(dir, "Cargo.toml")
	if !fileExists(cargoPath) {
		return nil, false
	}

	signals := []string{"Cargo.toml"}
	toolchain := "stable"
	if fileExists(filepath.Join(dir, "rust-toolchain.toml")) {
		signals = append(signals, "rust-toolchain.toml")
	} else if fileExists(filepath.Join(dir, "rust-toolchain")) {
		signals = append(signals, "rust-toolchain")
	}

	stack := &model.StackConfig{
		Type:            model.StackRust,
		Directory:       relDir,
		RuntimeVersion:  toolchain,
		RunLint:         true, // clippy
		RunFormatCheck:  true, // cargo fmt --check
		RunTest:         true,
		RunBuild:        true,
		DetectedSignals: signals,
	}
	if hasCargoWorkspace(cargoPath) {
		stack.MonorepoTool = "cargo-workspace"
		stack.DetectedSignals = append(stack.DetectedSignals, "cargo-workspace")
	}
	return stack, true
}

func hasCargoWorkspace(cargoPath string) bool {
	if !fileExists(cargoPath) {
		return false
	}
	return strings.Contains(readFileText(cargoPath), "[workspace]")
}
