// Package components provides reusable TUI components.
package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// ConfirmModel is a confirmation dialog component.
type ConfirmModel struct {
	Title       string
	Message     string
	ConfirmText string
	CancelText  string
	Destructive bool

	confirmed bool
	selected  int // 0 = confirm, 1 = cancel
	done      bool
}

// ConfirmMsg is sent when the user makes a selection.
type ConfirmMsg struct {
	Confirmed bool
}

// NewConfirm creates a new confirmation dialog.
func NewConfirm(title, message string) ConfirmModel {
	return ConfirmModel{
		Title:       title,
		Message:     message,
		ConfirmText: "Yes",
		CancelText:  "Cancel",
		Destructive: false,
		selected:    1, // Default to Cancel for safety
	}
}

// NewDestructiveConfirm creates a confirmation dialog for destructive actions.
func NewDestructiveConfirm(title, message string) ConfirmModel {
	return ConfirmModel{
		Title:       title,
		Message:     message,
		ConfirmText: "Yes, Delete",
		CancelText:  "Cancel",
		Destructive: true,
		selected:    1, // Default to Cancel
	}
}

// Init implements tea.Model.
func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			m.selected = 0
		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			m.selected = 1
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			m.selected = (m.selected + 1) % 2
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			m.confirmed = m.selected == 0
			m.done = true
			return m, func() tea.Msg {
				return ConfirmMsg{Confirmed: m.confirmed}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
			m.confirmed = true
			m.done = true
			return m, func() tea.Msg {
				return ConfirmMsg{Confirmed: true}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("n", "N", "esc"))):
			m.confirmed = false
			m.done = true
			return m, func() tea.Msg {
				return ConfirmMsg{Confirmed: false}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m ConfirmModel) View() string {
	if m.done {
		return ""
	}

	var s string

	// Title
	titleStyle := tui.TitleStyle
	if m.Destructive {
		titleStyle = titleStyle.Foreground(tui.ColorError)
	}
	s += titleStyle.Render(m.Title) + "\n\n"

	// Message
	s += m.Message + "\n\n"

	// Buttons
	confirmStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder())
	cancelStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder())

	if m.selected == 0 {
		if m.Destructive {
			confirmStyle = confirmStyle.
				BorderForeground(tui.ColorError).
				Foreground(tui.ColorError).
				Bold(true)
		} else {
			confirmStyle = confirmStyle.
				BorderForeground(tui.ColorPrimary).
				Foreground(tui.ColorPrimary).
				Bold(true)
		}
		cancelStyle = cancelStyle.BorderForeground(tui.ColorMuted)
	} else {
		confirmStyle = confirmStyle.BorderForeground(tui.ColorMuted)
		cancelStyle = cancelStyle.
			BorderForeground(tui.ColorPrimary).
			Foreground(tui.ColorPrimary).
			Bold(true)
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		confirmStyle.Render(m.ConfirmText),
		"  ",
		cancelStyle.Render(m.CancelText),
	)
	s += buttons + "\n\n"

	// Help
	s += tui.HelpStyle.Render("←→ select  enter confirm  y/n quick select  esc cancel")

	return tui.BoxStyle.Render(s)
}

// Confirmed returns whether the user confirmed the action.
func (m ConfirmModel) Confirmed() bool {
	return m.confirmed
}

// Done returns whether the dialog is complete.
func (m ConfirmModel) Done() bool {
	return m.done
}

// RunConfirm runs a confirmation dialog and returns the result.
func RunConfirm(title, message string, destructive bool) (bool, error) {
	var m ConfirmModel
	if destructive {
		m = NewDestructiveConfirm(title, message)
	} else {
		m = NewConfirm(title, message)
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	if cm, ok := finalModel.(ConfirmModel); ok {
		return cm.Confirmed(), nil
	}

	return false, fmt.Errorf("unexpected model type")
}
