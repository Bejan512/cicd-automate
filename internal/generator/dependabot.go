package generator

import (
	"fmt"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// RenderDependabot generates a .github/dependabot.yml covering GitHub Actions
// and every detected stack ecosystem.
func RenderDependabot(m model.Manifest) string {
	interval := m.Dependabot.Interval
	if interval == "" {
		interval = "weekly"
	}

	var b strings.Builder
	b.WriteString(generatedBanner)
	b.WriteString("version: 2\n")
	b.WriteString("updates:\n")
	b.WriteString("  - package-ecosystem: github-actions\n")
	b.WriteString("    directory: \"/\"\n")
	b.WriteString("    schedule:\n")
	fmt.Fprintf(&b, "      interval: %s\n", interval)

	seen := map[string]bool{}
	for _, s := range m.Stacks {
		eco := dependabotEcosystem(s.Type)
		if eco == "" {
			continue
		}
		dir := "/"
		if s.Directory != "" && s.Directory != "." {
			dir = "/" + strings.TrimPrefix(s.Directory, "/")
		}
		key := eco + ":" + dir
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Fprintf(&b, "\n  - package-ecosystem: %s\n", eco)
		fmt.Fprintf(&b, "    directory: %q\n", dir)
		b.WriteString("    schedule:\n")
		fmt.Fprintf(&b, "      interval: %s\n", interval)
	}

	if m.Docker.Enabled {
		dir := "/"
		if m.Docker.Context != "" && m.Docker.Context != "." {
			dir = "/" + strings.TrimPrefix(m.Docker.Context, "/")
		}
		b.WriteString("\n  - package-ecosystem: docker\n")
		fmt.Fprintf(&b, "    directory: %q\n", dir)
		b.WriteString("    schedule:\n")
		fmt.Fprintf(&b, "      interval: %s\n", interval)
	}

	return b.String()
}

func dependabotEcosystem(st model.StackType) string {
	switch st {
	case model.StackNode:
		return "npm"
	case model.StackPython:
		return "pip"
	case model.StackGo:
		return "gomod"
	case model.StackRust:
		return "cargo"
	default:
		return ""
	}
}
