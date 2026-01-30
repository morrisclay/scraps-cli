// Package cli implements the command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/morrisclay/scraps-cli/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:   "scraps",
	Short: "Scraps CLI - Git-native context sharing for AI agents",
	Long: `Scraps is a Git-native context sharing system for AI agents.
It provides stores, repositories, and coordination primitives
for multi-agent collaboration.`,
	Version: version.Version,
	SilenceUsage: true,
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add all command groups
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newLogoutCmd())
	rootCmd.AddCommand(newSignupCmd())
	rootCmd.AddCommand(newWhoamiCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newStoreCmd())
	rootCmd.AddCommand(newRepoCmd())
	rootCmd.AddCommand(newFileCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newCloneCmd())
	rootCmd.AddCommand(newTokenCmd())
	rootCmd.AddCommand(newKeyCmd())
	rootCmd.AddCommand(newClaimCmd())
	rootCmd.AddCommand(newReleaseCmd())
	rootCmd.AddCommand(newWatchCmd())
}

// helper functions for output

func success(msg string) {
	fmt.Printf("✓ %s\n", msg)
}

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
}

func warn(msg string) {
	fmt.Printf("! %s\n", msg)
}

func info(msg string) {
	fmt.Printf("→ %s\n", msg)
}
