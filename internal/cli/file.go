package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/model"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

func newFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "File operations",
	}

	cmd.AddCommand(newFileTreeCmd())
	cmd.AddCommand(newFileReadCmd())

	return cmd
}

// --- File Tree Command ---

func newFileTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tree <store/repo[:branch]> [path]",
		Short:   "List files in a repository",
		Example: "  scraps file tree mystore/myrepo\n  scraps file tree mystore/myrepo:main src/",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps file tree <store/repo[:branch]> [path]\n\nExample: scraps file tree mystore/myrepo")
			}
			if len(args) > 2 {
				return fmt.Errorf("too many arguments\n\nUsage: scraps file tree <store/repo[:branch]> [path]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, branch, _, err := parseStoreRepoBranchPath(args[0] + ":")
			if err != nil {
				// Try parsing as store/repo:branch
				store, repo, branch, err = parseStoreRepoBranch(args[0])
				if err != nil {
					return err
				}
			}

			path := ""
			if len(args) > 1 {
				path = args[1]
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			// If interactive, launch tree browser
			if isInteractive() && config.GetOutputFormat() != "json" {
				return runTreeBrowser(client, store, repo, branch, path)
			}

			// Non-interactive: just list
			entries, err := client.GetFileTree(store, repo, branch, path)
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(entries)
			} else {
				headers := []string{"TYPE", "NAME", "SHA"}
				rows := make([][]string, len(entries))
				for i, e := range entries {
					sha := ""
					if e.SHA != "" {
						sha = e.SHA[:8]
					}
					rows[i] = []string{e.Type, e.Name, sha}
				}
				outputTable(headers, rows)
			}
			return nil
		},
	}
	return cmd
}

// treeBrowserModel is the TUI model for the file tree browser.
type treeBrowserModel struct {
	client   *api.Client
	store    string
	repo     string
	branch   string
	path     []string
	entries  []model.FileTreeEntry
	cursor   int
	loading  bool
	err      error
	width    int
	height   int
}

func newTreeBrowserModel(client *api.Client, store, repo, branch, path string) treeBrowserModel {
	var pathParts []string
	if path != "" {
		pathParts = strings.Split(path, "/")
	}
	return treeBrowserModel{
		client:  client,
		store:   store,
		repo:    repo,
		branch:  branch,
		path:    pathParts,
		loading: true,
	}
}

type treeLoadedMsg struct {
	entries []model.FileTreeEntry
	err     error
}

func (m treeBrowserModel) Init() tea.Cmd {
	return m.loadTree()
}

func (m treeBrowserModel) loadTree() tea.Cmd {
	return func() tea.Msg {
		path := strings.Join(m.path, "/")
		entries, err := m.client.GetFileTree(m.store, m.repo, m.branch, path)
		return treeLoadedMsg{entries: entries, err: err}
	}
}

func (m treeBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "right", "l"))):
			if m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				if entry.Type == "tree" {
					m.path = append(m.path, entry.Name)
					m.loading = true
					m.cursor = 0
					return m, m.loadTree()
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "left", "h", "backspace"))):
			if len(m.path) > 0 {
				m.path = m.path[:len(m.path)-1]
				m.loading = true
				m.cursor = 0
				return m, m.loadTree()
			}
		}

	case treeLoadedMsg:
		m.loading = false
		m.entries = msg.entries
		m.err = msg.err
	}

	return m, nil
}

func (m treeBrowserModel) View() string {
	var s strings.Builder

	// Header
	title := fmt.Sprintf("%s/%s:%s", m.store, m.repo, m.branch)
	s.WriteString(tui.TitleStyle.Render(title))
	s.WriteString("\n")

	// Current path
	if len(m.path) > 0 {
		s.WriteString(tui.MutedStyle.Render("/" + strings.Join(m.path, "/")))
		s.WriteString("\n")
	}
	s.WriteString(strings.Repeat("─", 40))
	s.WriteString("\n")

	if m.loading {
		s.WriteString(tui.SpinnerStyle.Render("Loading..."))
		s.WriteString("\n")
	} else if m.err != nil {
		s.WriteString(tui.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n")
	} else if len(m.entries) == 0 {
		s.WriteString(tui.MutedStyle.Render("(empty directory)"))
		s.WriteString("\n")
	} else {
		for i, entry := range m.entries {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			var icon, name string
			if entry.Type == "tree" {
				if i == m.cursor {
					icon = "▼ "
				} else {
					icon = "▸ "
				}
				name = tui.DirStyle.Render(entry.Name + "/")
			} else {
				icon = "  "
				name = tui.FileStyle.Render(entry.Name)
			}

			if i == m.cursor {
				s.WriteString(tui.SelectedStyle.Render(cursor))
			} else {
				s.WriteString(cursor)
			}
			s.WriteString(icon)
			s.WriteString(name)
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(tui.HelpStyle.Render("↑↓ navigate  enter expand  esc back  q quit"))

	return s.String()
}

func runTreeBrowser(client *api.Client, store, repo, branch, path string) error {
	m := newTreeBrowserModel(client, store, repo, branch, path)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- File Read Command ---

func newFileReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "read <store/repo:branch:path>",
		Short:   "Read file contents",
		Example: "  scraps file read mystore/myrepo:main:README.md\n  scraps file read mystore/myrepo:main:src/index.ts",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("file reference required\n\nUsage: scraps file read <store/repo:branch:path>\n\nExample: scraps file read mystore/myrepo:main:README.md")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, branch, path, err := parseStoreRepoBranchPath(args[0])
			if err != nil {
				return err
			}

			if path == "" {
				return fmt.Errorf("file path is required")
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			content, err := client.GetFileContent(store, repo, branch, path)
			if err != nil {
				return err
			}

			// If interactive and content is large, use viewport
			if isInteractive() && len(content) > 2000 {
				return runFileViewer(string(content), path)
			}

			// Just output the content
			fmt.Print(string(content))
			return nil
		},
	}
	return cmd
}

// fileViewerModel is a scrollable file viewer.
type fileViewerModel struct {
	viewport viewport.Model
	filename string
	content  string
	ready    bool
}

func newFileViewerModel(content, filename string) fileViewerModel {
	return fileViewerModel{
		content:  content,
		filename: filename,
	}
}

func (m fileViewerModel) Init() tea.Cmd {
	return nil
}

func (m fileViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 2

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}

		// Add line numbers
		lines := strings.Split(m.content, "\n")
		var numberedLines []string
		for i, line := range lines {
			lineNum := lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(fmt.Sprintf("%4d ", i+1))
			numberedLines = append(numberedLines, lineNum+line)
		}
		m.viewport.SetContent(strings.Join(numberedLines, "\n"))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m fileViewerModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	header := tui.TitleStyle.Render(m.filename) + "\n" + strings.Repeat("─", m.viewport.Width) + "\n"
	footer := "\n" + tui.HelpStyle.Render(fmt.Sprintf("↑↓ scroll  q quit  %d%%", int(m.viewport.ScrollPercent()*100)))

	return header + m.viewport.View() + footer
}

func runFileViewer(content, filename string) error {
	m := newFileViewerModel(content, filename)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
