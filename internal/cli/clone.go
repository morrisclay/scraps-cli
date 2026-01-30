package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

func newCloneCmd() *cobra.Command {
	var urlOnly bool

	cmd := &cobra.Command{
		Use:     "clone <store/repo> [directory]",
		Short:   "Clone a repository",
		Example: "  scraps clone mystore/myrepo\n  scraps clone mystore/myrepo ./local-dir",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps clone <store/repo> [directory]\n\nExample: scraps clone mystore/myrepo")
			}
			if len(args) > 2 {
				return fmt.Errorf("too many arguments\n\nUsage: scraps clone <store/repo> [directory]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, err := parseStoreRepo(args[0])
			if err != nil {
				return err
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			cloneURL := client.GetCloneURL(store, repo)

			if urlOnly {
				fmt.Println(cloneURL)
				return nil
			}

			dir := repo
			if len(args) > 1 {
				dir = args[1]
			}

			// Interactive mode with progress
			if isInteractive() {
				return runCloneTUI(cloneURL, dir)
			}

			// Non-interactive mode
			gitCmd := exec.Command("git", "clone", cloneURL, dir)
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("git clone failed: %w", err)
			}

			success(fmt.Sprintf("Cloned to %s", dir))
			return nil
		},
	}

	cmd.Flags().BoolVar(&urlOnly, "url-only", false, "Print clone URL without cloning")
	return cmd
}

// cloneModel is the TUI model for cloning.
type cloneModel struct {
	url      string
	dir      string
	spinner  spinner.Model
	progress progress.Model
	state    string // "cloning", "done", "error"
	err      error
}

func newCloneModel(url, dir string) cloneModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = tui.SpinnerStyle

	p := progress.New(progress.WithDefaultGradient())

	return cloneModel{
		url:      url,
		dir:      dir,
		spinner:  s,
		progress: p,
		state:    "cloning",
	}
}

type cloneCompleteMsg struct {
	err error
}

func (m cloneModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			gitCmd := exec.Command("git", "clone", m.url, m.dir)
			err := gitCmd.Run()
			return cloneCompleteMsg{err: err}
		},
	)
}

func (m cloneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case cloneCompleteMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
		} else {
			m.state = "done"
		}
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == "cloning" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m cloneModel) View() string {
	switch m.state {
	case "cloning":
		return m.spinner.View() + fmt.Sprintf(" Cloning to %s...", m.dir)
	case "done":
		return tui.SuccessStyle.Render("✓") + fmt.Sprintf(" Cloned to %s", m.dir)
	case "error":
		return tui.ErrorStyle.Render("✗") + fmt.Sprintf(" Clone failed: %v", m.err)
	}
	return ""
}

func runCloneTUI(url, dir string) error {
	m := newCloneModel(url, dir)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if cm, ok := finalModel.(cloneModel); ok && cm.err != nil {
		return cm.err
	}
	return nil
}
