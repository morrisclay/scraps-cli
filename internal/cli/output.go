package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/tui/components"
	"golang.org/x/term"
)

// isInteractive returns true if stdout is a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// isInputInteractive returns true if stdin is a terminal.
func isInputInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// outputJSON outputs data as formatted JSON.
func outputJSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

// outputTable outputs data as a table.
func outputTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
		}
		headerLine += fmt.Sprintf("%-*s", widths[i], h)
	}
	fmt.Println(headerLine)

	// Print separator
	sepLine := ""
	for i, w := range widths {
		if i > 0 {
			sepLine += "  "
		}
		sepLine += strings.Repeat("-", w)
	}
	fmt.Println(sepLine)

	// Print rows
	for _, row := range rows {
		rowLine := ""
		for i := range headers {
			if i > 0 {
				rowLine += "  "
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			rowLine += fmt.Sprintf("%-*s", widths[i], cell)
		}
		fmt.Println(rowLine)
	}
}

// output outputs data as JSON or table based on config.
func output(data any, headers []string, rows [][]string) {
	format := config.GetOutputFormat()
	if format == "json" {
		outputJSON(data)
	} else {
		outputTable(headers, rows)
	}
}

// formatDate formats a date string for display.
func formatDate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 02, 2006")
}

// formatDateTime formats a datetime string for display.
func formatDateTime(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 02, 2006 15:04")
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	return t.Format("15:04:05")
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// outputInteractiveTable outputs data as an interactive table with selection.
// Returns the selected row, or nil if cancelled.
func outputInteractiveTable(title string, headers []string, rows [][]string) (table.Row, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Cap column widths
	maxWidth := 40
	for i := range widths {
		if widths[i] > maxWidth {
			widths[i] = maxWidth
		}
	}

	// Create table columns
	columns := make([]components.TableColumn, len(headers))
	for i, h := range headers {
		columns[i] = components.TableColumn{
			Title: h,
			Width: widths[i],
		}
	}

	// Convert rows to table.Row
	tableRows := make([]table.Row, len(rows))
	for i, row := range rows {
		tableRows[i] = row
	}

	return components.RunTableInline(title, columns, tableRows)
}

// outputWithInteractiveTable outputs data with optional interactive table.
// If interactive and not JSON format, shows interactive table; otherwise shows static table.
func outputWithInteractiveTable(title string, data any, headers []string, rows [][]string) (table.Row, error) {
	format := config.GetOutputFormat()
	if format == "json" {
		outputJSON(data)
		return nil, nil
	}

	if isInteractive() && len(rows) > 0 {
		return outputInteractiveTable(title, headers, rows)
	}

	outputTable(headers, rows)
	return nil, nil
}
