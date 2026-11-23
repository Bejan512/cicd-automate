package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var (
		checkSecrets bool
		strict       bool
		asJSON       bool
	)

	cmd := &cobra.Command{
		Use:   "doctor [dir]",
		Short: "Validate GitHub Actions workflows, detect drift, and check required secrets",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			report, err := doctor.Run(dir, doctor.Options{
				CheckSecrets: checkSecrets,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "CI/CD Doctor: %s\n", report.ProjectDir)
				fmt.Fprintf(out, "  Workflows scanned: %d\n", report.WorkflowCount)
				fmt.Fprintf(out, "  Manifest present:  %t\n", report.HasManifest)
				if len(report.RequiredSecrets) > 0 {
					fmt.Fprintf(out, "  Secrets referenced: %s\n", strings.Join(report.RequiredSecrets, ", "))
				}

				if len(report.Findings) == 0 {
					fmt.Fprintln(out, "\nAll checks passed with no issues.")
				} else {
					fmt.Fprintln(out, "\nFindings:")
					for _, f := range report.Findings {
						tag := strings.ToUpper(string(f.Severity))
						if f.File != "" {
							fmt.Fprintf(out, "  [%s] %s: %s\n", tag, f.File, f.Message)
						} else {
							fmt.Fprintf(out, "  [%s] %s\n", tag, f.Message)
						}
					}
				}
			}

			if report.HasErrors() {
				return fmt.Errorf("doctor found errors")
			}
			if strict && report.HasWarnings() {
				return fmt.Errorf("doctor found warnings (--strict)")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkSecrets, "check-secrets", false, "Verify referenced secrets exist in GitHub via `gh secret list`")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit with non-zero status on warnings as well as errors")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output doctor report as JSON")

	return cmd
}
