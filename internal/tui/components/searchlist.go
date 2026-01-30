package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

// SearchListItem represents an item in the searchable list.
type SearchListItem struct {
	title       string
	description string
	value       any
}

// NewSearchListItem creates a new list item.
func NewSearchListItem(title, description string, value any) SearchListItem {
	return SearchListItem{
		title:       title,
		description: description,
		value:       value,
	}
}

// FilterValue implements list.Item.
func (i SearchListItem) FilterValue() string { return i.title + " " + i.description }

// Title returns the item title.
func (i SearchListItem) Title() string { return i.title }

// Description returns the item description.
func (i SearchListItem) Description() string { return i.description }

// Value returns the item's associated value.
func (i SearchListItem) Value() any { return i.value }

// SearchListModel is a searchable list component.
type SearchListModel struct {
	list       list.Model
	filterMode bool
	filter     textinput.Model
	title      string
	items      []SearchListItem
	selected   *SearchListItem
	done       bool
	cancelled  bool
	width      int
	height     int
}

// SearchListSelectedMsg is sent when an item is selected.
type SearchListSelectedMsg struct {
	Item SearchListItem
}

// NewSearchList creates a new searchable list.
func NewSearchList(title string, items []SearchListItem) SearchListModel {
	// Convert items to list.Item
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Create delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(tui.ColorPrimary).
		BorderLeftForeground(tui.ColorPrimary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(tui.ColorMuted).
		BorderLeftForeground(tui.ColorPrimary)

	l := list.New(listItems, delegate, 40, 15)
	l.Title = title
	l.Styles.Title = tui.TitleStyle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Create filter input
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	ti.Width = 30
	ti.PromptStyle = tui.PromptStyle
	ti.TextStyle = lipgloss.NewStyle()

	return SearchListModel{
		list:   l,
		filter: ti,
		title:  title,
		items:  items,
		width:  40,
		height: 15,
	}
}

// Init implements tea.Model.
func (m SearchListModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SearchListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-4)

	case tea.KeyMsg:
		if m.filterMode {
			switch msg.String() {
			case "esc":
				m.filterMode = false
				m.filter.SetValue("")
				m.filterItems("")
			case "enter":
				m.filterMode = false
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				cmds = append(cmds, cmd)
				m.filterItems(m.filter.Value())
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			m.filterMode = true
			m.filter.Focus()
			return m, textinput.Blink

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if item, ok := m.list.SelectedItem().(SearchListItem); ok {
				m.selected = &item
				m.done = true
				return m, func() tea.Msg {
					return SearchListSelectedMsg{Item: item}
				}
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
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// filterItems filters the list based on the query.
func (m *SearchListModel) filterItems(query string) {
	if query == "" {
		listItems := make([]list.Item, len(m.items))
		for i, item := range m.items {
			listItems[i] = item
		}
		m.list.SetItems(listItems)
		return
	}

	query = strings.ToLower(query)
	var filtered []list.Item
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.FilterValue()), query) {
			filtered = append(filtered, item)
		}
	}
	m.list.SetItems(filtered)
}

// View implements tea.Model.
func (m SearchListModel) View() string {
	if m.done {
		return ""
	}

	var s strings.Builder

	if m.filterMode {
		s.WriteString("Filter: ")
		s.WriteString(m.filter.View())
		s.WriteString("\n\n")
	}

	s.WriteString(m.list.View())

	return s.String()
}

// Selected returns the selected item, if any.
func (m SearchListModel) Selected() *SearchListItem {
	return m.selected
}

// Done returns whether the list selection is complete.
func (m SearchListModel) Done() bool {
	return m.done
}

// Cancelled returns whether the selection was cancelled.
func (m SearchListModel) Cancelled() bool {
	return m.cancelled
}

// RunSearchList runs a searchable list and returns the selected item.
func RunSearchList(title string, items []SearchListItem) (*SearchListItem, error) {
	m := NewSearchList(title, items)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if sm, ok := finalModel.(SearchListModel); ok {
		if sm.Cancelled() {
			return nil, nil
		}
		return sm.Selected(), nil
	}

	return nil, nil
}
