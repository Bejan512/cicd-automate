# `cicd-automate`

Automate **GitHub Actions CI/CD creation and maintenance** across both new and existing projects.

`cicd-automate` combines two pieces in one repository:
1. **Centralized Reusable Workflows** (`.github/workflows/reusable-*.yml`) — maintain CI/CD best practices once, versioned with Git tags (`@v1`).
2. **A Fast Go CLI (`cicd`)** — auto-detects project stacks (Node/TS, Python, Go, Rust, Docker, monorepos, polyglot repos, and Makefile/justfile fallbacks), generates tiny caller workflows (or self-contained standalone YAML), audits repos, and bulk-onboards folders of projects.

---

## Quick Start

### 1. Publish or Fork This Repository
Push this repository to your GitHub account or organization (for example, `github.com/<your-username>/cicd-automate`) and create a version tag:

```bash
git tag v1
git push origin v1
```

### 2. Install the `cicd` CLI
From this repository:

```bash
make install
# or directly:
go install ./cmd/cicd
```

### 3. Set Your Per-User Default Owner (Once)
Every developer or organization sets their own default reusable workflow source — nothing is hardcoded to a single author:

```bash
cicd config set owner <your-github-username>
# Or specify a custom repo and tag:
cicd config set source <your-org>/<your-cicd-repo>@v1
```

This writes `~/.config/cicd-automate/config.yaml` (respects `$XDG_CONFIG_HOME` and `$CICD_CONFIG`).

### 4. Onboard Any New or Existing Project
```bash
# Interactive setup
cd ~/projects/my-app
cicd init

# Non-interactive with auto-detected defaults
cicd init --yes

# Override reusable workflow source for this specific project only
cicd init --source client-org/shared-workflows@v2

# Generate self-contained YAML with zero external workflow dependencies
cicd init --mode standalone
```

---

## Per-User Defaults & Per-Project Overrides

Workflow source resolution follows a clear precedence order:

1. **CLI flags**: `--source <owner>/<repo>@<ref>` or `--owner <owner>` (plus optional `--ref <tag>`)
2. **Per-project manifest**: `source:` in `.github/cicd.yml` (committed in each repo so every collaborator gets the same source)
3. **Environment variable**: `CICD_SOURCE=<owner>/<repo>@<ref>`
4. **Per-user config**: `~/.config/cicd-automate/config.yaml` (`cicd config set owner ...`)
5. **Fallback**: detected from the repo's git remote owner or `gh` CLI user

---

## CLI Commands

| Command | Description |
| :--- | :--- |
| `cicd config show` | Show your per-user configuration and effective workflow source |
| `cicd config set <key> <val>` | Set `owner`, `repo`, `ref`, `source`, `mode`, `docker-registry`, or `release-strategy` |
| `cicd detect [dir]` | Inspect a repo and print detected stacks, monorepo layout, Dockerfiles, and deploy targets (`--json`) |
| `cicd init [dir]` | Scaffold `.github/cicd.yml`, caller/standalone workflows, and `.github/dependabot.yml` |
| `cicd sync [dir]` | Regenerate workflows from `.github/cicd.yml`, bump tags (`--ref v2`), or merge newly added stacks (`--redetect`) |
| `cicd doctor [dir]` | Validate workflow YAML, flag mutable refs & manifest drift, and check missing GitHub secrets (`--check-secrets`) |
| `cicd bulk <parent-dir>` | Scan a folder of projects, display a CI/CD coverage table, and batch-initialize (`--init`) or batch-sync (`--sync`) |

### Bulk Onboarding & Maintenance Example
```bash
# Audit all projects in ~/Desktop/projects
cicd bulk ~/Desktop/projects

# Preview onboarding every repo missing CI/CD
cicd bulk ~/Desktop/projects --init --dry-run

# Onboard all repos missing CI/CD
cicd bulk ~/Desktop/projects --init

# Upgrade all managed repos from @v1 to @v2 in one command
cicd bulk ~/Desktop/projects --sync --ref v2
```

---

## Included Reusable Workflows

| Workflow | Supports |
| :--- | :--- |
| `reusable-node.yml` | `npm`, `pnpm`, `yarn`, `bun`; `.nvmrc`/`.node-version`; lint, typecheck, test, build, coverage |
| `reusable-python.yml` | `uv`, `poetry`, `pip`; `.python-version`; `ruff` lint/format, `mypy`/`pyright`, `pytest`, coverage |
| `reusable-go.yml` | `go.mod` version detection, `go vet`, `golangci-lint`, `go test -race`, `go build`, coverage |
| `reusable-rust.yml` | `cargo fmt --check`, `clippy -D warnings`, `cargo test`, `cargo build --release`, `rust-cache` |
| `reusable-generic.yml` | Any stack via `Makefile`, `justfile`, or custom setup/lint/test/build shell commands |
| `reusable-docker.yml` | Multi-arch Buildx (`amd64`/`arm64`), GHCR & Docker Hub, GHA layer cache, Trivy security scan |
| `reusable-deploy.yml` | Vercel, Fly.io, Cloudflare Pages/Workers, AWS S3 (OIDC), or custom deploy command |
| `reusable-release.yml` | `release-please` (automated semver + `CHANGELOG.md`), `tag-release`, and `goreleaser` |

### Monorepo & Polyglot Support
- Detects **Turborepo**, **Nx**, **pnpm/npm workspaces**, **Cargo workspaces**, and **Go workspaces**.
- Detects multi-service polyglot repositories (for example `frontend/` in TypeScript + `services/api/` in Go + `services/ml/` in Python) and generates **path-filtered jobs** (`dorny/paths-filter@v3`) so PRs only run CI for changed services.

---

## Extending with New Stacks
To support another ecosystem (e.g., Elixir, Ruby, Java/Gradle):
1. Add a detector implementing `detector.StackDetector` in `internal/detector/` and register it in `DefaultDetectors()`.
2. Add a corresponding `.github/workflows/reusable-<stack>.yml` (or map it to `reusable-generic.yml`).
3. Wire its inputs in `internal/generator/caller.go` and `internal/generator/standalone.go`.
