package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cicd-automate/cicd-automate/internal/detector"
	"github.com/cicd-automate/cicd-automate/internal/generator"
	"github.com/spf13/cobra"
)

func newBulkCmd() *cobra.Command {
	var (
		doInit     bool
		doSync     bool
		sourceFlag string
		ownerFlag  string
		refFlag    string
		modeFlag   string
		dryRun     bool
		enableRel  bool
	)

	cmd := &cobra.Command{
		Use:   "bulk <parent-dir>",
		Short: "Audit, initialize, or sync CI/CD across a directory of multiple projects",
		Long: `Scans immediate subdirectories of <parent-dir> (for example ~/Desktop/projects),
displays a CI/CD coverage table, and optionally initializes repos without CI
or syncs repos managed by cicd-automate.

Examples:
  cicd bulk ~/Desktop/projects                     # Report CI/CD status across all repos
  cicd bulk ~/Desktop/projects --init --dry-run    # Preview onboarding repos missing CI
  cicd bulk ~/Desktop/projects --init              # Auto-setup CI/CD on repos missing workflows
  cicd bulk ~/Desktop/projects --sync --ref v2     # Upgrade all managed repos to @v2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(parentDir)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tSTACKS\tCI STATUS\tMANIFEST\tSOURCE")

			type projectRow struct {
				name        string
				path        string
				hasCI       bool
				hasManifest bool
				hasStacks   bool
			}
			var rows []projectRow

			for _, entry := range entries {
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				projPath := filepath.Join(parentDir, entry.Name())
				det, err := detector.DetectProject(projPath)
				if err != nil {
					continue
				}
				// Skip non-project directories (no git repo and no detected stacks/docker/workflows).
				isGit := false
				if _, err := os.Stat(filepath.Join(projPath, ".git")); err == nil {
					isGit = true
				}
				if !isGit && len(det.Stacks) == 0 && !det.Docker.Enabled && len(det.ExistingWorkflows) == 0 {
					continue
				}

				stackLabels := []string{}
				for _, s := range det.Stacks {
					stackLabels = append(stackLabels, string(s.Type))
				}
				if det.Docker.Enabled {
					stackLabels = append(stackLabels, "docker")
				}
				if det.Deploy.Enabled {
					stackLabels = append(stackLabels, "deploy:"+det.Deploy.Provider)
				}
				stackStr := "-"
				if len(stackLabels) > 0 {
					stackStr = strings.Join(stackLabels, ",")
				}

				ciStatus := "missing"
				if len(det.ExistingWorkflows) > 0 {
					ciStatus = fmt.Sprintf("%d workflow(s)", len(det.ExistingWorkflows))
				}

				manifestStr := "no"
				sourceStr := "-"
				if det.HasManifest {
					manifestStr = "yes"
					if m, err := generator.LoadManifest(projPath); err == nil && m.Source != "" {
						sourceStr = m.Source
					}
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", entry.Name(), stackStr, ciStatus, manifestStr, sourceStr)
				rows = append(rows, projectRow{
					name:        entry.Name(),
					path:        projPath,
					hasCI:       len(det.ExistingWorkflows) > 0,
					hasManifest: det.HasManifest,
					hasStacks:   len(det.Stacks) > 0 || det.Docker.Enabled,
				})
			}
			_ = tw.Flush()

			if !doInit && !doSync {
				fmt.Fprintln(out, "\nTip: pass `--init` to set up missing repos, or `--sync` to update managed repos.")
				return nil
			}

			if doInit {
				fmt.Fprintln(out, "\n--- Initializing projects without CI/CD ---")
				initCount := 0
				for _, r := range rows {
					if r.hasCI || !r.hasStacks {
						continue
					}
					fmt.Fprintf(out, "\n==> %s\n", r.name)
					err := runInit(cmd, r.path, initFlags{
						Yes:       true,
						Source:    sourceFlag,
						Owner:     ownerFlag,
						Ref:       refFlag,
						Mode:      modeFlag,
						DryRun:    dryRun,
						EnableRel: enableRel,
					})
					if err != nil {
						fmt.Fprintf(out, "  error: %v\n", err)
						continue
					}
					initCount++
				}
				fmt.Fprintf(out, "\nInitialized %d project(s).\n", initCount)
			}

			if doSync {
				fmt.Fprintln(out, "\n--- Syncing managed projects ---")
				syncCount := 0
				for _, r := range rows {
					if !r.hasManifest {
						continue
					}
					fmt.Fprintf(out, "\n==> %s\n", r.name)
					err := runSync(cmd, r.path, syncOpts{
						Source: sourceFlag,
						Owner:  ownerFlag,
						Ref:    refFlag,
						Mode:   modeFlag,
						DryRun: dryRun,
					})
					if err != nil {
						fmt.Fprintf(out, "  error: %v\n", err)
						continue
					}
					syncCount++
				}
				fmt.Fprintf(out, "\nSynced %d project(s).\n", syncCount)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&doInit, "init", false, "Auto-initialize projects that currently have no CI workflows")
	cmd.Flags().BoolVar(&doSync, "sync", false, "Sync all projects that have a .github/cicd.yml manifest")
	cmd.Flags().StringVar(&sourceFlag, "source", "", "Override reusable workflow source (<owner>/<repo>[@<ref>])")
	cmd.Flags().StringVar(&ownerFlag, "owner", "", "Override GitHub owner/org")
	cmd.Flags().StringVar(&refFlag, "ref", "", "Override reusable workflow ref (e.g. v2)")
	cmd.Flags().StringVar(&modeFlag, "mode", "", "Workflow mode ('reusable' or 'standalone')")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing files")
	cmd.Flags().BoolVar(&enableRel, "release", false, "Enable release workflow when initializing")

	return cmd
}
