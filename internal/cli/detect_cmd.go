package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/detector"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "detect [dir]",
		Short: "Inspect a project directory and print detected stacks and CI/CD signals",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			det, err := detector.DetectProject(dir)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(det)
			}

			fmt.Fprintf(out, "Project:        %s\n", det.ProjectDir)
			fmt.Fprintf(out, "Default branch: %s\n", det.DefaultBranch)
			if det.GitOwner != "" {
				fmt.Fprintf(out, "Git remote:     %s/%s\n", det.GitOwner, det.GitRepo)
			}
			if det.IsMonorepo {
				label := "yes"
				if det.MonorepoTool != "" {
					label = fmt.Sprintf("yes (%s)", det.MonorepoTool)
				}
				fmt.Fprintf(out, "Monorepo:       %s\n", label)
			}

			fmt.Fprintln(out, "\nDetected stacks:")
			if len(det.Stacks) == 0 {
				fmt.Fprintln(out, "  (none detected — you can still configure a custom generic stack during `cicd init`)")
			}
			for _, s := range det.Stacks {
				stages := []string{}
				if s.RunLint {
					stages = append(stages, "lint")
				}
				if s.RunFormatCheck {
					stages = append(stages, "fmt")
				}
				if s.RunTypecheck {
					stages = append(stages, "typecheck")
				}
				if s.RunTest {
					stages = append(stages, "test")
				}
				if s.RunBuild {
					stages = append(stages, "build")
				}
				pm := ""
				if s.PackageManager != "" {
					pm = fmt.Sprintf(" [%s]", s.PackageManager)
				}
				fmt.Fprintf(out, "  - %-16s dir=%-12s type=%s%s stages=[%s]\n",
					s.Name, s.Directory, s.Type, pm, strings.Join(stages, ", "))
			}

			fmt.Fprintln(out, "\nAdditional stages:")
			if det.Docker.Enabled {
				fmt.Fprintf(out, "  - Docker:   detected (%s)\n", det.Docker.Dockerfile)
			} else {
				fmt.Fprintln(out, "  - Docker:   not detected")
			}
			if det.Deploy.Enabled {
				fmt.Fprintf(out, "  - Deploy:   detected (%s)\n", det.Deploy.Provider)
			} else {
				fmt.Fprintln(out, "  - Deploy:   not detected")
			}

			if len(det.ExistingWorkflows) > 0 {
				fmt.Fprintf(out, "\nExisting workflows: %s\n", strings.Join(det.ExistingWorkflows, ", "))
			}
			if det.HasManifest {
				fmt.Fprintln(out, "Manifest:           .github/cicd.yml present")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output detection result as JSON")
	return cmd
}
