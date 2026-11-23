package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cicd-automate/cicd-automate/internal/model"
	"gopkg.in/yaml.v3"
)

const ManifestRelPath = ".github/cicd.yml"

// LoadManifest reads and parses .github/cicd.yml from projectDir.
func LoadManifest(projectDir string) (model.Manifest, error) {
	path := filepath.Join(projectDir, ManifestRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Manifest{}, err
	}
	var m model.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return model.Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.Version == 0 {
		m.Version = 1
	}
	if m.Mode == "" {
		m.Mode = model.ModeReusable
	}
	if m.DefaultBranch == "" {
		m.DefaultBranch = "main"
	}
	return m, nil
}

// RenderManifest serializes a Manifest to YAML with a helpful header comment.
func RenderManifest(m model.Manifest) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("# Managed by cicd-automate. Run `cicd sync` after editing this file.\n")
	buf.WriteString("# To override the reusable workflow source for this repo only, edit `source` below.\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&m); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
