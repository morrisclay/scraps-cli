package components

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// TableModel is an interactive table component.
type TableModel struct {
	table      table.Model
	title      string
	done       bool
	cancelled  bool
	selected   table.Row
	showHelp   bool
	width      int
	height     int
	onSelect   func(row table.Row) tea.Cmd
}

// TableSelectedMsg is sent when a row is selected.
type TableSelectedMsg struct {
	Row table.Row
}

// TableColumn defines a column in the table.
type TableColumn struct {
	Title string
	Width int
}

// NewTable creates a new interactive table.
func NewTable(title string, columns []TableColumn, rows []table.Row) TableModel {
	cols := make([]table.Column, len(columns))
	for i, c := range columns {
		cols[i] = table.Column{
			Title: c.Title,
			Width: c.Width,
		}
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Apply custom styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(tui.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(tui.ColorPrimary)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(tui.ColorPrimary).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(lipgloss.Color("#FFFFFF"))
	t.SetStyles(s)

	return TableModel{
		table: t,
		title: title,
	}
}

// WithHeight sets the table height.
func (m TableModel) WithHeight(height int) TableModel {
	m.table.SetHeight(height)
	return m
}

// WithWidth sets the table width.
func (m TableModel) WithWidth(width int) TableModel {
	m.table.SetWidth(width)
	return m
}

// WithOnSelect sets a callback for when a row is selected.
func (m TableModel) WithOnSelect(fn func(row table.Row) tea.Cmd) TableModel {
	m.onSelect = fn
	return m
}

// Init implements tea.Model.
func (m TableModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m TableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust table size to fit within window
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 8)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			m.selected = m.table.SelectedRow()
			m.done = true
			if m.onSelect != nil {
				return m, m.onSelect(m.selected)
			}
			return m, func() tea.Msg {
				return TableSelectedMsg{Row: m.selected}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.cancelled = true
			m.done = true
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m TableModel) View() string {
	if m.done {
		return ""
	}

	var s string

	// Title
	if m.title != "" {
		s += tui.TitleStyle.Render(m.title) + "\n\n"
	}

	// Table
	s += m.table.View() + "\n\n"

	// Help
	if m.showHelp {
		s += tui.HelpStyle.Render("↑/k up  ↓/j down  enter select  esc quit  ? toggle help")
	} else {
		s += tui.HelpStyle.Render("↑↓ navigate  enter select  ? help")
	}

	return s
}

// Selected returns the selected row, if any.
func (m TableModel) Selected() table.Row {
	return m.selected
}

// Done returns whether the table selection is complete.
func (m TableModel) Done() bool {
	return m.done
}

// Cancelled returns whether the selection was cancelled.
func (m TableModel) Cancelled() bool {
	return m.cancelled
}

// Table returns the underlying table model for direct access.
func (m TableModel) Table() table.Model {
	return m.table
}

// RunTable runs an interactive table and returns the selected row.
func RunTable(title string, columns []TableColumn, rows []table.Row) (table.Row, error) {
	m := NewTable(title, columns, rows)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if tm, ok := finalModel.(TableModel); ok {
		if tm.Cancelled() {
			return nil, nil
		}
		return tm.Selected(), nil
	}

	return nil, nil
}

// RunTableInline runs a table without alt screen (inline in terminal).
func RunTableInline(title string, columns []TableColumn, rows []table.Row) (table.Row, error) {
	m := NewTable(title, columns, rows).WithHeight(min(len(rows)+2, 15))
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if tm, ok := finalModel.(TableModel); ok {
		if tm.Cancelled() {
			return nil, nil
		}
		return tm.Selected(), nil
	}

	return nil, nil
}
