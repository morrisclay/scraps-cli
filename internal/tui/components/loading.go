package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/scraps-sh/scraps-cli/internal/tui"
)

// LoadingModel is a loading spinner component.
type LoadingModel struct {
	spinner spinner.Model
	message string
	done    bool
	err     error
	result  any
}

// LoadingDoneMsg is sent when loading completes.
type LoadingDoneMsg struct {
	Result any
	Err    error
}

// NewLoading creates a new loading spinner.
func NewLoading(message string) LoadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = tui.SpinnerStyle
	return LoadingModel{
		spinner: s,
		message: message,
	}
}

// Init implements tea.Model.
func (m LoadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model.
func (m LoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case LoadingDoneMsg:
		m.done = true
		m.result = msg.Result
		m.err = msg.Err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m LoadingModel) View() string {
	if m.done {
		return ""
	}
	return m.spinner.View() + " " + m.message
}

// SetMessage updates the loading message.
func (m *LoadingModel) SetMessage(msg string) {
	m.message = msg
}

// Done returns whether loading is complete.
func (m LoadingModel) Done() bool {
	return m.done
}

// Error returns any error that occurred.
func (m LoadingModel) Error() error {
	return m.err
}

// Result returns the result of the loading operation.
func (m LoadingModel) Result() any {
	return m.result
}

// RunWithLoading runs a function with a loading spinner.
func RunWithLoading[T any](message string, fn func() (T, error)) (T, error) {
	m := NewLoading(message)

	var result T
	var err error

	p := tea.NewProgram(m)

	// Run the function in a goroutine
	go func() {
		result, err = fn()
		p.Send(LoadingDoneMsg{Result: result, Err: err})
	}()

	if _, runErr := p.Run(); runErr != nil {
		return result, runErr
	}

	return result, err
}
