package cli

import (
	"fmt"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/config"
	"github.com/cicd-automate/cicd-automate/internal/detector"
	"github.com/cicd-automate/cicd-automate/internal/generator"
	"github.com/cicd-automate/cicd-automate/internal/gitinfo"
	"github.com/cicd-automate/cicd-automate/internal/model"
	"github.com/cicd-automate/cicd-automate/internal/prompt"
	"github.com/spf13/cobra"
)

type initFlags struct {
	Yes          bool
	Source       string
	Owner        string
	Ref          string
	Mode         string
	Force        bool
	DryRun       bool
	Local        bool
	EnableRel    bool
	NoDependabot bool
	DockerReg    string
	DeployProv   string
}

func newInitCmd() *cobra.Command {
	var f initFlags

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Detect project stack(s) and generate GitHub Actions CI/CD workflows",
		Long: `Inspects a new or existing project directory and generates:
  - .github/cicd.yml           (project CI/CD manifest with optional per-project source override)
  - .github/workflows/ci.yml   (lint, format, typecheck, test, build — monorepo path-filter aware)
  - .github/workflows/docker.yml   (if Dockerfile detected or enabled)
  - .github/workflows/deploy.yml   (if deploy config detected or enabled)
  - .github/workflows/release.yml  (if automated releases enabled)
  - .github/dependabot.yml         (keeps actions and dependencies fresh)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			return runInit(cmd, dir, f)
		},
	}

	cmd.Flags().BoolVarP(&f.Yes, "yes", "y", false, "Accept detected defaults without interactive prompts")
	cmd.Flags().StringVar(&f.Source, "source", "", "Per-project reusable workflow source (<owner>/<repo>[@<ref>])")
	cmd.Flags().StringVar(&f.Owner, "owner", "", "Per-project GitHub owner/org override for reusable workflows")
	cmd.Flags().StringVar(&f.Ref, "ref", "", "Reusable workflow git ref override (e.g. v1, v2)")
	cmd.Flags().StringVar(&f.Mode, "mode", "", "Workflow mode: 'reusable' or 'standalone'")
	cmd.Flags().BoolVar(&f.Force, "force", false, "Overwrite existing workflow files")
	cmd.Flags().BoolVar(&f.DryRun, "dry-run", false, "Preview generated files without writing them")
	cmd.Flags().BoolVar(&f.Local, "local", false, "Reference local ./.github/workflows/reusable-*.yml")
	cmd.Flags().BoolVar(&f.EnableRel, "release", false, "Enable automated release workflow (release-please / tag-release)")
	cmd.Flags().BoolVar(&f.NoDependabot, "no-dependabot", false, "Skip generating .github/dependabot.yml")
	cmd.Flags().StringVar(&f.DockerReg, "docker-registry", "", "Container registry override (ghcr.io or docker.io)")
	cmd.Flags().StringVar(&f.DeployProv, "deploy", "", "Enable deploy workflow for provider (vercel, fly, cloudflare-pages, cloudflare-workers, aws-s3, custom)")

	return cmd
}

func runInit(cmd *cobra.Command, dir string, f initFlags) error {
	out := cmd.OutOrStdout()
	p := prompt.New(f.Yes)
	p.Out = out

	det, err := detector.DetectProject(dir)
	if err != nil {
		return err
	}

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Inspecting %s...\n", det.ProjectDir)
	if len(det.Stacks) > 0 {
		names := make([]string, 0, len(det.Stacks))
		for _, s := range det.Stacks {
			names = append(names, fmt.Sprintf("%s (%s)", s.Type, s.Directory))
		}
		fmt.Fprintf(out, "  Detected stacks: %s\n", strings.Join(names, ", "))
	} else {
		fmt.Fprintln(out, "  No standard stack detected automatically.")
	}
	if det.Docker.Enabled {
		fmt.Fprintf(out, "  Detected Docker: %s\n", det.Docker.Dockerfile)
	}
	if det.Deploy.Enabled {
		fmt.Fprintf(out, "  Detected Deploy: %s\n", det.Deploy.Provider)
	}

	// Determine workflow mode (reusable vs standalone).
	mode := userCfg.DefaultMode
	if f.Mode != "" {
		mode = model.WorkflowMode(f.Mode)
	} else if !f.Yes {
		chosen, err := p.Select("Workflow generation mode", []string{"reusable", "standalone"}, string(mode))
		if err != nil {
			return err
		}
		mode = model.WorkflowMode(chosen)
	}
	if mode != model.ModeReusable && mode != model.ModeStandalone {
		return fmt.Errorf("invalid mode %q: expected 'reusable' or 'standalone'", mode)
	}

	// Resolve reusable workflow source (per-user default + per-project override).
	var resolvedSrc model.SourceRef
	var projectSourceOverride string

	if mode == model.ModeReusable && !f.Local {
		fallbackOwner := det.GitOwner
		if fallbackOwner == "" {
			fallbackOwner = gitinfo.DetectGHUser()
		}

		res, resErr := config.ResolveSource(config.ResolveOptions{
			CLISource:     f.Source,
			CLIOwner:      f.Owner,
			CLIRef:        f.Ref,
			UserCfg:       userCfg,
			FallbackOwner: fallbackOwner,
		})

		if resErr != nil {
			if f.Yes {
				return resErr
			}
			owner, err := p.Input("GitHub username/org hosting cicd-automate reusable workflows", fallbackOwner)
			if err != nil {
				return err
			}
			owner = strings.TrimSpace(owner)
			if owner == "" {
				return fmt.Errorf("a GitHub owner is required for reusable mode (or pass --mode standalone)")
			}
			resolvedSrc = model.SourceRef{
				Owner: owner,
				Repo:  userCfg.Repo,
				Ref:   userCfg.Ref,
			}
			if f.Ref != "" {
				resolvedSrc.Ref = f.Ref
			}
			saveDefault, err := p.Confirm(fmt.Sprintf("Save %q as your per-user default owner?", owner), true)
			if err != nil {
				return err
			}
			if saveDefault {
				userCfg.Owner = owner
				if err := config.SaveUserConfig(userCfg); err != nil {
					fmt.Fprintf(out, "Warning: could not save user config: %v\n", err)
				}
			}
		} else {
			resolvedSrc = res.Ref
			// If user hasn't saved a per-user owner yet and we fell back to git/gh detection, offer to confirm/save.
			if res.Origin == "detected" && !f.Yes {
				inputSrc, err := p.Input("Reusable workflow source (<owner>/<repo>@<ref>)", resolvedSrc.String())
				if err != nil {
					return err
				}
				parsed, err := model.ParseSourceRef(inputSrc)
				if err != nil {
					return err
				}
				resolvedSrc = parsed
				saveDefault, err := p.Confirm(fmt.Sprintf("Save %s as your per-user default?", resolvedSrc.String()), true)
				if err != nil {
					return err
				}
				if saveDefault {
					userCfg.Owner = resolvedSrc.Owner
					userCfg.Repo = resolvedSrc.Repo
					userCfg.Ref = resolvedSrc.Ref
					_ = config.SaveUserConfig(userCfg)
				}
			} else if res.Origin == "flag" {
				// Explicit per-project override passed via CLI flags!
				projectSourceOverride = resolvedSrc.String()
			} else if !f.Yes {
				// Let the user optionally override source for this specific project.
				inputSrc, err := p.Input("Reusable workflow source for this project", resolvedSrc.String())
				if err != nil {
					return err
				}
				parsed, err := model.ParseSourceRef(inputSrc)
				if err != nil {
					return err
				}
				if parsed.String() != resolvedSrc.String() {
					projectSourceOverride = parsed.String()
				}
				resolvedSrc = parsed
			}
		}

		// Always record the resolved source in .github/cicd.yml so any collaborator running `cicd sync`
		// on this repo gets the exact same reusable workflow reference without guessing!
		if projectSourceOverride == "" {
			projectSourceOverride = resolvedSrc.String()
		}
	}

	// If no stack was detected, offer to set up a generic stack.
	stacks := det.Stacks
	if len(stacks) == 0 && !f.Yes {
		addGeneric, err := p.Confirm("No known stack detected. Configure a custom generic CI workflow?", true)
		if err != nil {
			return err
		}
		if addGeneric {
			lintCmd, _ := p.Input("Lint command (optional)", "")
			testCmd, _ := p.Input("Test command (optional)", "")
			buildCmd, _ := p.Input("Build command (optional)", "")
			stacks = append(stacks, model.StackConfig{
				Name:         "ci",
				Type:         model.StackGeneric,
				Directory:    ".",
				RunLint:      lintCmd != "",
				RunTest:      testCmd != "",
				RunBuild:     buildCmd != "",
				LintCommand:  lintCmd,
				TestCommand:  testCmd,
				BuildCommand: buildCmd,
			})
		}
	}

	// Docker stage configuration.
	dockerCfg := det.Docker
	if f.DockerReg != "" {
		dockerCfg.Registry = f.DockerReg
	} else if dockerCfg.Registry == "" {
		dockerCfg.Registry = userCfg.DefaultDockerRegistry
	}
	if !f.Yes {
		enableDocker, err := p.Confirm("Enable Docker build & publish workflow?", dockerCfg.Enabled)
		if err != nil {
			return err
		}
		dockerCfg.Enabled = enableDocker
		if dockerCfg.Enabled {
			if dockerCfg.Context == "" {
				dockerCfg.Context = "."
			}
			if dockerCfg.Dockerfile == "" {
				dockerCfg.Dockerfile = "Dockerfile"
			}
			if dockerCfg.Platforms == "" {
				dockerCfg.Platforms = "linux/amd64,linux/arm64"
			}
			reg, err := p.Select("Container registry", []string{"ghcr.io", "docker.io"}, dockerCfg.Registry)
			if err != nil {
				return err
			}
			dockerCfg.Registry = reg
			dockerCfg.ScanTrivy = true
		}
	}

	// Deploy stage configuration.
	deployCfg := det.Deploy
	if f.DeployProv != "" {
		deployCfg.Enabled = true
		deployCfg.Provider = f.DeployProv
		if deployCfg.Environment == "" {
			deployCfg.Environment = "production"
		}
		if deployCfg.Directory == "" {
			deployCfg.Directory = "."
		}
	} else if !f.Yes {
		enableDeploy, err := p.Confirm("Enable automated Deploy workflow?", deployCfg.Enabled)
		if err != nil {
			return err
		}
		deployCfg.Enabled = enableDeploy
		if deployCfg.Enabled {
			prov, err := p.Select("Deploy provider", []string{"vercel", "fly", "cloudflare-pages", "cloudflare-workers", "aws-s3", "custom"}, deployCfg.Provider)
			if err != nil {
				return err
			}
			deployCfg.Provider = prov
			if deployCfg.Environment == "" {
				deployCfg.Environment = "production"
			}
			if deployCfg.Directory == "" {
				deployCfg.Directory = "."
			}
			if prov == "custom" {
				deployCfg.DeployCommand, _ = p.Input("Custom deploy command", "make deploy")
			}
		}
	}

	// Release stage configuration.
	releaseCfg := model.ReleaseConfig{
		Enabled:  f.EnableRel,
		Strategy: userCfg.DefaultReleaseStrategy,
	}
	if !f.Yes && !f.EnableRel {
		enableRel, err := p.Confirm("Enable automated Releases & Changelogs workflow?", false)
		if err != nil {
			return err
		}
		releaseCfg.Enabled = enableRel
		if releaseCfg.Enabled {
			strat, err := p.Select("Release strategy", []string{"release-please", "tag-release", "goreleaser"}, releaseCfg.Strategy)
			if err != nil {
				return err
			}
			releaseCfg.Strategy = strat
		}
	}

	dependabotCfg := model.DependabotConfig{
		Enabled:  !f.NoDependabot,
		Interval: "weekly",
	}

	manifest := model.Manifest{
		Version:       1,
		Source:        projectSourceOverride,
		Mode:          mode,
		DefaultBranch: det.DefaultBranch,
		Monorepo:      det.IsMonorepo,
		Stacks:        stacks,
		Docker:        dockerCfg,
		Deploy:        deployCfg,
		Release:       releaseCfg,
		Dependabot:    dependabotCfg,
	}

	files, err := generator.Generate(manifest, resolvedSrc, generator.Options{LocalReusable: f.Local})
	if err != nil {
		return err
	}

	if f.DryRun {
		fmt.Fprintln(out, "\n[dry-run] Would generate:")
		for _, gf := range files {
			fmt.Fprintf(out, "  - %s (%d bytes)\n", gf.RelPath, len(gf.Content))
		}
		return nil
	}

	writeRes, err := generator.WriteFiles(det.ProjectDir, files, f.Force)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	for _, pth := range writeRes.Created {
		fmt.Fprintf(out, "  created  %s\n", pth)
	}
	for _, pth := range writeRes.Updated {
		fmt.Fprintf(out, "  updated  %s\n", pth)
	}
	for _, pth := range writeRes.Skipped {
		fmt.Fprintf(out, "  skipped  %s (already exists; pass --force to overwrite)\n", pth)
	}

	if mode == model.ModeReusable && !f.Local {
		fmt.Fprintf(out, "\nWorkflows linked to: %s\n", resolvedSrc.String())
	}
	fmt.Fprintln(out, "Next: run `cicd doctor` to validate workflows and check required secrets.")
	return nil
}
