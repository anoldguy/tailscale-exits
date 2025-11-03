package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Table represents a simple bordered table
type Table struct {
	headers []string
	rows    [][]string
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	return &Table{
		headers: headers,
		rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	t.rows = append(t.rows, cells)
}

// Render renders the table with borders and styling
func (t *Table) Render() string {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(t.headers))
	for i, header := range t.headers {
		colWidths[i] = lipgloss.Width(header)
	}

	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(colWidths) {
				// Strip ANSI codes to get actual width
				width := lipgloss.Width(cell)
				if width > colWidths[i] {
					colWidths[i] = width
				}
			}
		}
	}

	// Define border styles
	var (
		borderStyle = renderer.NewStyle().Foreground(lipgloss.Color("205")) // Pink (same as titles)
		headerStyle = BoldStyle
	)

	var b strings.Builder

	// Top border (rounded)
	b.WriteString(borderStyle.Render("╭"))
	for i, width := range colWidths {
		b.WriteString(borderStyle.Render(strings.Repeat("─", width+2)))
		if i < len(colWidths)-1 {
			b.WriteString(borderStyle.Render("┬"))
		}
	}
	b.WriteString(borderStyle.Render("╮"))
	b.WriteString("\n")

	// Header row
	b.WriteString(borderStyle.Render("│"))
	for i, header := range t.headers {
		padded := padRight(headerStyle.Render(header), colWidths[i])
		b.WriteString(" " + padded + " ")
		b.WriteString(borderStyle.Render("│"))
	}
	b.WriteString("\n")

	// Header separator (heavy double-line)
	b.WriteString(borderStyle.Render("╞"))
	for i, width := range colWidths {
		b.WriteString(borderStyle.Render(strings.Repeat("═", width+2)))
		if i < len(colWidths)-1 {
			b.WriteString(borderStyle.Render("╪"))
		}
	}
	b.WriteString(borderStyle.Render("╡"))
	b.WriteString("\n")

	// Data rows
	for _, row := range t.rows {
		b.WriteString(borderStyle.Render("│"))
		for i, cell := range row {
			if i < len(colWidths) {
				padded := padRight(cell, colWidths[i])
				b.WriteString(" " + padded + " ")
				b.WriteString(borderStyle.Render("│"))
			}
		}
		b.WriteString("\n")
	}

	// Bottom border
	b.WriteString(borderStyle.Render("╰"))
	for i, width := range colWidths {
		b.WriteString(borderStyle.Render(strings.Repeat("─", width+2)))
		if i < len(colWidths)-1 {
			b.WriteString(borderStyle.Render("┴"))
		}
	}
	b.WriteString(borderStyle.Render("╯"))

	return b.String()
}

// padRight pads a string to the right, accounting for ANSI codes
func padRight(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}
