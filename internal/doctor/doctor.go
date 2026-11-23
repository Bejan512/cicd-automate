package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/detector"
	"github.com/cicd-automate/cicd-automate/internal/generator"
	"github.com/cicd-automate/cicd-automate/internal/model"
	"gopkg.in/yaml.v3"
)

// Severity classifies a diagnostic finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Finding describes a single check result from doctor.
type Finding struct {
	Severity Severity `json:"severity"`
	File     string   `json:"file,omitempty"`
	Message  string   `json:"message"`
}

// Report aggregates all findings for a repository.
type Report struct {
	ProjectDir      string    `json:"project_dir"`
	WorkflowCount   int       `json:"workflow_count"`
	HasManifest     bool      `json:"has_manifest"`
	RequiredSecrets []string  `json:"required_secrets,omitempty"`
	Findings        []Finding `json:"findings"`
}

// HasErrors returns true if any finding has SeverityError.
func (r Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any finding has SeverityWarning.
func (r Report) HasWarnings() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// Options configures doctor execution.
type Options struct {
	CheckSecrets bool // Query GitHub repo secrets via `gh secret list`
}

var secretRefRe = regexp.MustCompile(`\$\{\{\s*secrets\.([A-Za-z0-9_]+)\s*\}\}`)

type workflowDoc struct {
	Name string                  `yaml:"name"`
	On   any                     `yaml:"on"`
	Jobs map[string]workflowJob  `yaml:"jobs"`
}

type workflowJob struct {
	Uses  string         `yaml:"uses"`
	Steps []workflowStep `yaml:"steps"`
}

type workflowStep struct {
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
}

// Run audits a repository's CI/CD setup, workflow YAML validity, action refs, drift, and secrets.
func Run(projectDir string, opts Options) (Report, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return Report{}, err
	}

	report := Report{ProjectDir: absDir}
	wfDir := filepath.Join(absDir, ".github", "workflows")

	entries, err := os.ReadDir(wfDir)
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityWarning,
				Message:  "No .github/workflows directory found. Run `cicd init` to set up CI/CD.",
			})
		} else {
			return report, err
		}
	}

	secretSet := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		report.WorkflowCount++
		relPath := filepath.ToSlash(filepath.Join(".github", "workflows", name))
		rawBytes, readErr := os.ReadFile(filepath.Join(wfDir, name))
		if readErr != nil {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				File:     relPath,
				Message:  fmt.Sprintf("cannot read file: %v", readErr),
			})
			continue
		}
		raw := string(rawBytes)

		var doc workflowDoc
		if yamlErr := yaml.Unmarshal(rawBytes, &doc); yamlErr != nil {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				File:     relPath,
				Message:  fmt.Sprintf("invalid YAML: %v", yamlErr),
			})
			continue
		}

		if doc.On == nil {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				File:     relPath,
				Message:  "workflow is missing required `on:` trigger",
			})
		}
		if len(doc.Jobs) == 0 {
			report.Findings = append(report.Findings, Finding{
				Severity: SeverityError,
				File:     relPath,
				Message:  "workflow defines no jobs",
			})
		}

		for jobName, job := range doc.Jobs {
			if job.Uses == "" && len(job.Steps) == 0 {
				report.Findings = append(report.Findings, Finding{
					Severity: SeverityError,
					File:     relPath,
					Message:  fmt.Sprintf("job %q has neither `uses:` nor `steps:`", jobName),
				})
			}
			if job.Uses != "" {
				checkUsesRef(relPath, "job "+jobName, job.Uses, &report.Findings)
			}
			for _, step := range job.Steps {
				if step.Uses != "" {
					checkUsesRef(relPath, "step", step.Uses, &report.Findings)
				}
			}
		}

		// Collect referenced secrets (excluding built-in GITHUB_TOKEN and reusable workflow_call definitions).
		if !isWorkflowCallOnly(doc.On) {
			for _, m := range secretRefRe.FindAllStringSubmatch(raw, -1) {
				if len(m) == 2 && m[1] != "GITHUB_TOKEN" {
					secretSet[m[1]] = true
				}
			}
		}
	}

	for sec := range secretSet {
		report.RequiredSecrets = append(report.RequiredSecrets, sec)
	}
	sort.Strings(report.RequiredSecrets)

	// Check manifest & drift against live project detection.
	manifest, err := generator.LoadManifest(absDir)
	if err == nil {
		report.HasManifest = true
		det, detErr := detector.DetectProject(absDir)
		if detErr == nil {
			checkManifestDrift(manifest, det, &report.Findings)
		}
	} else if !os.IsNotExist(err) {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityError,
			File:     generator.ManifestRelPath,
			Message:  fmt.Sprintf("invalid manifest: %v", err),
		})
	} else if report.WorkflowCount > 0 {
		report.Findings = append(report.Findings, Finding{
			Severity: SeverityInfo,
			Message:  "No .github/cicd.yml manifest found (existing workflows were not generated by cicd-automate).",
		})
	}

	// Optional GitHub secret check via `gh secret list`.
	if opts.CheckSecrets && len(report.RequiredSecrets) > 0 {
		checkGitHubSecrets(absDir, report.RequiredSecrets, &report.Findings)
	}

	return report, nil
}

func isWorkflowCallOnly(onField any) bool {
	switch v := onField.(type) {
	case string:
		return v == "workflow_call"
	case map[string]any:
		_, hasCall := v["workflow_call"]
		return hasCall && len(v) == 1
	default:
		return false
	}
}

func checkUsesRef(file, ctx, uses string, findings *[]Finding) {
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
		return
	}
	parts := strings.SplitN(uses, "@", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		*findings = append(*findings, Finding{
			Severity: SeverityError,
			File:     file,
			Message:  fmt.Sprintf("%s uses %q without a version ref (@vX or SHA)", ctx, uses),
		})
		return
	}
	ref := parts[1]
	// dtolnay/rust-toolchain and superfly/flyctl-actions intentionally track master as their official release ref.
	if (ref == "main" || ref == "master") &&
		!strings.HasPrefix(uses, "dtolnay/rust-toolchain@") &&
		!strings.HasPrefix(uses, "superfly/flyctl-actions/") {
		*findings = append(*findings, Finding{
			Severity: SeverityWarning,
			File:     file,
			Message:  fmt.Sprintf("%s uses mutable branch ref %q; prefer a version tag (e.g. @v1) or commit SHA", ctx, uses),
		})
	}
}

func checkManifestDrift(manifest model.Manifest, det model.DetectionResult, findings *[]Finding) {
	if det.Docker.Enabled && !manifest.Docker.Enabled {
		*findings = append(*findings, Finding{
			Severity: SeverityWarning,
			File:     generator.ManifestRelPath,
			Message:  fmt.Sprintf("Dockerfile detected (%s) but docker workflow is disabled — run `cicd sync --redetect` to enable it", det.Docker.Dockerfile),
		})
	}
	if det.Deploy.Enabled && !manifest.Deploy.Enabled {
		*findings = append(*findings, Finding{
			Severity: SeverityWarning,
			File:     generator.ManifestRelPath,
			Message:  fmt.Sprintf("Deploy config detected (%s) but deploy workflow is disabled — run `cicd sync --redetect` to enable it", det.Deploy.Provider),
		})
	}

	existingDirs := map[string]bool{}
	for _, s := range manifest.Stacks {
		existingDirs[string(s.Type)+":"+s.Directory] = true
	}
	for _, s := range det.Stacks {
		key := string(s.Type) + ":" + s.Directory
		if !existingDirs[key] {
			*findings = append(*findings, Finding{
				Severity: SeverityWarning,
				File:     generator.ManifestRelPath,
				Message:  fmt.Sprintf("New stack detected (%s in %s) not in .github/cicd.yml — run `cicd sync --redetect`", s.Type, s.Directory),
			})
		}
	}
}

func checkGitHubSecrets(dir string, required []string, findings *[]Finding) {
	if _, err := exec.LookPath("gh"); err != nil {
		*findings = append(*findings, Finding{
			Severity: SeverityWarning,
			Message:  "Cannot verify GitHub secrets: `gh` CLI is not installed",
		})
		return
	}
	cmd := exec.Command("gh", "secret", "list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		*findings = append(*findings, Finding{
			Severity: SeverityWarning,
			Message:  "Cannot list GitHub secrets (ensure repo has a GitHub remote and `gh auth login` is active)",
		})
		return
	}
	present := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			present[fields[0]] = true
		}
	}
	for _, req := range required {
		if !present[req] {
			*findings = append(*findings, Finding{
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("Secret %q is referenced in workflows but not set in GitHub repository secrets", req),
			})
		}
	}
}
