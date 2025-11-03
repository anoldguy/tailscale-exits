package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DangerBox renders a prominent danger warning box with red/orange borders
func DangerBox(title string, items []string, confirmText string) string {
	// Danger colors - red/orange for destruction
	dangerStyle := renderer.NewStyle().
		Foreground(lipgloss.Color("9")). // Red
		Bold(true)

	dangerBorderStyle := renderer.NewStyle().
		Foreground(lipgloss.Color("202")) // Orange

	// Build the box content
	var content strings.Builder

	// Title with fire emojis
	content.WriteString(dangerStyle.Render(fmt.Sprintf("    🔥 %s 🔥", title)))
	content.WriteString("\n\n")

	// Items to be deleted
	for _, item := range items {
		content.WriteString(fmt.Sprintf("  %s %s\n", Warning("•"), item))
	}

	content.WriteString("\n")

	// Confirmation instruction
	content.WriteString(fmt.Sprintf("  %s\n", dangerStyle.Render(confirmText)))

	// Calculate width based on content
	lines := strings.Split(content.String(), "\n")
	maxWidth := 0
	for _, line := range lines {
		width := lipgloss.Width(line)
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Add padding
	boxWidth := maxWidth + 4

	var box strings.Builder

	// Top border (double line for emphasis)
	box.WriteString(dangerBorderStyle.Render("╔"))
	box.WriteString(dangerBorderStyle.Render(strings.Repeat("═", boxWidth)))
	box.WriteString(dangerBorderStyle.Render("╗"))
	box.WriteString("\n")

	// Content rows
	for _, line := range lines {
		box.WriteString(dangerBorderStyle.Render("║"))
		box.WriteString("  ")
		box.WriteString(line)
		// Pad to box width
		currentWidth := lipgloss.Width(line)
		padding := boxWidth - currentWidth - 2
		if padding > 0 {
			box.WriteString(strings.Repeat(" ", padding))
		}
		box.WriteString(dangerBorderStyle.Render("║"))
		box.WriteString("\n")
	}

	// Bottom border (double line)
	box.WriteString(dangerBorderStyle.Render("╚"))
	box.WriteString(dangerBorderStyle.Render(strings.Repeat("═", boxWidth)))
	box.WriteString(dangerBorderStyle.Render("╝"))

	return box.String()
}
