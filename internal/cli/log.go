package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
)

func newLogCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:     "log <store/repo[:branch]>",
		Short:   "Show commit history",
		Example: "  scraps log mystore/myrepo\n  scraps log mystore/myrepo:main -n 20",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps log <store/repo[:branch]>\n\nExample: scraps log mystore/myrepo")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, branch, err := parseStoreRepoBranch(args[0])
			if err != nil {
				return err
			}

			if branch == "" {
				branch = "main"
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			commits, err := client.GetLog(store, repo, branch, limit)
			if err != nil {
				return err
			}

			if len(commits) == 0 {
				info("No commits found")
				return nil
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(commits)
			} else {
				for _, c := range commits {
					sha := c.SHA
					if sha == "" {
						sha = c.Commit
					}
					if len(sha) > 7 {
						sha = sha[:7]
					}

					author := ""
					if c.Author.Name != "" {
						author = c.Author.Name
					} else if c.Author.Raw != "" {
						author = c.Author.Raw
					}

					date := ""
					if c.Date != "" {
						date = formatDateTime(c.Date)
					}

					msg := c.Message
					if len(msg) > 60 {
						msg = msg[:57] + "..."
					}

					fmt.Printf("\033[33m%s\033[0m %s\n", sha, msg)
					if author != "" || date != "" {
						fmt.Printf("         %s %s\n", author, date)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of commits to show")
	return cmd
}
