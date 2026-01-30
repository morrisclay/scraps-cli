package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// TextareaModel is a multi-line text input component.
type TextareaModel struct {
	textarea    textarea.Model
	title       string
	prompt      string
	help        HelpModel
	showHelp    bool
	done        bool
	cancelled   bool
	value       string
	charLimit   int
	lineLimit   int
	width       int
	height      int
}

// TextareaSubmitMsg is sent when the textarea content is submitted.
type TextareaSubmitMsg struct {
	Value string
}

// TextareaCancelMsg is sent when the textarea is cancelled.
type TextareaCancelMsg struct{}

// NewTextarea creates a new multi-line text input.
func NewTextarea(title, prompt, placeholder string) TextareaModel {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.CharLimit = 1000
	ta.SetWidth(50)
	ta.SetHeight(5)
	ta.Focus()

	// Style the textarea
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(tui.ColorPrimary)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorPrimary).
		Padding(0, 1)

	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorBorder).
		Padding(0, 1)

	return TextareaModel{
		textarea:  ta,
		title:     title,
		prompt:    prompt,
		help:      NewHelp(DefaultTextareaKeyMap()),
		charLimit: 1000,
	}
}

// WithCharLimit sets the character limit.
func (m TextareaModel) WithCharLimit(limit int) TextareaModel {
	m.charLimit = limit
	m.textarea.CharLimit = limit
	return m
}

// WithLineLimit sets the line limit.
func (m TextareaModel) WithLineLimit(limit int) TextareaModel {
	m.lineLimit = limit
	return m
}

// WithSize sets the textarea dimensions.
func (m TextareaModel) WithSize(width, height int) TextareaModel {
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
	return m
}

// Init implements tea.Model.
func (m TextareaModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model.
func (m TextareaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 8)
		m.help.SetWidth(msg.Width)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
			if !m.textarea.Focused() {
				m.showHelp = !m.showHelp
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
			// Submit
			m.value = m.textarea.Value()
			m.done = true
			return m, func() tea.Msg {
				return TextareaSubmitMsg{Value: m.value}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.cancelled = true
			m.done = true
			return m, func() tea.Msg {
				return TextareaCancelMsg{}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}

		// Check line limit
		if m.lineLimit > 0 {
			lines := strings.Count(m.textarea.Value(), "\n") + 1
			if lines >= m.lineLimit && msg.String() == "enter" {
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update help
	var helpCmd tea.Cmd
	m.help, helpCmd = m.help.Update(msg)
	cmds = append(cmds, helpCmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m TextareaModel) View() string {
	if m.done {
		return ""
	}

	var s strings.Builder

	// Title
	if m.title != "" {
		s.WriteString(tui.TitleStyle.Render(m.title))
		s.WriteString("\n\n")
	}

	// Prompt
	if m.prompt != "" {
		s.WriteString(m.prompt)
		s.WriteString("\n\n")
	}

	// Textarea
	s.WriteString(m.textarea.View())
	s.WriteString("\n")

	// Character count
	charCount := len(m.textarea.Value())
	countStyle := tui.MutedStyle
	if charCount > m.charLimit*9/10 {
		countStyle = tui.WarningStyle
	}
	if charCount >= m.charLimit {
		countStyle = tui.ErrorStyle
	}
	s.WriteString(countStyle.Render(
		strings.Repeat(" ", m.textarea.Width()-10) +
			string(rune('0'+charCount/1000%10)) +
			string(rune('0'+charCount/100%10)) +
			string(rune('0'+charCount/10%10)) +
			string(rune('0'+charCount%10)) +
			"/" +
			string(rune('0'+m.charLimit/1000%10)) +
			string(rune('0'+m.charLimit/100%10)) +
			string(rune('0'+m.charLimit/10%10)) +
			string(rune('0'+m.charLimit%10)),
	))
	s.WriteString("\n\n")

	// Help
	if m.showHelp {
		s.WriteString(m.help.FullView())
	} else {
		s.WriteString(tui.HelpStyle.Render("ctrl+d submit  esc cancel  ? help"))
	}

	return s.String()
}

// Value returns the current textarea value.
func (m TextareaModel) Value() string {
	return m.textarea.Value()
}

// Done returns whether input is complete.
func (m TextareaModel) Done() bool {
	return m.done
}

// Cancelled returns whether input was cancelled.
func (m TextareaModel) Cancelled() bool {
	return m.cancelled
}

// RunTextarea runs a textarea and returns the entered text.
func RunTextarea(title, prompt, placeholder string) (string, error) {
	m := NewTextarea(title, prompt, placeholder)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	if tm, ok := finalModel.(TextareaModel); ok {
		if tm.Cancelled() {
			return "", nil
		}
		return tm.Value(), nil
	}

	return "", nil
}

// RunTextareaInline runs a textarea inline (without alt screen).
func RunTextareaInline(title, prompt, placeholder string) (string, error) {
	m := NewTextarea(title, prompt, placeholder)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	if tm, ok := finalModel.(TextareaModel); ok {
		if tm.Cancelled() {
			return "", nil
		}
		return tm.Value(), nil
	}

	return "", nil
}
