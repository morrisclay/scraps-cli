package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/tui/components"
)

func newStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Manage stores",
	}

	cmd.AddCommand(newStoreListCmd())
	cmd.AddCommand(newStoreCreateCmd())
	cmd.AddCommand(newStoreShowCmd())
	cmd.AddCommand(newStoreDeleteCmd())
	cmd.AddCommand(newStoreMembersCmd())

	return cmd
}

func newStoreListCmd() *cobra.Command {
	var useTable bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stores you are a member of",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			stores, err := client.ListStores()
			if err != nil {
				return err
			}

			if len(stores) == 0 {
				info("No stores found")
				return nil
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(stores)
			} else {
				headers := []string{"SLUG", "ROLE", "CREATED"}
				rows := make([][]string, len(stores))
				for i, s := range stores {
					rows[i] = []string{s.Slug, s.Role, formatDate(s.CreatedAt)}
				}

				// Interactive mode - use table or searchable list
				if isInteractive() {
					if useTable {
						// Use interactive table
						columns := []components.TableColumn{
							{Title: "SLUG", Width: 20},
							{Title: "ROLE", Width: 10},
							{Title: "CREATED", Width: 15},
						}
						tableRows := make([]table.Row, len(rows))
						for i, row := range rows {
							tableRows[i] = row
						}
						selected, err := components.RunTableInline("Stores", columns, tableRows)
						if err != nil {
							return err
						}
						if selected != nil {
							fmt.Printf("\nSelected: %s\n", selected[0])
						}
					} else {
						// Use searchable list
						items := make([]components.SearchListItem, len(stores))
						for i, s := range stores {
							items[i] = components.NewSearchListItem(
								s.Slug,
								fmt.Sprintf("Role: %s • Created: %s", s.Role, formatDate(s.CreatedAt)),
								s,
							)
						}

						selected, err := components.RunSearchList("Select Store", items)
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

func newStoreCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <slug>",
		Short:   "Create a new store",
		Example: "  scraps store create mystore",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("store slug required. Usage: scraps store create <slug>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			if config.GetOutputFormat() != "json" {
				fmt.Printf("Creating store '%s'...\n", args[0])
			}

			store, err := client.CreateStore(args[0])
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(store)
			} else {
				success(fmt.Sprintf("Store '%s' created", store.Slug))
			}
			return nil
		},
	}
	return cmd
}

func newStoreShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <slug>",
		Short:   "Show store details",
		Example: "  scraps store show mystore",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("store slug required\n\nUsage: scraps store show <slug>\n\nExample: scraps store show mystore")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			store, err := client.GetStore(args[0])
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(store)
			} else {
				fmt.Printf("Slug:       %s\n", store.Slug)
				fmt.Printf("ID:         %s\n", store.ID)
				fmt.Printf("Role:       %s\n", store.Role)
				fmt.Printf("Created:    %s\n", formatDateTime(store.CreatedAt))
			}
			return nil
		},
	}
	return cmd
}

func newStoreDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <slug>",
		Short:   "Delete a store and all its repositories",
		Example: "  scraps store delete mystore",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("store slug required\n\nUsage: scraps store delete <slug>\n\nExample: scraps store delete mystore")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			// Confirm deletion
			if !force && isInteractive() {
				confirmed, err := components.RunConfirm(
					"Delete Store",
					fmt.Sprintf("Are you sure you want to delete '%s'?\nThis will delete ALL repositories in this store.\nThis cannot be undone.", slug),
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

			if err := client.DeleteStore(slug); err != nil {
				return err
			}

			success(fmt.Sprintf("Store '%s' deleted", slug))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- Store Members ---

func newStoreMembersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage store members",
	}

	cmd.AddCommand(newStoreMembersListCmd())
	cmd.AddCommand(newStoreMembersAddCmd())
	cmd.AddCommand(newStoreMembersUpdateCmd())
	cmd.AddCommand(newStoreMembersRemoveCmd())

	return cmd
}

func newStoreMembersListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <store>",
		Short:   "List members of a store",
		Example: "  scraps store members list mystore",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("store slug required\n\nUsage: scraps store members list <store>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			members, err := client.ListStoreMembers(args[0])
			if err != nil {
				return err
			}

			if len(members) == 0 {
				info("No members found")
				return nil
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(members)
			} else {
				headers := []string{"USERNAME", "ROLE", "ADDED"}
				rows := make([][]string, len(members))
				for i, m := range members {
					rows[i] = []string{m.Username, m.Role, formatDate(m.CreatedAt)}
				}

				// Use interactive table if available
				if isInteractive() {
					selected, err := outputInteractiveTable("Store Members", headers, rows)
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

func newStoreMembersAddCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:     "add <store> <username>",
		Short:   "Add a member to a store",
		Example: "  scraps store members add mystore johndoe --role member",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("store and username required\n\nUsage: scraps store members add <store> <username>\n\nExample: scraps store members add mystore johndoe")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, username := args[0], args[1]

			// Interactive role selection if not provided
			if role == "" && isInteractive() {
				items := []components.SearchListItem{
					components.NewSearchListItem("read", "Read-only access", "read"),
					components.NewSearchListItem("member", "Can create and manage repos", "member"),
					components.NewSearchListItem("admin", "Full administrative access", "admin"),
				}
				selected, err := components.RunSearchList("Select Role", items)
				if err != nil {
					return err
				}
				if selected == nil {
					return fmt.Errorf("role selection cancelled")
				}
				role = selected.Value().(string)
			}

			if role == "" {
				role = "read"
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			member, err := client.AddStoreMember(store, username, role)
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(member)
			} else {
				success(fmt.Sprintf("Added %s to %s with role %s", username, store, member.Role))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "Member role (admin, member, read)")
	return cmd
}

func newStoreMembersUpdateCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:     "update <store> <username>",
		Short:   "Update a member's role",
		Example: "  scraps store members update mystore johndoe --role admin",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("store and username required\n\nUsage: scraps store members update <store> <username> --role <role>\n\nExample: scraps store members update mystore johndoe --role admin")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, username := args[0], args[1]

			if role == "" {
				return fmt.Errorf("role is required")
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			// Find member ID
			members, err := client.ListStoreMembers(store)
			if err != nil {
				return err
			}

			var memberID string
			for _, m := range members {
				if m.Username == username {
					memberID = m.ID
					break
				}
			}

			if memberID == "" {
				return fmt.Errorf("member '%s' not found in store '%s'", username, store)
			}

			if err := client.UpdateStoreMember(store, memberID, role); err != nil {
				return err
			}

			success(fmt.Sprintf("Updated %s's role to %s", username, role))
			return nil
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "New role (admin, member, read)")
	cmd.MarkFlagRequired("role")
	return cmd
}

func newStoreMembersRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove <store> <username>",
		Short:   "Remove a member from a store",
		Example: "  scraps store members remove mystore johndoe",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("store and username required\n\nUsage: scraps store members remove <store> <username>\n\nExample: scraps store members remove mystore johndoe")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, username := args[0], args[1]

			// Confirm removal
			if !force && isInteractive() {
				confirmed, err := components.RunConfirm(
					"Remove Member",
					fmt.Sprintf("Remove '%s' from store '%s'?", username, store),
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

			// Find member ID
			members, err := client.ListStoreMembers(store)
			if err != nil {
				return err
			}

			var memberID string
			for _, m := range members {
				if m.Username == username {
					memberID = m.ID
					break
				}
			}

			if memberID == "" {
				return fmt.Errorf("member '%s' not found in store '%s'", username, store)
			}

			if err := client.RemoveStoreMember(store, memberID); err != nil {
				return err
			}

			success(fmt.Sprintf("Removed %s from %s", username, store))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}
