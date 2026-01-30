package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "API key management",
	}

	cmd.AddCommand(newKeyResetRequestCmd())
	cmd.AddCommand(newKeyResetConfirmCmd())

	return cmd
}

func newKeyResetRequestCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:     "reset-request <email>",
		Short:   "Request an API key reset email",
		Example: "  scraps key reset-request user@example.com",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("email address required\n\nUsage: scraps key reset-request <email>\n\nExample: scraps key reset-request user@example.com")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			email := args[0]

			if host == "" {
				host = config.GetHost()
			}

			client := api.NewClient(host, "")
			if err := client.ResetAPIKeyRequest(email); err != nil {
				return err
			}

			success(fmt.Sprintf("Reset email sent to %s", email))
			info("Check your email for the reset link")
			return nil
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host")
	return cmd
}

func newKeyResetConfirmCmd() *cobra.Command {
	var host string
	var noLogin bool

	cmd := &cobra.Command{
		Use:     "reset-confirm <token>",
		Short:   "Confirm API key reset with token from email",
		Example: "  scraps key reset-confirm abc123token",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("reset token required\n\nUsage: scraps key reset-confirm <token>\n\nThe token is sent to your email after running 'scraps key reset-request'")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]

			if host == "" {
				host = config.GetHost()
			}

			client := api.NewClient(host, "")
			resp, err := client.ResetAPIKeyConfirm(token)
			if err != nil {
				return err
			}

			if !noLogin {
				// Save the new credentials
				if err := config.SetCredential(host, config.Credential{
					APIKey:   resp.APIKey,
					UserID:   resp.UserID,
					Username: resp.Username,
				}); err != nil {
					warn(fmt.Sprintf("Failed to save credentials: %v", err))
				} else {
					success(fmt.Sprintf("Logged in as %s", resp.Username))
				}
			}

			fmt.Printf("\nYour new API key: %s\n", resp.APIKey)
			fmt.Println("Save this key - it won't be shown again!")

			return nil
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host")
	cmd.Flags().BoolVar(&noLogin, "no-login", false, "Don't save credentials after reset")

	return cmd
}
