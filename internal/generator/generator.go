package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// GeneratedFile represents one file to write relative to a project root.
type GeneratedFile struct {
	RelPath string
	Content string
}

// Options controls generation behavior.
type Options struct {
	LocalReusable bool // Use ./.github/workflows/reusable-*.yml (for the cicd-automate repo itself)
}

// Generate renders all enabled workflow files, dependabot.yml, and .github/cicd.yml for manifest.
func Generate(m model.Manifest, src model.SourceRef, opts Options) ([]GeneratedFile, error) {
	if m.DefaultBranch == "" {
		m.DefaultBranch = "main"
	}
	if m.Mode == "" {
		m.Mode = model.ModeReusable
	}
	if m.Version == 0 {
		m.Version = 1
	}

	var files []GeneratedFile

	manifestContent, err := RenderManifest(m)
	if err != nil {
		return nil, fmt.Errorf("render manifest: %w", err)
	}
	files = append(files, GeneratedFile{
		RelPath: ManifestRelPath,
		Content: manifestContent,
	})

	if len(m.Stacks) > 0 {
		var ci string
		if m.Mode == model.ModeStandalone {
			ci = RenderStandaloneCI(m)
		} else {
			ci = RenderReusableCI(m, src, opts.LocalReusable)
		}
		files = append(files, GeneratedFile{
			RelPath: ".github/workflows/ci.yml",
			Content: ci,
		})
	}

	if m.Docker.Enabled {
		var dockerWF string
		if m.Mode == model.ModeStandalone {
			dockerWF = RenderStandaloneDocker(m)
		} else {
			dockerWF = RenderReusableDocker(m, src, opts.LocalReusable)
		}
		files = append(files, GeneratedFile{
			RelPath: ".github/workflows/docker.yml",
			Content: dockerWF,
		})
	}

	if m.Deploy.Enabled && m.Deploy.Provider != "" {
		var deployWF string
		if m.Mode == model.ModeStandalone {
			deployWF = RenderStandaloneDeploy(m)
		} else {
			deployWF = RenderReusableDeploy(m, src, opts.LocalReusable)
		}
		files = append(files, GeneratedFile{
			RelPath: ".github/workflows/deploy.yml",
			Content: deployWF,
		})
	}

	if m.Release.Enabled {
		var releaseWF string
		if m.Mode == model.ModeStandalone {
			releaseWF = RenderStandaloneRelease(m)
		} else {
			releaseWF = RenderReusableRelease(m, src, opts.LocalReusable)
		}
		files = append(files, GeneratedFile{
			RelPath: ".github/workflows/release.yml",
			Content: releaseWF,
		})
	}

	if m.Dependabot.Enabled {
		files = append(files, GeneratedFile{
			RelPath: ".github/dependabot.yml",
			Content: RenderDependabot(m),
		})
	}

	return files, nil
}

// WriteResult summarizes file write operations.
type WriteResult struct {
	Created []string
	Updated []string
	Skipped []string
}

// WriteFiles writes generated files into projectDir.
// If overwrite is false, existing files are skipped unless unchanged.
func WriteFiles(projectDir string, files []GeneratedFile, overwrite bool) (WriteResult, error) {
	var res WriteResult
	for _, f := range files {
		fullPath := filepath.Join(projectDir, filepath.FromSlash(f.RelPath))
		existing, err := os.ReadFile(fullPath)
		exists := err == nil

		if exists && !overwrite {
			res.Skipped = append(res.Skipped, f.RelPath)
			continue
		}
		if exists && string(existing) == f.Content {
			res.Skipped = append(res.Skipped, f.RelPath)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return res, fmt.Errorf("create dir for %s: %w", f.RelPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0o644); err != nil {
			return res, fmt.Errorf("write %s: %w", f.RelPath, err)
		}

		if exists {
			res.Updated = append(res.Updated, f.RelPath)
		} else {
			res.Created = append(res.Created, f.RelPath)
		}
	}
	return res, nil
}
