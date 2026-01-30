// Package tui provides TUI components and styles for the scraps CLI.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors for the TUI theme.
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
)

// Styles for common TUI elements.
var (
	// Title style for section headers
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginBottom(1)

	// Box style for bordered containers
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	// Focused box style
	FocusedBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// Error message style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	// Warning message style
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// Muted text style
	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Help text style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)

	// Selected item style for lists
	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Unselected item style for lists
	UnselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Label style for form fields
	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	// Value style for displaying values
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Prompt style for input prompts
	PromptStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	// Cursor style
	CursorStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// StatusBar style
	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	// Connected indicator style
	ConnectedStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	// Disconnected indicator style
	DisconnectedStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// Tree directory style
	DirStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	// Tree file style
	FileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)
)

// WizardStepStyle returns style for wizard step indicators.
func WizardStepStyle(current, step int) lipgloss.Style {
	if step < current {
		return lipgloss.NewStyle().Foreground(ColorSuccess)
	} else if step == current {
		return lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(ColorMuted)
}

// ProgressBarStyle creates a progress bar style.
func ProgressBarStyle(width int, percent float64) string {
	filled := int(float64(width) * percent)
	empty := width - filled

	filledBar := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Render(repeat("█", filled))
	emptyBar := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(repeat("░", empty))

	return filledBar + emptyBar
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
