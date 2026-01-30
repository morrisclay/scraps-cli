package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// WizardStep represents a step in the wizard.
type WizardStep interface {
	Title() string
	View() string
	Update(msg tea.Msg) (WizardStep, tea.Cmd)
	Init() tea.Cmd
	IsComplete() bool
	Value() any
}

// WizardModel is a multi-step wizard component.
type WizardModel struct {
	title       string
	steps       []WizardStep
	currentStep int
	done        bool
	cancelled   bool
	width       int
	height      int
}

// WizardCompleteMsg is sent when the wizard completes.
type WizardCompleteMsg struct {
	Values []any
}

// WizardCancelledMsg is sent when the wizard is cancelled.
type WizardCancelledMsg struct{}

// NewWizard creates a new multi-step wizard.
func NewWizard(title string, steps []WizardStep) WizardModel {
	return WizardModel{
		title:       title,
		steps:       steps,
		currentStep: 0,
	}
}

// Init implements tea.Model.
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) > 0 {
		return m.steps[0].Init()
	}
	return nil
}

// Update implements tea.Model.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			m.cancelled = true
			m.done = true
			return m, func() tea.Msg { return WizardCancelledMsg{} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.currentStep > 0 {
				m.currentStep--
				return m, m.steps[m.currentStep].Init()
			}
			m.cancelled = true
			m.done = true
			return m, func() tea.Msg { return WizardCancelledMsg{} }
		}
	}

	if m.currentStep < len(m.steps) {
		step, cmd := m.steps[m.currentStep].Update(msg)
		m.steps[m.currentStep] = step

		if step.IsComplete() {
			if m.currentStep < len(m.steps)-1 {
				m.currentStep++
				return m, m.steps[m.currentStep].Init()
			} else {
				m.done = true
				values := make([]any, len(m.steps))
				for i, s := range m.steps {
					values[i] = s.Value()
				}
				return m, func() tea.Msg {
					return WizardCompleteMsg{Values: values}
				}
			}
		}

		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m WizardModel) View() string {
	if m.done {
		return ""
	}

	var s strings.Builder

	// Title
	s.WriteString(tui.TitleStyle.Render(m.title))
	s.WriteString("\n")

	// Progress bar
	progressWidth := 32
	s.WriteString(tui.MutedStyle.Render(strings.Repeat("━", progressWidth)))
	s.WriteString("\n")

	// Step indicator
	s.WriteString(fmt.Sprintf("Step %d of %d: %s\n\n",
		m.currentStep+1,
		len(m.steps),
		m.steps[m.currentStep].Title()))

	// Current step content
	s.WriteString(m.steps[m.currentStep].View())
	s.WriteString("\n\n")

	// Help
	helpText := "↑↓ navigate  enter select"
	if m.currentStep > 0 {
		helpText += "  esc back"
	}
	s.WriteString(tui.HelpStyle.Render(helpText))

	return tui.BoxStyle.Render(s.String())
}

// Done returns whether the wizard is complete.
func (m WizardModel) Done() bool {
	return m.done
}

// Cancelled returns whether the wizard was cancelled.
func (m WizardModel) Cancelled() bool {
	return m.cancelled
}

// Values returns all step values.
func (m WizardModel) Values() []any {
	values := make([]any, len(m.steps))
	for i, s := range m.steps {
		values[i] = s.Value()
	}
	return values
}

// --- Text Input Step ---

// TextInputStep is a wizard step with a text input.
type TextInputStep struct {
	title    string
	prompt   string
	input    textinput.Model
	complete bool
	value    string
}

// NewTextInputStep creates a new text input step.
func NewTextInputStep(title, prompt, placeholder string) *TextInputStep {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 30
	ti.PromptStyle = tui.PromptStyle
	ti.TextStyle = lipgloss.NewStyle()

	return &TextInputStep{
		title:  title,
		prompt: prompt,
		input:  ti,
	}
}

// NewPasswordInputStep creates a password input step.
func NewPasswordInputStep(title, prompt, placeholder string) *TextInputStep {
	step := NewTextInputStep(title, prompt, placeholder)
	step.input.EchoMode = textinput.EchoPassword
	step.input.EchoCharacter = '•'
	return step
}

// Title implements WizardStep.
func (s *TextInputStep) Title() string { return s.title }

// Init implements WizardStep.
func (s *TextInputStep) Init() tea.Cmd {
	s.input.Focus()
	return textinput.Blink
}

// Update implements WizardStep.
func (s *TextInputStep) Update(msg tea.Msg) (WizardStep, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" && s.input.Value() != "" {
			s.complete = true
			s.value = s.input.Value()
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View implements WizardStep.
func (s *TextInputStep) View() string {
	return s.prompt + "\n\n" + s.input.View()
}

// IsComplete implements WizardStep.
func (s *TextInputStep) IsComplete() bool { return s.complete }

// Value implements WizardStep.
func (s *TextInputStep) Value() any { return s.value }

// --- Select Step ---

// SelectStep is a wizard step with a selection list.
type SelectStep struct {
	title    string
	prompt   string
	options  []string
	selected int
	complete bool
}

// NewSelectStep creates a new selection step.
func NewSelectStep(title, prompt string, options []string) *SelectStep {
	return &SelectStep{
		title:   title,
		prompt:  prompt,
		options: options,
	}
}

// Title implements WizardStep.
func (s *SelectStep) Title() string { return s.title }

// Init implements WizardStep.
func (s *SelectStep) Init() tea.Cmd { return nil }

// Update implements WizardStep.
func (s *SelectStep) Update(msg tea.Msg) (WizardStep, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.options)-1 {
				s.selected++
			}
		case "enter":
			s.complete = true
			return s, nil
		}
	}
	return s, nil
}

// View implements WizardStep.
func (s *SelectStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt)
	b.WriteString("\n\n")

	for i, opt := range s.options {
		if i == s.selected {
			b.WriteString(tui.SelectedStyle.Render("> " + opt))
		} else {
			b.WriteString(tui.MutedStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// IsComplete implements WizardStep.
func (s *SelectStep) IsComplete() bool { return s.complete }

// Value implements WizardStep.
func (s *SelectStep) Value() any {
	if s.selected >= 0 && s.selected < len(s.options) {
		return s.options[s.selected]
	}
	return ""
}

// SelectedIndex returns the selected index.
func (s *SelectStep) SelectedIndex() int { return s.selected }

// --- Item Select Step (with values) ---

// ItemSelectStep is a wizard step with items that have associated values.
type ItemSelectStep struct {
	title    string
	prompt   string
	items    []SearchListItem
	selected int
	complete bool
}

// NewItemSelectStep creates a new item selection step.
func NewItemSelectStep(title, prompt string, items []SearchListItem) *ItemSelectStep {
	return &ItemSelectStep{
		title:  title,
		prompt: prompt,
		items:  items,
	}
}

// Title implements WizardStep.
func (s *ItemSelectStep) Title() string { return s.title }

// Init implements WizardStep.
func (s *ItemSelectStep) Init() tea.Cmd { return nil }

// Update implements WizardStep.
func (s *ItemSelectStep) Update(msg tea.Msg) (WizardStep, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.items)-1 {
				s.selected++
			}
		case "enter":
			s.complete = true
			return s, nil
		}
	}
	return s, nil
}

// View implements WizardStep.
func (s *ItemSelectStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt)
	b.WriteString("\n\n")

	for i, item := range s.items {
		if i == s.selected {
			b.WriteString(tui.SelectedStyle.Render("> "+item.Title()) + "\n")
			if item.Description() != "" {
				b.WriteString(tui.MutedStyle.Render("  "+item.Description()) + "\n")
			}
		} else {
			b.WriteString(tui.MutedStyle.Render("  "+item.Title()) + "\n")
		}
	}

	return b.String()
}

// IsComplete implements WizardStep.
func (s *ItemSelectStep) IsComplete() bool { return s.complete }

// Value implements WizardStep.
func (s *ItemSelectStep) Value() any {
	if s.selected >= 0 && s.selected < len(s.items) {
		return s.items[s.selected].Value()
	}
	return nil
}

// SelectedItem returns the selected item.
func (s *ItemSelectStep) SelectedItem() *SearchListItem {
	if s.selected >= 0 && s.selected < len(s.items) {
		return &s.items[s.selected]
	}
	return nil
}

// --- Textarea Step ---

// TextareaStep is a wizard step with multi-line text input.
type TextareaStep struct {
	title       string
	prompt      string
	textarea    textarea.Model
	complete    bool
	value       string
	charLimit   int
	lineLimit   int
}

// NewTextareaStep creates a new textarea step.
func NewTextareaStep(title, prompt, placeholder string) *TextareaStep {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.CharLimit = 1000
	ta.SetWidth(40)
	ta.SetHeight(5)

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

	return &TextareaStep{
		title:     title,
		prompt:    prompt,
		textarea:  ta,
		charLimit: 1000,
	}
}

// WithCharLimit sets the character limit for the textarea.
func (s *TextareaStep) WithCharLimit(limit int) *TextareaStep {
	s.charLimit = limit
	s.textarea.CharLimit = limit
	return s
}

// WithLineLimit sets the line limit for the textarea.
func (s *TextareaStep) WithLineLimit(limit int) *TextareaStep {
	s.lineLimit = limit
	return s
}

// WithSize sets the textarea dimensions.
func (s *TextareaStep) WithSize(width, height int) *TextareaStep {
	s.textarea.SetWidth(width)
	s.textarea.SetHeight(height)
	return s
}

// Title implements WizardStep.
func (s *TextareaStep) Title() string { return s.title }

// Init implements WizardStep.
func (s *TextareaStep) Init() tea.Cmd {
	s.textarea.Focus()
	return textarea.Blink
}

// Update implements WizardStep.
func (s *TextareaStep) Update(msg tea.Msg) (WizardStep, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ctrl+D to submit (allows empty content)
		if msg.String() == "ctrl+d" {
			s.complete = true
			s.value = s.textarea.Value()
			return s, nil
		}

		// Check line limit before allowing new lines
		if s.lineLimit > 0 && msg.String() == "enter" {
			lines := strings.Count(s.textarea.Value(), "\n") + 1
			if lines >= s.lineLimit {
				return s, nil
			}
		}
	}

	var cmd tea.Cmd
	s.textarea, cmd = s.textarea.Update(msg)
	return s, cmd
}

// View implements WizardStep.
func (s *TextareaStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt)
	b.WriteString("\n\n")
	b.WriteString(s.textarea.View())
	b.WriteString("\n")

	// Character count
	charCount := len(s.textarea.Value())
	countStyle := tui.MutedStyle
	if charCount > s.charLimit*9/10 {
		countStyle = tui.WarningStyle
	}
	if charCount >= s.charLimit {
		countStyle = tui.ErrorStyle
	}
	b.WriteString(countStyle.Render(fmt.Sprintf("%d/%d", charCount, s.charLimit)))
	b.WriteString("\n\n")
	b.WriteString(tui.MutedStyle.Render("ctrl+d to continue"))

	return b.String()
}

// IsComplete implements WizardStep.
func (s *TextareaStep) IsComplete() bool { return s.complete }

// Value implements WizardStep.
func (s *TextareaStep) Value() any { return s.value }
