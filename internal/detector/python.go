package detector

import (
	"path/filepath"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// PythonDetector detects Python projects using uv, poetry, or pip.
type PythonDetector struct{}

func (PythonDetector) Name() string { return "python" }

func (PythonDetector) Detect(rootDir, relDir string) (*model.StackConfig, bool) {
	dir := filepath.Join(rootDir, relDir)
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	reqPath := filepath.Join(dir, "requirements.txt")
	setupPyPath := filepath.Join(dir, "setup.py")

	hasPyproject := fileExists(pyprojectPath)
	hasReq := fileExists(reqPath)
	hasSetupPy := fileExists(setupPyPath)
	if !hasPyproject && !hasReq && !hasSetupPy {
		return nil, false
	}

	pyprojectText := strings.ToLower(readFileText(pyprojectPath))
	reqText := strings.ToLower(readFileText(reqPath) + "\n" + readFileText(filepath.Join(dir, "requirements-dev.txt")))
	allDeps := pyprojectText + "\n" + reqText

	pm := "pip"
	signals := []string{}
	switch {
	case fileExists(filepath.Join(dir, "uv.lock")) || strings.Contains(pyprojectText, "[tool.uv"):
		pm = "uv"
		signals = append(signals, "uv")
	case fileExists(filepath.Join(dir, "poetry.lock")) || strings.Contains(pyprojectText, "[tool.poetry"):
		pm = "poetry"
		signals = append(signals, "poetry")
	default:
		if hasPyproject {
			signals = append(signals, "pyproject.toml")
		}
		if hasReq {
			signals = append(signals, "requirements.txt")
		}
	}

	stack := &model.StackConfig{
		Type:           model.StackPython,
		Directory:      relDir,
		PackageManager: pm,
		RuntimeVersion: "3.12",
		RunTest:        true,
		TestCommand:    "pytest",
	}

	if fileExists(filepath.Join(dir, ".python-version")) {
		if relDir == "." {
			stack.VersionFile = ".python-version"
		} else {
			stack.VersionFile = filepath.ToSlash(filepath.Join(relDir, ".python-version"))
		}
		signals = append(signals, ".python-version")
	}

	// Detect linters / formatters / typecheckers.
	if strings.Contains(allDeps, "ruff") || fileExists(filepath.Join(dir, "ruff.toml")) || fileExists(filepath.Join(dir, ".ruff.toml")) {
		stack.RunLint = true
		stack.LintCommand = "ruff check ."
		stack.RunFormatCheck = true
		stack.FormatCommand = "ruff format --check ."
		signals = append(signals, "ruff")
	} else if strings.Contains(allDeps, "flake8") {
		stack.RunLint = true
		stack.LintCommand = "flake8 ."
		signals = append(signals, "flake8")
	}

	if strings.Contains(allDeps, "mypy") || fileExists(filepath.Join(dir, "mypy.ini")) {
		stack.RunTypecheck = true
		stack.TypecheckCmd = "mypy ."
		signals = append(signals, "mypy")
	} else if strings.Contains(allDeps, "pyright") {
		stack.RunTypecheck = true
		stack.TypecheckCmd = "pyright"
		signals = append(signals, "pyright")
	}

	if strings.Contains(allDeps, "pytest") || fileExists(filepath.Join(dir, "pytest.ini")) {
		stack.RunTest = true
		stack.TestCommand = "pytest"
		signals = append(signals, "pytest")
	}

	stack.DetectedSignals = signals
	return stack, true
}
