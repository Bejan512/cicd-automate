package detector

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/gitinfo"
	"github.com/cicd-automate/cicd-automate/internal/model"
)

// StackDetector inspects a single directory and returns a StackConfig if that stack is present.
// Add new stack detectors by implementing this interface and registering them in DefaultDetectors().
type StackDetector interface {
	Name() string
	Detect(rootDir, relDir string) (*model.StackConfig, bool)
}

// DefaultDetectors returns the built-in stack detectors in priority order.
func DefaultDetectors() []StackDetector {
	return []StackDetector{
		NodeDetector{},
		PythonDetector{},
		GoDetector{},
		RustDetector{},
	}
}

var ignoredDirs = map[string]bool{
	".git":         true,
	".github":      true,
	".vscode":      true,
	".idea":        true,
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true,
	".next":        true,
	".nuxt":        true,
	".turbo":       true,
	".cache":       true,
	"coverage":     true,
	"bin":          true,
	"tmp":          true,
	"testdata":     true,
}

var containerDirs = map[string]bool{
	"apps":     true,
	"packages": true,
	"services": true,
	"crates":   true,
	"libs":     true,
}

// DetectProject inspects projectDir and returns all detected stacks, monorepo signals,
// Dockerfiles, deployment configs, and existing workflows.
func DetectProject(projectDir string) (model.DetectionResult, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return model.DetectionResult{}, err
	}

	ginfo := gitinfo.Inspect(absDir)
	res := model.DetectionResult{
		ProjectDir:    absDir,
		DefaultBranch: ginfo.DefaultBranch,
		GitOwner:      ginfo.RemoteOwner,
		GitRepo:       ginfo.RemoteRepo,
	}

	detectors := DefaultDetectors()

	// 1. Check root monorepo signals.
	monoTool := detectMonorepoTool(absDir)
	if monoTool != "" {
		res.IsMonorepo = true
		res.MonorepoTool = monoTool
	}

	// 2. Detect stacks at repository root.
	rootHasNodeWorkspace := false
	for _, d := range detectors {
		if stack, ok := d.Detect(absDir, "."); ok {
			if stack.Type == model.StackNode && monoTool != "" {
				stack.MonorepoTool = monoTool
				rootHasNodeWorkspace = true
			}
			res.Stacks = append(res.Stacks, *stack)
		}
	}

	// 3. Scan subdirectories for multi-service / polyglot repos.
	candidateDirs := findSubServiceDirs(absDir)
	for _, relDir := range candidateDirs {
		for _, d := range detectors {
			stack, ok := d.Detect(absDir, relDir)
			if !ok {
				continue
			}
			// If root is already a Node monorepo workspace (turbo/nx/pnpm-workspace),
			// skip nested Node packages so we don't duplicate jobs managed by the root orchestrator.
			if stack.Type == model.StackNode && rootHasNodeWorkspace {
				continue
			}
			// If root is a Rust Cargo workspace, skip nested Cargo crates.
			if stack.Type == model.StackRust && hasRootStack(res.Stacks, model.StackRust) && monoTool == "cargo-workspace" {
				continue
			}
			res.Stacks = append(res.Stacks, *stack)
		}
	}

	if len(res.Stacks) > 1 {
		res.IsMonorepo = true
	}

	// 4. Generic fallback (Makefile / justfile) if no specialized stack was found.
	if len(res.Stacks) == 0 {
		if generic, ok := (GenericDetector{}).Detect(absDir, "."); ok {
			res.Stacks = append(res.Stacks, *generic)
		}
	}

	// Assign stable, unique names to each stack.
	assignStackNames(res.Stacks)

	// 5. Detect Docker & Deploy configurations.
	res.Docker = DetectDocker(absDir)
	res.Deploy = DetectDeploy(absDir)

	// 6. Inspect existing .github/workflows and manifest.
	res.ExistingWorkflows = listExistingWorkflows(absDir)
	res.HasManifest = fileExists(filepath.Join(absDir, ".github", "cicd.yml"))

	return res, nil
}

func detectMonorepoTool(rootDir string) string {
	switch {
	case fileExists(filepath.Join(rootDir, "turbo.json")):
		return "turborepo"
	case fileExists(filepath.Join(rootDir, "nx.json")):
		return "nx"
	case fileExists(filepath.Join(rootDir, "pnpm-workspace.yaml")):
		return "pnpm-workspaces"
	case fileExists(filepath.Join(rootDir, "go.work")):
		return "go-workspace"
	case hasCargoWorkspace(filepath.Join(rootDir, "Cargo.toml")):
		return "cargo-workspace"
	case hasPackageJSONWorkspaces(filepath.Join(rootDir, "package.json")):
		return "npm-workspaces"
	default:
		return ""
	}
}

func findSubServiceDirs(rootDir string) []string {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || ignoredDirs[name] {
			continue
		}

		dirs = append(dirs, name)

		// Also inspect one level inside standard container directories like apps/, services/, packages/.
		if containerDirs[name] {
			subEntries, err := os.ReadDir(filepath.Join(rootDir, name))
			if err == nil {
				for _, sub := range subEntries {
					if !sub.IsDir() || strings.HasPrefix(sub.Name(), ".") || ignoredDirs[sub.Name()] {
						continue
					}
					dirs = append(dirs, filepath.ToSlash(filepath.Join(name, sub.Name())))
				}
			}
		}
	}

	sort.Strings(dirs)
	return dirs
}

func hasRootStack(stacks []model.StackConfig, st model.StackType) bool {
	for _, s := range stacks {
		if s.Directory == "." && s.Type == st {
			return true
		}
	}
	return false
}

func assignStackNames(stacks []model.StackConfig) {
	used := map[string]int{}
	for i := range stacks {
		base := string(stacks[i].Type)
		if stacks[i].Directory != "." && stacks[i].Directory != "" {
			clean := strings.NewReplacer("/", "-", "_", "-", ".", "-").Replace(stacks[i].Directory)
			clean = strings.Trim(clean, "-")
			base = clean + "-" + string(stacks[i].Type)
		}
		used[base]++
		if used[base] > 1 {
			stacks[i].Name = base + "-" + string(rune('0'+used[base]))
		} else {
			stacks[i].Name = base
		}
	}
}

func listExistingWorkflows(rootDir string) []string {
	wfDir := filepath.Join(rootDir, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func readFileText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
