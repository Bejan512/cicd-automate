package detector

import (
	"path/filepath"
	"regexp"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// GenericDetector provides a fallback for any stack using a Makefile or justfile.
type GenericDetector struct{}

func (GenericDetector) Name() string { return "generic" }

var targetRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9_-]+)\s*:`)

func (GenericDetector) Detect(rootDir, relDir string) (*model.StackConfig, bool) {
	dir := filepath.Join(rootDir, relDir)

	var runner string
	var text string
	var signal string

	switch {
	case fileExists(filepath.Join(dir, "Makefile")):
		runner = "make"
		text = readFileText(filepath.Join(dir, "Makefile"))
		signal = "Makefile"
	case fileExists(filepath.Join(dir, "justfile")):
		runner = "just"
		text = readFileText(filepath.Join(dir, "justfile"))
		signal = "justfile"
	default:
		return nil, false
	}

	targets := map[string]bool{}
	for _, m := range targetRe.FindAllStringSubmatch(text, -1) {
		if len(m) == 2 {
			targets[m[1]] = true
		}
	}

	stack := &model.StackConfig{
		Type:            model.StackGeneric,
		Directory:       relDir,
		DetectedSignals: []string{signal},
	}

	if targets["lint"] {
		stack.RunLint = true
		stack.LintCommand = runner + " lint"
	} else if targets["check"] {
		stack.RunLint = true
		stack.LintCommand = runner + " check"
	}

	if targets["test"] {
		stack.RunTest = true
		stack.TestCommand = runner + " test"
	}

	if targets["build"] {
		stack.RunBuild = true
		stack.BuildCommand = runner + " build"
	}

	return stack, true
}
