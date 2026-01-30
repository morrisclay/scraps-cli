package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/tui"
	"github.com/morrisclay/scraps-cli/internal/tui/components"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage API keys and tokens",
	}

	cmd.AddCommand(newTokenCreateCmd())
	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenRevokeCmd())

	return cmd
}

func newTokenCreateCmd() *cobra.Command {
	var name, store, repo, permission string
	var scoped bool
	var expires int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API key or scoped token",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			// Interactive wizard mode
			if isInteractive() && !scoped && name == "" {
				return runTokenWizard(client)
			}

			if scoped {
				// Create scoped token
				permissions := []string{permission}
				if permission == "" {
					permissions = []string{"read"}
				}

				var repos []string
				if repo != "" {
					repos = strings.Split(repo, ",")
				}

				resp, err := client.CreateScopedToken(name, store, repos, permissions, expires)
				if err != nil {
					return err
				}

				if config.GetOutputFormat() == "json" {
					outputJSON(resp)
				} else {
					success("Scoped token created")
					fmt.Printf("\nToken: %s\n", resp.RawKey)
					fmt.Println("\nSave this token - it won't be shown again!")
				}
			} else {
				// Create API key
				resp, err := client.CreateAPIKey(name)
				if err != nil {
					return err
				}

				if config.GetOutputFormat() == "json" {
					outputJSON(resp)
				} else {
					success("API key created")
					fmt.Printf("\nKey: %s\n", resp.RawKey)
					fmt.Println("\nSave this key - it won't be shown again!")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Token name/label")
	cmd.Flags().BoolVar(&scoped, "scoped", false, "Create scoped token instead of API key")
	cmd.Flags().StringVarP(&store, "store", "s", "", "Store ID for scoped token")
	cmd.Flags().StringVarP(&repo, "repo", "r", "", "Repository names (comma-separated) for scoped token")
	cmd.Flags().StringVarP(&permission, "permission", "p", "read", "Permission (read, write)")
	cmd.Flags().IntVar(&expires, "expires", 0, "Expiration in days")

	return cmd
}

// tokenWizardModel is the wizard for creating tokens.
type tokenWizardModel struct {
	client     *api.Client
	steps      []string
	current    int
	tokenType  string // "api-key" or "scoped"
	name       string
	store      string
	repos      []string
	permission string
	expires    int
	stores     []string
	allRepos   []string

	// Sub-components
	typeSelect   *components.SelectStep
	nameInput    *components.TextInputStep
	storeSelect  *components.ItemSelectStep
	repoSelect   *components.SelectStep
	permSelect   *components.SelectStep

	state  string // "type", "name", "store", "repo", "perm", "creating", "done", "error"
	result string
	err    error
}

func newTokenWizardModel(client *api.Client) tokenWizardModel {
	return tokenWizardModel{
		client:  client,
		steps:   []string{"type", "name", "store", "repo", "perm"},
		current: 0,
		state:   "type",
		typeSelect: components.NewSelectStep(
			"Token Type",
			"What type of token do you want to create?",
			[]string{"API Key (full access)", "Scoped Token (limited access)"},
		),
		nameInput: components.NewTextInputStep(
			"Token Name",
			"Enter a name for this token (optional):",
			"my-token",
		),
	}
}

func (m tokenWizardModel) Init() tea.Cmd {
	return m.typeSelect.Init()
}

type storesLoadedMsg struct {
	stores []string
	err    error
}

type reposLoadedMsg struct {
	repos []string
	err   error
}

type tokenCreatedMsg struct {
	key string
	err error
}

func (m tokenWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "esc" && m.current > 0 {
			m.current--
			m.state = m.steps[m.current]
			return m, nil
		}

	case storesLoadedMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
			return m, tea.Quit
		}
		m.stores = msg.stores
		items := make([]components.SearchListItem, len(msg.stores))
		for i, s := range msg.stores {
			items[i] = components.NewSearchListItem(s, "", s)
		}
		m.storeSelect = components.NewItemSelectStep("Select Store", "Choose a store:", items)
		return m, nil

	case reposLoadedMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
			return m, tea.Quit
		}
		m.allRepos = msg.repos
		options := append([]string{"All repositories"}, msg.repos...)
		m.repoSelect = components.NewSelectStep("Select Repository", "Choose repositories:", options)
		return m, nil

	case tokenCreatedMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
		} else {
			m.state = "done"
			m.result = msg.key
		}
		return m, tea.Quit
	}

	// Handle current step
	switch m.state {
	case "type":
		step, cmd := m.typeSelect.Update(msg)
		m.typeSelect = step.(*components.SelectStep)
		if m.typeSelect.IsComplete() {
			if m.typeSelect.SelectedIndex() == 0 {
				m.tokenType = "api-key"
				m.state = "name"
				m.current = 1
				return m, m.nameInput.Init()
			} else {
				m.tokenType = "scoped"
				m.state = "name"
				m.current = 1
				return m, m.nameInput.Init()
			}
		}
		return m, cmd

	case "name":
		step, cmd := m.nameInput.Update(msg)
		m.nameInput = step.(*components.TextInputStep)
		if m.nameInput.IsComplete() {
			m.name = m.nameInput.Value().(string)
			if m.tokenType == "api-key" {
				m.state = "creating"
				return m, func() tea.Msg {
					resp, err := m.client.CreateAPIKey(m.name)
					if err != nil {
						return tokenCreatedMsg{err: err}
					}
					return tokenCreatedMsg{key: resp.RawKey}
				}
			}
			// Load stores for scoped token
			m.state = "store"
			m.current = 2
			return m, func() tea.Msg {
				stores, err := m.client.ListStores()
				if err != nil {
					return storesLoadedMsg{err: err}
				}
				slugs := make([]string, len(stores))
				for i, s := range stores {
					slugs[i] = s.Slug
				}
				return storesLoadedMsg{stores: slugs}
			}
		}
		return m, cmd

	case "store":
		if m.storeSelect == nil {
			return m, nil // waiting for stores to load
		}
		step, cmd := m.storeSelect.Update(msg)
		m.storeSelect = step.(*components.ItemSelectStep)
		if m.storeSelect.IsComplete() {
			m.store = m.storeSelect.Value().(string)
			m.state = "repo"
			m.current = 3
			return m, func() tea.Msg {
				repos, err := m.client.ListRepos(m.store)
				if err != nil {
					return reposLoadedMsg{err: err}
				}
				names := make([]string, len(repos))
				for i, r := range repos {
					names[i] = r.Name
				}
				return reposLoadedMsg{repos: names}
			}
		}
		return m, cmd

	case "repo":
		if m.repoSelect == nil {
			return m, nil // waiting for repos to load
		}
		step, cmd := m.repoSelect.Update(msg)
		m.repoSelect = step.(*components.SelectStep)
		if m.repoSelect.IsComplete() {
			selected := m.repoSelect.Value().(string)
			if selected == "All repositories" {
				m.repos = nil
			} else {
				m.repos = []string{selected}
			}
			m.state = "perm"
			m.current = 4
			m.permSelect = components.NewSelectStep("Permission", "Choose permission level:", []string{"read", "write"})
			return m, nil
		}
		return m, cmd

	case "perm":
		step, cmd := m.permSelect.Update(msg)
		m.permSelect = step.(*components.SelectStep)
		if m.permSelect.IsComplete() {
			m.permission = m.permSelect.Value().(string)
			m.state = "creating"
			return m, func() tea.Msg {
				// Get store ID from store slug
				store, err := m.client.GetStore(m.store)
				if err != nil {
					return tokenCreatedMsg{err: err}
				}
				resp, err := m.client.CreateScopedToken(m.name, store.ID, m.repos, []string{m.permission}, 0)
				if err != nil {
					return tokenCreatedMsg{err: err}
				}
				return tokenCreatedMsg{key: resp.RawKey}
			}
		}
		return m, cmd
	}

	return m, nil
}

func (m tokenWizardModel) View() string {
	var s strings.Builder

	s.WriteString(tui.TitleStyle.Render("Create Token"))
	s.WriteString("\n")
	s.WriteString(tui.MutedStyle.Render(strings.Repeat("━", 32)))
	s.WriteString("\n")

	switch m.state {
	case "type":
		s.WriteString(fmt.Sprintf("Step 1 of 5: %s\n\n", m.typeSelect.Title()))
		s.WriteString(m.typeSelect.View())

	case "name":
		s.WriteString(fmt.Sprintf("Step 2 of 5: %s\n\n", m.nameInput.Title()))
		s.WriteString(m.nameInput.View())

	case "store":
		s.WriteString("Step 3 of 5: Select Store\n\n")
		if m.storeSelect != nil {
			s.WriteString(m.storeSelect.View())
		} else {
			s.WriteString("Loading stores...")
		}

	case "repo":
		s.WriteString("Step 4 of 5: Select Repository\n\n")
		if m.repoSelect != nil {
			s.WriteString(m.repoSelect.View())
		} else {
			s.WriteString("Loading repositories...")
		}

	case "perm":
		s.WriteString("Step 5 of 5: Select Permission\n\n")
		s.WriteString(m.permSelect.View())

	case "creating":
		s.WriteString(tui.SpinnerStyle.Render("Creating token..."))

	case "done":
		s.WriteString(tui.SuccessStyle.Render("✓ Token created!\n\n"))
		s.WriteString(tui.LabelStyle.Render("Token: "))
		s.WriteString(m.result)
		s.WriteString("\n\n")
		s.WriteString(tui.WarningStyle.Render("Save this token - it won't be shown again!"))

	case "error":
		s.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("✗ Error: %v", m.err)))
	}

	s.WriteString("\n\n")
	s.WriteString(tui.HelpStyle.Render("↑↓ navigate  enter select  esc back"))

	return tui.BoxStyle.Render(s.String())
}

func runTokenWizard(client *api.Client) error {
	m := newTokenWizardModel(client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if tm, ok := finalModel.(tokenWizardModel); ok && tm.err != nil {
		return tm.err
	}
	return nil
}

func newTokenListCmd() *cobra.Command {
	var keysOnly, tokensOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys and scoped tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				result := map[string]any{}

				if !tokensOnly {
					keys, err := client.ListAPIKeys()
					if err != nil {
						return err
					}
					result["api_keys"] = keys
				}

				if !keysOnly {
					tokens, err := client.ListScopedTokens()
					if err != nil {
						return err
					}
					result["scoped_tokens"] = tokens
				}

				outputJSON(result)
				return nil
			}

			// Table output
			if !tokensOnly {
				keys, err := client.ListAPIKeys()
				if err != nil {
					return err
				}

				if len(keys) > 0 {
					fmt.Println("API Keys:")
					headers := []string{"ID", "LABEL", "PREFIX", "CREATED", "LAST USED"}
					rows := make([][]string, len(keys))
					for i, k := range keys {
						lastUsed := "-"
						if k.LastUsedAt != nil {
							lastUsed = formatDateTime(*k.LastUsedAt)
						}
						rows[i] = []string{
							truncate(k.ID, 12),
							k.Label,
							k.KeyPrefix,
							formatDate(k.CreatedAt),
							lastUsed,
						}
					}

					// Use interactive table if available
					if isInteractive() {
						selected, err := outputInteractiveTable("API Keys", headers, rows)
						if err != nil {
							return err
						}
						if selected != nil {
							// Copy full ID to show user what was selected
							for _, k := range keys {
								if truncate(k.ID, 12) == selected[0] {
									fmt.Printf("\nSelected: %s (ID: %s)\n", k.Label, k.ID)
									break
								}
							}
						}
					} else {
						outputTable(headers, rows)
					}
					fmt.Println()
				}
			}

			if !keysOnly {
				tokens, err := client.ListScopedTokens()
				if err != nil {
					return err
				}

				if len(tokens) > 0 {
					fmt.Println("Scoped Tokens:")
					headers := []string{"ID", "LABEL", "PERMISSIONS", "CREATED", "EXPIRES"}
					rows := make([][]string, len(tokens))
					for i, t := range tokens {
						expires := "-"
						if t.ExpiresAt != nil {
							expires = formatDate(*t.ExpiresAt)
						}
						rows[i] = []string{
							truncate(t.ID, 12),
							t.Label,
							strings.Join(t.Scope.Permissions, ","),
							formatDate(t.CreatedAt),
							expires,
						}
					}

					// Use interactive table if available
					if isInteractive() {
						selected, err := outputInteractiveTable("Scoped Tokens", headers, rows)
						if err != nil {
							return err
						}
						if selected != nil {
							for _, t := range tokens {
								if truncate(t.ID, 12) == selected[0] {
									fmt.Printf("\nSelected: %s (ID: %s)\n", t.Label, t.ID)
									break
								}
							}
						}
					} else {
						outputTable(headers, rows)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&keysOnly, "keys", false, "Show only API keys")
	cmd.Flags().BoolVar(&tokensOnly, "tokens", false, "Show only scoped tokens")

	return cmd
}

func newTokenRevokeCmd() *cobra.Command {
	var isToken, force bool

	cmd := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API key or scoped token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			// Confirm revocation
			if !force && isInteractive() {
				tokenType := "API key"
				if isToken {
					tokenType = "scoped token"
				}
				confirmed, err := components.RunConfirm(
					"Revoke Token",
					fmt.Sprintf("Are you sure you want to revoke this %s?\nID: %s", tokenType, id),
					true,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					info("Revocation cancelled")
					return nil
				}
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			if isToken {
				if err := client.RevokeScopedToken(id); err != nil {
					return err
				}
				success("Scoped token revoked")
			} else {
				if err := client.RevokeAPIKey(id); err != nil {
					return err
				}
				success("API key revoked")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&isToken, "token", false, "Revoke scoped token instead of API key")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
