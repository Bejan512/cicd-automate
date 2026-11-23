package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// NewRootCmd constructs the root `cicd` CLI command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cicd",
		Short: "Automate GitHub Actions CI/CD creation and maintenance across projects",
		Long: `cicd-automate inspects new or existing projects (Node/TS, Python, Go, Rust, Docker,
monorepos, or Makefile/justfile repos) and generates clean GitHub Actions workflows
backed by centralized reusable workflows — or standalone YAML when needed.

Configure your default reusable workflow repository once per user:
  cicd config set owner <your-github-username>

And override per project at any time:
  cicd init --source <org>/<repo>@v1`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newConfigCmd(),
		newDetectCmd(),
		newInitCmd(),
		newSyncCmd(),
		newDoctorCmd(),
		newBulkCmd(),
		&cobra.Command{
			Use:   "version",
			Short: "Print the cicd CLI version",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Fprintf(cmd.OutOrStdout(), "cicd %s\n", Version)
			},
		},
	)

	return root
}

// Execute runs the root CLI command.
func Execute() error {
	return NewRootCmd().Execute()
}
