package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/morrisclay/scraps-cli/internal/config"
)

func newConfigCmd() *cobra.Command {
	var host, outputFormat string
	var show bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or update CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show config if --show or no flags
			if show || (host == "" && outputFormat == "") {
				cfg, err := config.LoadConfig()
				if err != nil {
					return err
				}

				if config.GetOutputFormat() == "json" {
					outputJSON(cfg)
				} else {
					fmt.Printf("default_host:  %s\n", cfg.DefaultHost)
					fmt.Printf("output_format: %s\n", cfg.OutputFormat)
				}
				return nil
			}

			// Update config
			if host != "" {
				if err := config.SetHost(host); err != nil {
					return fmt.Errorf("failed to set host: %w", err)
				}
				success(fmt.Sprintf("Default host set to %s", host))
			}

			if outputFormat != "" {
				if outputFormat != "table" && outputFormat != "json" {
					return fmt.Errorf("output format must be 'table' or 'json'")
				}
				if err := config.SetOutputFormat(outputFormat); err != nil {
					return fmt.Errorf("failed to set output format: %w", err)
				}
				success(fmt.Sprintf("Output format set to %s", outputFormat))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Set default host")
	cmd.Flags().StringVar(&outputFormat, "output", "", "Set output format (table, json)")
	cmd.Flags().BoolVar(&show, "show", false, "Show current configuration")

	return cmd
}
