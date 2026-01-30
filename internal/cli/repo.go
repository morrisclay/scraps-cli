package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/tui/components"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories",
	}

	cmd.AddCommand(newRepoListCmd())
	cmd.AddCommand(newRepoCreateCmd())
	cmd.AddCommand(newRepoShowCmd())
	cmd.AddCommand(newRepoDeleteCmd())
	cmd.AddCommand(newRepoCollaboratorsCmd())

	return cmd
}

func newRepoListCmd() *cobra.Command {
	var useTable bool

	cmd := &cobra.Command{
		Use:   "list [store]",
		Short: "List repositories",
		Long:  "List repositories. If store is specified, lists repos in that store. Otherwise lists all accessible repos.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			var repos []struct {
				Store string
				Name  string
				ID    string
				CreatedAt string
			}

			if len(args) > 0 {
				// List repos in specific store
				storeRepos, err := client.ListRepos(args[0])
				if err != nil {
					return err
				}
				for _, r := range storeRepos {
					repos = append(repos, struct {
						Store     string
						Name      string
						ID        string
						CreatedAt string
					}{args[0], r.Name, r.ID, r.CreatedAt})
				}
			} else {
				// List all repos
				stores, err := client.ListStores()
				if err != nil {
					return err
				}
				for _, store := range stores {
					storeRepos, err := client.ListRepos(store.Slug)
					if err != nil {
						continue
					}
					for _, r := range storeRepos {
						repos = append(repos, struct {
							Store     string
							Name      string
							ID        string
							CreatedAt string
						}{store.Slug, r.Name, r.ID, r.CreatedAt})
					}
				}
			}

			if len(repos) == 0 {
				info("No repositories found")
				return nil
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(repos)
			} else {
				headers := []string{"REPOSITORY", "CREATED"}
				rows := make([][]string, len(repos))
				for i, r := range repos {
					rows[i] = []string{formatStoreRepo(r.Store, r.Name), formatDate(r.CreatedAt)}
				}

				// Interactive mode - use table or searchable list
				if isInteractive() {
					if useTable || len(args) > 0 {
						// Use interactive table for specific store or when flag set
						columns := []components.TableColumn{
							{Title: "REPOSITORY", Width: 30},
							{Title: "CREATED", Width: 15},
						}
						tableRows := make([]table.Row, len(rows))
						for i, row := range rows {
							tableRows[i] = row
						}
						selected, err := components.RunTableInline("Repositories", columns, tableRows)
						if err != nil {
							return err
						}
						if selected != nil {
							fmt.Printf("\nSelected: %s\n", selected[0])
						}
					} else {
						// Use searchable list for browsing all repos
						items := make([]components.SearchListItem, len(repos))
						for i, r := range repos {
							items[i] = components.NewSearchListItem(
								formatStoreRepo(r.Store, r.Name),
								fmt.Sprintf("Created: %s", formatDate(r.CreatedAt)),
								r,
							)
						}

						selected, err := components.RunSearchList("Select Repository", items)
						if err != nil {
							return err
						}
						if selected != nil {
							fmt.Printf("Selected: %s\n", selected.Title())
						}
					}
					return nil
				}

				// Non-interactive table output
				outputTable(headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&useTable, "table", false, "Use interactive table view instead of list")
	return cmd
}

func newRepoCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <store/repo>",
		Short:   "Create a new repository",
		Example: "  scraps repo create mystore/myrepo",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps repo create <store/repo>\n\nExample: scraps repo create mystore/myrepo")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			repo, err := client.CreateRepo(store, name)
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(repo)
			} else {
				success(fmt.Sprintf("Repository '%s/%s' created", store, repo.Name))
			}
			return nil
		},
	}
	return cmd
}

func newRepoShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <store/repo>",
		Short:   "Show repository details",
		Example: "  scraps repo show mystore/myrepo",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps repo show <store/repo>\n\nExample: scraps repo show mystore/myrepo")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			repo, err := client.GetRepo(store, name)
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(repo)
			} else {
				fmt.Printf("Name:           %s\n", repo.Name)
				fmt.Printf("Store:          %s\n", store)
				fmt.Printf("ID:             %s\n", repo.ID)
				fmt.Printf("Default Branch: %s\n", repo.DefaultBranch)
				fmt.Printf("Created:        %s\n", formatDateTime(repo.CreatedAt))
			}
			return nil
		},
	}
	return cmd
}

func newRepoDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <store/repo>",
		Short:   "Delete a repository",
		Example: "  scraps repo delete mystore/myrepo",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps repo delete <store/repo>\n\nExample: scraps repo delete mystore/myrepo")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}

			// Confirm deletion
			if !force && isInteractive() {
				confirmed, err := components.RunConfirm(
					"Delete Repository",
					fmt.Sprintf("Are you sure you want to delete '%s/%s'?\nThis cannot be undone.", store, name),
					true,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					info("Deletion cancelled")
					return nil
				}
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			if err := client.DeleteRepo(store, name); err != nil {
				return err
			}

			success(fmt.Sprintf("Repository '%s/%s' deleted", store, name))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- Repository Collaborators ---

func newRepoCollaboratorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collaborators",
		Short: "Manage repository collaborators",
	}

	cmd.AddCommand(newRepoCollaboratorsListCmd())
	cmd.AddCommand(newRepoCollaboratorsAddCmd())
	cmd.AddCommand(newRepoCollaboratorsRemoveCmd())

	return cmd
}

func newRepoCollaboratorsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <store/repo>",
		Short:   "List collaborators of a repository",
		Example: "  scraps repo collaborators list mystore/myrepo",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps repo collaborators list <store/repo>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			collabs, err := client.ListCollaborators(store, name)
			if err != nil {
				return err
			}

			if len(collabs) == 0 {
				info("No collaborators found")
				return nil
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(collabs)
			} else {
				headers := []string{"USERNAME", "ROLE", "ADDED"}
				rows := make([][]string, len(collabs))
				for i, c := range collabs {
					rows[i] = []string{c.Username, c.Role, formatDate(c.CreatedAt)}
				}

				// Use interactive table if available
				if isInteractive() {
					selected, err := outputInteractiveTable("Collaborators", headers, rows)
					if err != nil {
						return err
					}
					if selected != nil {
						fmt.Printf("\nSelected: %s (%s)\n", selected[0], selected[1])
					}
				} else {
					outputTable(headers, rows)
				}
			}
			return nil
		},
	}
	return cmd
}

func newRepoCollaboratorsAddCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:     "add <store/repo> <username>",
		Short:   "Add a collaborator to a repository",
		Example: "  scraps repo collaborators add mystore/myrepo johndoe --role write",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("repository and username required\n\nUsage: scraps repo collaborators add <store/repo> <username>\n\nExample: scraps repo collaborators add mystore/myrepo johndoe")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}
			username := args[1]

			if role == "" {
				role = "read"
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			collab, err := client.AddCollaborator(store, name, username, role)
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(collab)
			} else {
				success(fmt.Sprintf("Added %s to %s/%s with role %s", username, store, name, collab.Role))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "read", "Collaborator role (read, write, admin)")
	return cmd
}

func newRepoCollaboratorsRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove <store/repo> <username>",
		Short:   "Remove a collaborator from a repository",
		Example: "  scraps repo collaborators remove mystore/myrepo johndoe",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("repository and username required\n\nUsage: scraps repo collaborators remove <store/repo> <username>\n\nExample: scraps repo collaborators remove mystore/myrepo johndoe")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, name, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}
			username := args[1]

			// Confirm removal
			if !force && isInteractive() {
				confirmed, err := components.RunConfirm(
					"Remove Collaborator",
					fmt.Sprintf("Remove '%s' from '%s/%s'?", username, store, name),
					false,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					info("Removal cancelled")
					return nil
				}
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			// Find collaborator ID
			collabs, err := client.ListCollaborators(store, name)
			if err != nil {
				return err
			}

			var collabID string
			for _, c := range collabs {
				if c.Username == username {
					collabID = c.ID
					break
				}
			}

			if collabID == "" {
				return fmt.Errorf("collaborator '%s' not found in '%s/%s'", username, store, name)
			}

			if err := client.RemoveCollaborator(store, name, collabID); err != nil {
				return err
			}

			success(fmt.Sprintf("Removed %s from %s/%s", username, store, name))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}
