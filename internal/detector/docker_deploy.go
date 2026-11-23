package detector

import (
	"path/filepath"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// DetectDocker inspects the repository for a Dockerfile.
func DetectDocker(rootDir string) model.DockerConfig {
	candidates := []string{
		"Dockerfile",
		"docker/Dockerfile",
	}
	for _, rel := range candidates {
		if fileExists(filepath.Join(rootDir, rel)) {
			return model.DockerConfig{
				Enabled:    true,
				Context:    ".",
				Dockerfile: rel,
				Registry:   "ghcr.io",
				Platforms:  "linux/amd64,linux/arm64",
				ScanTrivy:  true,
			}
		}
	}
	return model.DockerConfig{}
}

// DetectDeploy inspects the repository for common deployment platform configs.
func DetectDeploy(rootDir string) model.DeployConfig {
	switch {
	case fileExists(filepath.Join(rootDir, "vercel.json")):
		return model.DeployConfig{
			Enabled:     true,
			Provider:    "vercel",
			Environment: "production",
			Directory:   ".",
		}
	case fileExists(filepath.Join(rootDir, "fly.toml")):
		return model.DeployConfig{
			Enabled:     true,
			Provider:    "fly",
			Environment: "production",
			Directory:   ".",
		}
	case fileExists(filepath.Join(rootDir, "wrangler.toml")) || fileExists(filepath.Join(rootDir, "wrangler.jsonc")) || fileExists(filepath.Join(rootDir, "wrangler.json")):
		provider := "cloudflare-workers"
		wrangler := readFileText(filepath.Join(rootDir, "wrangler.toml")) + readFileText(filepath.Join(rootDir, "wrangler.jsonc"))
		if strings.Contains(wrangler, "pages_build_output_dir") {
			provider = "cloudflare-pages"
		}
		return model.DeployConfig{
			Enabled:     true,
			Provider:    provider,
			Environment: "production",
			Directory:   ".",
			DistDir:     "dist",
		}
	default:
		return model.DeployConfig{}
	}
}
