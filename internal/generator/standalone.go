package generator

import (
	"fmt"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/model"
)

// RenderStandaloneCI generates a fully self-contained .github/workflows/ci.yml
// with no external reusable workflow dependencies.
func RenderStandaloneCI(m model.Manifest) string {
	var b strings.Builder
	b.WriteString(generatedBanner)
	b.WriteString("name: CI\n\n")
	b.WriteString("on:\n")
	b.WriteString("  push:\n")
	fmt.Fprintf(&b, "    branches: [%s]\n", m.DefaultBranch)
	b.WriteString("  pull_request:\n")
	b.WriteString("  workflow_dispatch:\n\n")
	b.WriteString("concurrency:\n")
	b.WriteString("  group: ${{ github.workflow }}-${{ github.ref }}\n")
	b.WriteString("  cancel-in-progress: true\n\n")
	b.WriteString("permissions:\n")
	b.WriteString("  contents: read\n\n")
	b.WriteString("jobs:\n")

	usePathFilter := shouldUsePathFilter(m.Stacks)
	if usePathFilter {
		writePathFilterJob(&b, m.Stacks)
	}

	for _, s := range m.Stacks {
		jobID := sanitizeJobID(s.Name)
		dir := s.Directory
		if dir == "" {
			dir = "."
		}
		fmt.Fprintf(&b, "  %s:\n", jobID)
		fmt.Fprintf(&b, "    name: %s (%s)\n", strings.ToUpper(string(s.Type)), dir)
		if usePathFilter {
			b.WriteString("    needs: changes\n")
			fmt.Fprintf(&b, "    if: github.event_name != 'pull_request' || needs.changes.outputs.%s == 'true'\n", jobID)
		}
		b.WriteString("    runs-on: ubuntu-latest\n")
		b.WriteString("    defaults:\n")
		b.WriteString("      run:\n")
		fmt.Fprintf(&b, "        working-directory: %s\n", dir)
		b.WriteString("    steps:\n")
		b.WriteString("      - name: Checkout repository\n")
		b.WriteString("        uses: actions/checkout@v4\n\n")

		switch s.Type {
		case model.StackNode:
			writeStandaloneNodeSteps(&b, s, dir)
		case model.StackPython:
			writeStandalonePythonSteps(&b, s)
		case model.StackGo:
			writeStandaloneGoSteps(&b, s, dir)
		case model.StackRust:
			writeStandaloneRustSteps(&b, s, dir)
		default:
			writeStandaloneGenericSteps(&b, s)
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeStandaloneNodeSteps(b *strings.Builder, s model.StackConfig, dir string) {
	pm := s.PackageManager
	if pm == "" {
		pm = "npm"
	}
	if pm == "pnpm" {
		b.WriteString("      - name: Setup pnpm\n")
		b.WriteString("        uses: pnpm/action-setup@v4\n\n")
	}
	if pm == "bun" {
		b.WriteString("      - name: Setup Bun\n")
		b.WriteString("        uses: oven-sh/setup-bun@v2\n\n")
	} else {
		b.WriteString("      - name: Setup Node.js\n")
		b.WriteString("        uses: actions/setup-node@v4\n")
		b.WriteString("        with:\n")
		if s.VersionFile != "" {
			fmt.Fprintf(b, "          node-version-file: %q\n", s.VersionFile)
		} else {
			ver := s.RuntimeVersion
			if ver == "" {
				ver = "22"
			}
			fmt.Fprintf(b, "          node-version: %q\n", ver)
		}
		fmt.Fprintf(b, "          cache: %q\n", pm)
		if dir != "." {
			lockfile := "package-lock.json"
			switch pm {
			case "pnpm":
				lockfile = "pnpm-lock.yaml"
			case "yarn":
				lockfile = "yarn.lock"
			}
			fmt.Fprintf(b, "          cache-dependency-path: %q\n", strings.TrimSuffix(dir, "/")+"/"+lockfile)
		}
		b.WriteString("\n")
	}

	installCmd := "npm ci"
	switch pm {
	case "pnpm":
		installCmd = "pnpm install --frozen-lockfile"
	case "yarn":
		installCmd = "yarn install --frozen-lockfile"
	case "bun":
		installCmd = "bun install --frozen-lockfile"
	}
	fmt.Fprintf(b, "      - name: Install dependencies\n        run: %s\n", installCmd)

	if s.RunLint {
		cmd := s.LintCommand
		if cmd == "" {
			cmd = pm + " run lint"
		}
		fmt.Fprintf(b, "\n      - name: Lint\n        run: %s\n", cmd)
	}
	if s.RunTypecheck {
		cmd := s.TypecheckCmd
		if cmd == "" {
			cmd = pm + " run typecheck"
		}
		fmt.Fprintf(b, "\n      - name: Typecheck\n        run: %s\n", cmd)
	}
	if s.RunTest {
		cmd := s.TestCommand
		if cmd == "" {
			if pm == "bun" {
				cmd = "bun test"
			} else {
				cmd = pm + " test"
			}
		}
		fmt.Fprintf(b, "\n      - name: Test\n        run: %s\n", cmd)
	}
	if s.RunBuild {
		cmd := s.BuildCommand
		if cmd == "" {
			cmd = pm + " run build"
		}
		fmt.Fprintf(b, "\n      - name: Build\n        run: %s\n", cmd)
	}
}

func writeStandalonePythonSteps(b *strings.Builder, s model.StackConfig) {
	pm := s.PackageManager
	if pm == "" {
		pm = "uv"
	}
	prefix := ""
	switch pm {
	case "uv":
		prefix = "uv run "
		b.WriteString("      - name: Setup uv\n")
		b.WriteString("        uses: astral-sh/setup-uv@v5\n")
		b.WriteString("        with:\n")
		b.WriteString("          enable-cache: true\n\n")
		b.WriteString("      - name: Install dependencies\n")
		b.WriteString("        run: uv sync --all-extras --dev\n")
	case "poetry":
		prefix = "poetry run "
		ver := s.RuntimeVersion
		if ver == "" {
			ver = "3.12"
		}
		b.WriteString("      - name: Setup Python\n")
		b.WriteString("        uses: actions/setup-python@v5\n")
		b.WriteString("        with:\n")
		fmt.Fprintf(b, "          python-version: %q\n\n", ver)
		b.WriteString("      - name: Install dependencies\n")
		b.WriteString("        run: |\n")
		b.WriteString("          pip install --upgrade poetry\n")
		b.WriteString("          poetry install --no-interaction --no-ansi\n")
	default:
		ver := s.RuntimeVersion
		if ver == "" {
			ver = "3.12"
		}
		b.WriteString("      - name: Setup Python\n")
		b.WriteString("        uses: actions/setup-python@v5\n")
		b.WriteString("        with:\n")
		fmt.Fprintf(b, "          python-version: %q\n", ver)
		b.WriteString("          cache: pip\n\n")
		b.WriteString("      - name: Install dependencies\n")
		b.WriteString("        run: |\n")
		b.WriteString("          python -m pip install --upgrade pip\n")
		b.WriteString("          if [ -f requirements.txt ]; then pip install -r requirements.txt; fi\n")
		b.WriteString("          if [ -f requirements-dev.txt ]; then pip install -r requirements-dev.txt; fi\n")
	}

	if s.RunLint {
		cmd := s.LintCommand
		if cmd == "" {
			cmd = "ruff check ."
		}
		fmt.Fprintf(b, "\n      - name: Lint\n        run: %s%s\n", prefix, cmd)
	}
	if s.RunFormatCheck {
		cmd := s.FormatCommand
		if cmd == "" {
			cmd = "ruff format --check ."
		}
		fmt.Fprintf(b, "\n      - name: Format check\n        run: %s%s\n", prefix, cmd)
	}
	if s.RunTypecheck {
		cmd := s.TypecheckCmd
		if cmd == "" {
			cmd = "mypy ."
		}
		fmt.Fprintf(b, "\n      - name: Typecheck\n        run: %s%s\n", prefix, cmd)
	}
	if s.RunTest {
		cmd := s.TestCommand
		if cmd == "" {
			cmd = "pytest"
		}
		fmt.Fprintf(b, "\n      - name: Test\n        run: %s%s\n", prefix, cmd)
	}
}

func writeStandaloneGoSteps(b *strings.Builder, s model.StackConfig, dir string) {
	b.WriteString("      - name: Setup Go\n")
	b.WriteString("        uses: actions/setup-go@v5\n")
	b.WriteString("        with:\n")
	if s.VersionFile != "" {
		fmt.Fprintf(b, "          go-version-file: %q\n", s.VersionFile)
	} else {
		ver := s.RuntimeVersion
		if ver == "" {
			ver = "1.24"
		}
		fmt.Fprintf(b, "          go-version: %q\n", ver)
	}
	b.WriteString("          cache: true\n")
	b.WriteString("\n      - name: Vet\n        run: go vet ./...\n")
	if s.RunLint {
		b.WriteString("\n      - name: Lint\n")
		b.WriteString("        uses: golangci/golangci-lint-action@v6\n")
		b.WriteString("        with:\n")
		b.WriteString("          version: v1.64\n")
		fmt.Fprintf(b, "          working-directory: %q\n", dir)
	}
	if s.RunTest {
		b.WriteString("\n      - name: Test\n        run: go test -race ./...\n")
	}
	if s.RunBuild {
		cmd := s.BuildCommand
		if cmd == "" {
			cmd = "go build ./..."
		}
		fmt.Fprintf(b, "\n      - name: Build\n        run: %s\n", cmd)
	}
}

func writeStandaloneRustSteps(b *strings.Builder, s model.StackConfig, dir string) {
	tc := s.RuntimeVersion
	if tc == "" {
		tc = "stable"
	}
	b.WriteString("      - name: Install Rust toolchain\n")
	b.WriteString("        uses: dtolnay/rust-toolchain@master\n")
	b.WriteString("        with:\n")
	fmt.Fprintf(b, "          toolchain: %q\n", tc)
	b.WriteString("          components: rustfmt, clippy\n\n")
	b.WriteString("      - name: Cache Cargo artifacts\n")
	b.WriteString("        uses: Swatinem/rust-cache@v2\n")
	b.WriteString("        with:\n")
	fmt.Fprintf(b, "          workspaces: %q\n", dir)

	if s.RunFormatCheck {
		b.WriteString("\n      - name: Check formatting\n        run: cargo fmt --all -- --check\n")
	}
	if s.RunLint {
		b.WriteString("\n      - name: Clippy\n        run: cargo clippy --all-targets --all-features -- -D warnings\n")
	}
	if s.RunTest {
		b.WriteString("\n      - name: Test\n        run: cargo test --all-targets --all-features\n")
	}
	if s.RunBuild {
		b.WriteString("\n      - name: Build\n        run: cargo build --release\n")
	}
}

func writeStandaloneGenericSteps(b *strings.Builder, s model.StackConfig) {
	if s.SetupCommand != "" {
		fmt.Fprintf(b, "      - name: Setup\n        run: %s\n\n", s.SetupCommand)
	}
	if s.LintCommand != "" {
		fmt.Fprintf(b, "      - name: Lint\n        run: %s\n\n", s.LintCommand)
	}
	if s.TestCommand != "" {
		fmt.Fprintf(b, "      - name: Test\n        run: %s\n\n", s.TestCommand)
	}
	if s.BuildCommand != "" {
		fmt.Fprintf(b, "      - name: Build\n        run: %s\n", s.BuildCommand)
	}
}

// RenderStandaloneDocker generates a self-contained .github/workflows/docker.yml.
func RenderStandaloneDocker(m model.Manifest) string {
	d := m.Docker
	ctx := d.Context
	if ctx == "" {
		ctx = "."
	}
	df := d.Dockerfile
	if df == "" {
		df = "Dockerfile"
	}
	reg := d.Registry
	if reg == "" {
		reg = "ghcr.io"
	}
	platforms := d.Platforms
	if platforms == "" {
		platforms = "linux/amd64,linux/arm64"
	}

	var b strings.Builder
	b.WriteString(generatedBanner)
	b.WriteString("name: Docker\n\n")
	b.WriteString("on:\n")
	b.WriteString("  push:\n")
	fmt.Fprintf(&b, "    branches: [%s]\n", m.DefaultBranch)
	b.WriteString("    tags: ['v*']\n")
	b.WriteString("  pull_request:\n")
	b.WriteString("  workflow_dispatch:\n\n")
	b.WriteString("permissions:\n")
	b.WriteString("  contents: read\n")
	b.WriteString("  packages: write\n\n")
	b.WriteString("jobs:\n")
	b.WriteString("  docker:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	b.WriteString("      - uses: docker/setup-qemu-action@v3\n")
	b.WriteString("      - uses: docker/setup-buildx-action@v3\n")
	b.WriteString("      - name: Log in to registry\n")
	b.WriteString("        if: github.event_name != 'pull_request'\n")
	b.WriteString("        uses: docker/login-action@v3\n")
	b.WriteString("        with:\n")
	fmt.Fprintf(&b, "          registry: %s\n", reg)
	if reg == "ghcr.io" {
		b.WriteString("          username: ${{ github.actor }}\n")
		b.WriteString("          password: ${{ secrets.GITHUB_TOKEN }}\n")
	} else {
		b.WriteString("          username: ${{ secrets.REGISTRY_USERNAME }}\n")
		b.WriteString("          password: ${{ secrets.REGISTRY_PASSWORD }}\n")
	}
	b.WriteString("      - id: meta\n")
	b.WriteString("        uses: docker/metadata-action@v5\n")
	b.WriteString("        with:\n")
	img := d.ImageName
	if img == "" {
		img = "${{ github.repository }}"
	}
	fmt.Fprintf(&b, "          images: %s/%s\n", reg, img)
	b.WriteString("      - uses: docker/build-push-action@v6\n")
	b.WriteString("        with:\n")
	fmt.Fprintf(&b, "          context: %q\n", ctx)
	fmt.Fprintf(&b, "          file: %q\n", df)
	fmt.Fprintf(&b, "          platforms: %q\n", platforms)
	b.WriteString("          push: ${{ github.event_name != 'pull_request' }}\n")
	b.WriteString("          tags: ${{ steps.meta.outputs.tags }}\n")
	b.WriteString("          labels: ${{ steps.meta.outputs.labels }}\n")
	b.WriteString("          cache-from: type=gha\n")
	b.WriteString("          cache-to: type=gha,mode=max\n")
	return b.String()
}

// RenderStandaloneDeploy generates a self-contained .github/workflows/deploy.yml.
func RenderStandaloneDeploy(m model.Manifest) string {
	dep := m.Deploy
	envName := dep.Environment
	if envName == "" {
		envName = "production"
	}
	dir := dep.Directory
	if dir == "" {
		dir = "."
	}

	var b strings.Builder
	b.WriteString(generatedBanner)
	b.WriteString("name: Deploy\n\n")
	b.WriteString("on:\n")
	b.WriteString("  push:\n")
	fmt.Fprintf(&b, "    branches: [%s]\n", m.DefaultBranch)
	b.WriteString("  workflow_dispatch:\n\n")
	b.WriteString("concurrency:\n")
	fmt.Fprintf(&b, "  group: deploy-%s\n", envName)
	b.WriteString("  cancel-in-progress: false\n\n")
	b.WriteString("permissions:\n")
	b.WriteString("  contents: read\n")
	b.WriteString("  id-token: write\n\n")
	b.WriteString("jobs:\n")
	b.WriteString("  deploy:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	fmt.Fprintf(&b, "    environment: %s\n", envName)
	b.WriteString("    defaults:\n")
	b.WriteString("      run:\n")
	fmt.Fprintf(&b, "        working-directory: %s\n", dir)
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n")
	switch dep.Provider {
	case "vercel":
		b.WriteString("      - name: Deploy to Vercel\n")
		b.WriteString("        env:\n")
		b.WriteString("          VERCEL_ORG_ID: ${{ secrets.VERCEL_ORG_ID }}\n")
		b.WriteString("          VERCEL_PROJECT_ID: ${{ secrets.VERCEL_PROJECT_ID }}\n")
		b.WriteString("        run: |\n")
		b.WriteString("          npm install --global vercel@latest\n")
		b.WriteString("          vercel pull --yes --environment=production --token=${{ secrets.VERCEL_TOKEN }}\n")
		b.WriteString("          vercel build --prod --token=${{ secrets.VERCEL_TOKEN }}\n")
		b.WriteString("          vercel deploy --prebuilt --prod --token=${{ secrets.VERCEL_TOKEN }}\n")
	case "fly":
		b.WriteString("      - uses: superfly/flyctl-actions/setup-flyctl@master\n")
		b.WriteString("      - name: Deploy to Fly.io\n")
		b.WriteString("        env:\n")
		b.WriteString("          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}\n")
		b.WriteString("        run: flyctl deploy --remote-only\n")
	case "cloudflare-pages":
		b.WriteString("      - uses: cloudflare/wrangler-action@v3\n")
		b.WriteString("        with:\n")
		b.WriteString("          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n")
		b.WriteString("          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}\n")
		fmt.Fprintf(&b, "          command: pages deploy %s --project-name=%s\n", dep.DistDir, dep.ProjectName)
	case "cloudflare-workers":
		b.WriteString("      - uses: cloudflare/wrangler-action@v3\n")
		b.WriteString("        with:\n")
		b.WriteString("          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}\n")
		b.WriteString("          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}\n")
	default:
		cmd := dep.DeployCommand
		if cmd == "" {
			cmd = "echo 'Configure deploy_command in .github/cicd.yml'"
		}
		b.WriteString("      - name: Deploy\n")
		b.WriteString("        env:\n")
		b.WriteString("          DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}\n")
		fmt.Fprintf(&b, "        run: %s\n", cmd)
	}
	return b.String()
}

// RenderStandaloneRelease generates a self-contained .github/workflows/release.yml.
func RenderStandaloneRelease(m model.Manifest) string {
	strategy := m.Release.Strategy
	if strategy == "" {
		strategy = "release-please"
	}
	relType := m.Release.ReleaseType
	if relType == "" {
		relType = inferReleaseType(m.Stacks)
	}

	var b strings.Builder
	b.WriteString(generatedBanner)
	b.WriteString("name: Release\n\n")
	b.WriteString("on:\n")
	if strategy == "release-please" {
		b.WriteString("  push:\n")
		fmt.Fprintf(&b, "    branches: [%s]\n", m.DefaultBranch)
	} else {
		b.WriteString("  push:\n")
		b.WriteString("    tags: ['v*']\n")
	}
	b.WriteString("  workflow_dispatch:\n\n")
	b.WriteString("permissions:\n")
	b.WriteString("  contents: write\n")
	b.WriteString("  pull-requests: write\n\n")
	b.WriteString("jobs:\n")
	b.WriteString("  release:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	switch strategy {
	case "goreleaser":
		b.WriteString("      - uses: actions/checkout@v4\n")
		b.WriteString("        with:\n")
		b.WriteString("          fetch-depth: 0\n")
		b.WriteString("      - uses: actions/setup-go@v5\n")
		b.WriteString("        with:\n")
		b.WriteString("          go-version-file: go.mod\n")
		b.WriteString("      - uses: goreleaser/goreleaser-action@v6\n")
		b.WriteString("        with:\n")
		b.WriteString("          version: '~> v2'\n")
		b.WriteString("          args: release --clean\n")
		b.WriteString("        env:\n")
		b.WriteString("          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}\n")
	case "tag-release":
		b.WriteString("      - uses: actions/checkout@v4\n")
		b.WriteString("        with:\n")
		b.WriteString("          fetch-depth: 0\n")
		b.WriteString("      - name: Create GitHub Release\n")
		b.WriteString("        env:\n")
		b.WriteString("          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}\n")
		b.WriteString("        run: gh release create \"${GITHUB_REF_NAME}\" --generate-notes\n")
	default:
		b.WriteString("      - uses: googleapis/release-please-action@v4\n")
		b.WriteString("        with:\n")
		fmt.Fprintf(&b, "          release-type: %q\n", relType)
	}
	return b.String()
}
