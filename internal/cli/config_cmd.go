package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/cicd-automate/cicd-automate/internal/config"
	"github.com/cicd-automate/cicd-automate/internal/model"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage per-user defaults (GitHub owner, reusable repo, ref, mode)",
		Long: `Configure per-user defaults stored in ~/.config/cicd-automate/config.yaml
(or $XDG_CONFIG_HOME/cicd-automate/config.yaml).

Examples:
  cicd config set owner octocat
  cicd config set source my-org/shared-cicd@v1
  cicd config set mode reusable
  cicd config show`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Show current per-user configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				out := cmd.OutOrStdout()
				path, err := config.ConfigPath()
				if err != nil {
					return err
				}
				cfg, err := config.LoadUserConfig()
				if err != nil {
					return err
				}

				fmt.Fprintf(out, "Config file: %s\n\n", path)
				ownerDisplay := cfg.Owner
				if ownerDisplay == "" {
					ownerDisplay = "(not set — run `cicd config set owner <github-username>`)"
				}
				fmt.Fprintf(out, "  owner:                    %s\n", ownerDisplay)
				fmt.Fprintf(out, "  repo:                     %s\n", cfg.Repo)
				fmt.Fprintf(out, "  ref:                      %s\n", cfg.Ref)
				if src, ok := cfg.SourceRef(); ok {
					fmt.Fprintf(out, "  effective source:         %s\n", src.String())
				}
				if envSrc := strings.TrimSpace(os.Getenv("CICD_SOURCE")); envSrc != "" {
					fmt.Fprintf(out, "  CICD_SOURCE env override: %s\n", envSrc)
				}
				fmt.Fprintf(out, "  default_mode:             %s\n", cfg.DefaultMode)
				fmt.Fprintf(out, "  default_docker_registry:  %s\n", cfg.DefaultDockerRegistry)
				fmt.Fprintf(out, "  default_release_strategy: %s\n", cfg.DefaultReleaseStrategy)
				return nil
			},
		},
		&cobra.Command{
			Use:   "path",
			Short: "Print path to the per-user config file",
			RunE: func(cmd *cobra.Command, args []string) error {
				p, err := config.ConfigPath()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), p)
				return nil
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a per-user default (owner, repo, ref, source, mode, docker-registry, release-strategy)",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				key := strings.ToLower(strings.TrimSpace(args[0]))
				val := strings.TrimSpace(args[1])

				cfg, err := config.LoadUserConfig()
				if err != nil {
					return err
				}

				switch key {
				case "owner":
					if strings.Contains(val, "/") {
						return fmt.Errorf("owner should be a GitHub username or org without '/'; use `cicd config set source %s` instead", val)
					}
					cfg.Owner = val
				case "repo":
					cfg.Repo = val
				case "ref":
					cfg.Ref = val
				case "source":
					parsed, err := model.ParseSourceRef(val)
					if err != nil {
						return err
					}
					cfg.Owner = parsed.Owner
					cfg.Repo = parsed.Repo
					cfg.Ref = parsed.Ref
				case "mode", "default_mode":
					m := model.WorkflowMode(val)
					if m != model.ModeReusable && m != model.ModeStandalone {
						return fmt.Errorf("invalid mode %q: expected 'reusable' or 'standalone'", val)
					}
					cfg.DefaultMode = m
				case "docker-registry", "default_docker_registry":
					cfg.DefaultDockerRegistry = val
				case "release-strategy", "default_release_strategy":
					switch val {
					case "release-please", "tag-release", "goreleaser":
						cfg.DefaultReleaseStrategy = val
					default:
						return fmt.Errorf("invalid release strategy %q: expected release-please, tag-release, or goreleaser", val)
					}
				default:
					return fmt.Errorf("unknown config key %q (valid: owner, repo, ref, source, mode, docker-registry, release-strategy)", key)
				}

				if err := config.SaveUserConfig(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s = %s\n", key, val)
				if src, ok := cfg.SourceRef(); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "Default reusable workflow source: %s\n", src.String())
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "unset <key>",
			Short: "Reset a per-user configuration key to default",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				key := strings.ToLower(strings.TrimSpace(args[0]))
				cfg, err := config.LoadUserConfig()
				if err != nil {
					return err
				}
				switch key {
				case "owner":
					cfg.Owner = ""
				case "repo":
					cfg.Repo = config.DefaultRepoName
				case "ref":
					cfg.Ref = config.DefaultRef
				case "source":
					cfg.Owner = ""
					cfg.Repo = config.DefaultRepoName
					cfg.Ref = config.DefaultRef
				case "mode", "default_mode":
					cfg.DefaultMode = model.ModeReusable
				case "docker-registry", "default_docker_registry":
					cfg.DefaultDockerRegistry = config.DefaultDockerRegistry
				case "release-strategy", "default_release_strategy":
					cfg.DefaultReleaseStrategy = config.DefaultReleaseStrategy
				default:
					return fmt.Errorf("unknown config key %q", key)
				}
				if err := config.SaveUserConfig(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Unset %s\n", key)
				return nil
			},
		},
	)

	return cmd
}
