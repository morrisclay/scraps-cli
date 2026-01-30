// Package cli implements the command-line interface.
package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/morrisclay/scraps-cli/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scraps",
	Short: "Scraps CLI - Git-native context sharing for AI agents",
	Long: `Scraps is a Git-native context sharing system for AI agents.
It provides stores, repositories, and coordination primitives
for multi-agent collaboration.`,
	Version:      version.Version,
	SilenceUsage: true,
}


// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Command group IDs
const (
	groupAuth         = "auth"
	groupData         = "data"
	groupWorkflow     = "workflow"
	groupCoordination = "coordination"
	groupSettings     = "settings"
)

func init() {
	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Define command groups in display order
	rootCmd.AddGroup(
		&cobra.Group{ID: groupAuth, Title: "Authentication:"},
		&cobra.Group{ID: groupData, Title: "Data Management:"},
		&cobra.Group{ID: groupWorkflow, Title: "Workflow:"},
		&cobra.Group{ID: groupCoordination, Title: "Coordination:"},
		&cobra.Group{ID: groupSettings, Title: "Settings:"},
	)

	// Authentication commands
	rootCmd.AddCommand(withGroup(newSignupCmd(), groupAuth))
	rootCmd.AddCommand(withGroup(newLoginCmd(), groupAuth))
	rootCmd.AddCommand(withGroup(newLogoutCmd(), groupAuth))
	rootCmd.AddCommand(withGroup(newWhoamiCmd(), groupAuth))
	rootCmd.AddCommand(withGroup(newStatusCmd(), groupAuth))

	// Data management commands
	rootCmd.AddCommand(withGroup(newStoreCmd(), groupData))
	rootCmd.AddCommand(withGroup(newRepoCmd(), groupData))
	rootCmd.AddCommand(withGroup(newFileCmd(), groupData))

	// Workflow commands
	rootCmd.AddCommand(withGroup(newCloneCmd(), groupWorkflow))
	rootCmd.AddCommand(withGroup(newLogCmd(), groupWorkflow))
	rootCmd.AddCommand(withGroup(newWatchCmd(), groupWorkflow))

	// Coordination commands
	rootCmd.AddCommand(withGroup(newClaimCmd(), groupCoordination))
	rootCmd.AddCommand(withGroup(newReleaseCmd(), groupCoordination))

	// Settings commands
	rootCmd.AddCommand(withGroup(newConfigCmd(), groupSettings))
	rootCmd.AddCommand(withGroup(newKeyCmd(), groupSettings))
	rootCmd.AddCommand(withGroup(newTokenCmd(), groupSettings))

	// Register custom template functions and set usage template
	cobra.AddTemplateFunc("commandsByGroupOrdered", func(cmds []*cobra.Command, groupID string) []*cobra.Command {
		var result []*cobra.Command
		for _, c := range cmds {
			if c.GroupID == groupID && c.IsAvailableCommand() {
				result = append(result, c)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].Annotations["order"] < result[j].Annotations["order"]
		})
		return result
	})
	cobra.AddTemplateFunc("ungroupedCommands", func(cmd *cobra.Command) []*cobra.Command {
		var result []*cobra.Command
		for _, c := range cmd.Commands() {
			if c.GroupID == "" && c.IsAvailableCommand() {
				result = append(result, c)
			}
		}
		return result
	})
	rootCmd.SetUsageTemplate(usageTemplate)
}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{range .Groups}}

{{.Title}}{{range commandsByGroupOrdered $.Commands .ID}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if ungroupedCommands .}}

Additional Commands:{{range ungroupedCommands .}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// orderCounter tracks registration order
var orderCounter int

// withGroup assigns a command to a group with ordering annotation
func withGroup(cmd *cobra.Command, groupID string) *cobra.Command {
	cmd.GroupID = groupID
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations["order"] = fmt.Sprintf("%03d", orderCounter)
	orderCounter++
	return cmd
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
