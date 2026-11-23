package cli

import (
	"fmt"
	"os"

	"github.com/cicd-automate/cicd-automate/internal/config"
	"github.com/cicd-automate/cicd-automate/internal/detector"
	"github.com/cicd-automate/cicd-automate/internal/generator"
	"github.com/cicd-automate/cicd-automate/internal/gitinfo"
	"github.com/cicd-automate/cicd-automate/internal/model"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		sourceFlag string
		ownerFlag  string
		refFlag    string
		modeFlag   string
		redetect   bool
		dryRun     bool
		local      bool
	)

	cmd := &cobra.Command{
		Use:   "sync [dir]",
		Short: "Regenerate workflows from .github/cicd.yml or upgrade reusable workflow ref",
		Long: `Keeps an existing project's workflows up to date with .github/cicd.yml.

Examples:
  cicd sync                    # Regenerate workflows from .github/cicd.yml
  cicd sync --ref v2           # Upgrade reusable workflow version tag to @v2
  cicd sync --source org/ci@v2 # Override reusable workflow source for this project
  cicd sync --redetect         # Re-scan repo for newly added stacks, Dockerfiles, or deploy configs`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			return runSync(cmd, dir, syncOpts{
				Source:   sourceFlag,
				Owner:    ownerFlag,
				Ref:      refFlag,
				Mode:     modeFlag,
				Redetect: redetect,
				DryRun:   dryRun,
				Local:    local,
			})
		},
	}

	cmd.Flags().StringVar(&sourceFlag, "source", "", "Override reusable workflow source (<owner>/<repo>[@<ref>])")
	cmd.Flags().StringVar(&ownerFlag, "owner", "", "Override GitHub owner/org for this project")
	cmd.Flags().StringVar(&refFlag, "ref", "", "Upgrade/change reusable workflow git ref (e.g. v2)")
	cmd.Flags().StringVar(&modeFlag, "mode", "", "Switch workflow mode ('reusable' or 'standalone')")
	cmd.Flags().BoolVar(&redetect, "redetect", false, "Re-detect stacks, Dockerfile, and deploy configs and merge into manifest")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
	cmd.Flags().BoolVar(&local, "local", false, "Reference local ./.github/workflows/reusable-*.yml")

	return cmd
}

type syncOpts struct {
	Source   string
	Owner    string
	Ref      string
	Mode     string
	Redetect bool
	DryRun   bool
	Local    bool
}

func runSync(cmd *cobra.Command, dir string, opts syncOpts) error {
	out := cmd.OutOrStdout()

	manifest, err := generator.LoadManifest(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s found in %s — run `cicd init` first", generator.ManifestRelPath, dir)
		}
		return err
	}

	if opts.Mode != "" {
		m := model.WorkflowMode(opts.Mode)
		if m != model.ModeReusable && m != model.ModeStandalone {
			return fmt.Errorf("invalid mode %q: expected 'reusable' or 'standalone'", opts.Mode)
		}
		manifest.Mode = m
	}

	if opts.Redetect {
		det, err := detector.DetectProject(dir)
		if err != nil {
			return err
		}
		manifest.Stacks = det.Stacks
		manifest.Monorepo = det.IsMonorepo
		if det.Docker.Enabled && !manifest.Docker.Enabled {
			manifest.Docker = det.Docker
		}
		if det.Deploy.Enabled && !manifest.Deploy.Enabled {
			manifest.Deploy = det.Deploy
		}
	}

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return err
	}

	var srcRef model.SourceRef
	if manifest.Mode == model.ModeReusable && !opts.Local {
		ginfo := gitinfo.Inspect(dir)
		res, err := config.ResolveSource(config.ResolveOptions{
			CLISource:     opts.Source,
			CLIOwner:      opts.Owner,
			CLIRef:        opts.Ref,
			ProjectSource: manifest.Source,
			UserCfg:       userCfg,
			FallbackOwner: ginfo.RemoteOwner,
		})
		if err != nil {
			return err
		}
		srcRef = res.Ref
		manifest.Source = srcRef.String()
	}

	files, err := generator.Generate(manifest, srcRef, generator.Options{LocalReusable: opts.Local})
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Fprintf(out, "[dry-run] Would sync %s:\n", dir)
		for _, gf := range files {
			fmt.Fprintf(out, "  - %s (%d bytes)\n", gf.RelPath, len(gf.Content))
		}
		return nil
	}

	writeRes, err := generator.WriteFiles(dir, files, true)
	if err != nil {
		return err
	}

	for _, p := range writeRes.Created {
		fmt.Fprintf(out, "  created   %s\n", p)
	}
	for _, p := range writeRes.Updated {
		fmt.Fprintf(out, "  updated   %s\n", p)
	}
	if len(writeRes.Created) == 0 && len(writeRes.Updated) == 0 {
		fmt.Fprintln(out, "  Already up to date.")
	}
	return nil
}
