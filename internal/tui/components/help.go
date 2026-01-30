package components

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// HelpKeyMap defines the interface for components that provide help.
type HelpKeyMap interface {
	help.KeyMap
}

// HelpModel is a help overlay component.
type HelpModel struct {
	help     help.Model
	keyMap   HelpKeyMap
	showFull bool
}

// NewHelp creates a new help component.
func NewHelp(keyMap HelpKeyMap) HelpModel {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(tui.ColorBorder)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(tui.ColorBorder)

	return HelpModel{
		help:   h,
		keyMap: keyMap,
	}
}

// Init implements tea.Model.
func (m HelpModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("?"))) {
			m.showFull = !m.showFull
		}
	case tea.WindowSizeMsg:
		m.help.Width = msg.Width
	}
	return m, nil
}

// View returns the help view.
func (m HelpModel) View() string {
	if m.showFull {
		return m.help.View(m.keyMap)
	}
	return m.help.View(m.keyMap)
}

// ShortView returns the short help view.
func (m HelpModel) ShortView() string {
	return m.help.ShortHelpView(m.keyMap.ShortHelp())
}

// FullView returns the full help view.
func (m HelpModel) FullView() string {
	return m.help.FullHelpView(m.keyMap.FullHelp())
}

// ShowFull returns whether full help is shown.
func (m HelpModel) ShowFull() bool {
	return m.showFull
}

// SetShowFull sets whether to show full help.
func (m *HelpModel) SetShowFull(show bool) {
	m.showFull = show
}

// Toggle toggles between short and full help.
func (m *HelpModel) Toggle() {
	m.showFull = !m.showFull
}

// SetWidth sets the help width.
func (m *HelpModel) SetWidth(width int) {
	m.help.Width = width
}

// --- Common Key Maps ---

// TableKeyMap defines keybindings for table components.
type TableKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Quit   key.Binding
	Help   key.Binding
}

// ShortHelp implements HelpKeyMap.
func (k TableKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Help}
}

// FullHelp implements HelpKeyMap.
func (k TableKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Quit, k.Help},
	}
}

// DefaultTableKeyMap returns the default table keybindings.
func DefaultTableKeyMap() TableKeyMap {
	return TableKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// ListKeyMap defines keybindings for list/searchlist components.
type ListKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Filter key.Binding
	Quit   key.Binding
	Help   key.Binding
}

// ShortHelp implements HelpKeyMap.
func (k ListKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter}
}

// FullHelp implements HelpKeyMap.
func (k ListKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Filter},
		{k.Quit, k.Help},
	}
}

// DefaultListKeyMap returns the default list keybindings.
func DefaultListKeyMap() ListKeyMap {
	return ListKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// WizardKeyMap defines keybindings for wizard components.
type WizardKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
	Help  key.Binding
}

// ShortHelp implements HelpKeyMap.
func (k WizardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back}
}

// FullHelp implements HelpKeyMap.
func (k WizardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Back},
		{k.Quit, k.Help},
	}
}

// DefaultWizardKeyMap returns the default wizard keybindings.
func DefaultWizardKeyMap() WizardKeyMap {
	return WizardKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

// TextareaKeyMap defines keybindings for textarea components.
type TextareaKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
	Help   key.Binding
}

// ShortHelp implements HelpKeyMap.
func (k TextareaKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Cancel, k.Help}
}

// FullHelp implements HelpKeyMap.
func (k TextareaKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Submit, k.Cancel, k.Help},
	}
}

// DefaultTextareaKeyMap returns the default textarea keybindings.
func DefaultTextareaKeyMap() TextareaKeyMap {
	return TextareaKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}
