package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BoxType defines the style and purpose of a box
type BoxType int

const (
	BoxSuccess   BoxType = iota // Green - success messages, completion
	BoxHighlight                // Yellow/Gold - important info like tokens, URLs
	BoxInfo                     // Cyan - general information, details
	BoxWarning                  // Orange - warnings, important notices
	BoxNeutral                  // Pink - neutral status, brand color
)

// Box represents a bordered content box with title and items/content
type Box struct {
	Title   string
	Content []string // Each string is a line of content
	boxType BoxType
}

// NewBox creates a new box of the specified type
func NewBox(boxType BoxType, title string, content ...string) *Box {
	return &Box{
		Title:   title,
		Content: content,
		boxType: boxType,
	}
}

// SuccessBox creates a green success box
func SuccessBox(title string, content ...string) string {
	return NewBox(BoxSuccess, title, content...).Render()
}

// HighlightBox creates a yellow/gold highlight box for important info
func HighlightBox(title string, content ...string) string {
	return NewBox(BoxHighlight, title, content...).Render()
}

// InfoBox creates a cyan information box
func InfoBox(title string, content ...string) string {
	return NewBox(BoxInfo, title, content...).Render()
}

// WarningBox creates an orange warning box
func WarningBox(title string, content ...string) string {
	return NewBox(BoxWarning, title, content...).Render()
}

// NeutralBox creates a pink neutral box
func NeutralBox(title string, content ...string) string {
	return NewBox(BoxNeutral, title, content...).Render()
}

// getBorderColor returns the lipgloss color for the box type
func (b *Box) getBorderColor() lipgloss.Color {
	switch b.boxType {
	case BoxSuccess:
		return lipgloss.Color("42") // Green
	case BoxHighlight:
		return lipgloss.Color("229") // Yellow/Gold
	case BoxInfo:
		return lipgloss.Color("51") // Cyan
	case BoxWarning:
		return lipgloss.Color("208") // Orange
	case BoxNeutral:
		return lipgloss.Color("205") // Pink
	default:
		return lipgloss.Color("205") // Default to pink
	}
}

// getTitleStyle returns styled title based on box type
func (b *Box) getTitleStyle() lipgloss.Style {
	color := b.getBorderColor()
	return renderer.NewStyle().
		Foreground(color).
		Bold(true)
}

// Render renders the box with borders, title, and content
func (b *Box) Render() string {
	borderColor := b.getBorderColor()
	borderStyle := renderer.NewStyle().Foreground(borderColor)
	titleStyle := b.getTitleStyle()

	// Build content
	var content strings.Builder

	// Title (centered or left-aligned depending on length)
	if b.Title != "" {
		content.WriteString(titleStyle.Render(fmt.Sprintf("  %s", b.Title)))
		content.WriteString("\n")
		if len(b.Content) > 0 {
			content.WriteString("\n") // Extra spacing after title if there's content
		}
	}

	// Content lines
	for i, line := range b.Content {
		content.WriteString(fmt.Sprintf("  %s", line))
		if i < len(b.Content)-1 {
			content.WriteString("\n")
		}
	}

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

	// Top border (rounded): ╭─────╮
	box.WriteString(borderStyle.Render("╭"))
	box.WriteString(borderStyle.Render(strings.Repeat("─", boxWidth)))
	box.WriteString(borderStyle.Render("╮"))
	box.WriteString("\n")

	// Content rows
	for _, line := range lines {
		box.WriteString(borderStyle.Render("│"))
		box.WriteString("  ")
		box.WriteString(line)
		// Pad to box width
		currentWidth := lipgloss.Width(line)
		padding := boxWidth - currentWidth - 2
		if padding > 0 {
			box.WriteString(strings.Repeat(" ", padding))
		}
		box.WriteString(borderStyle.Render("│"))
		box.WriteString("\n")
	}

	// Bottom border (rounded): ╰─────╯
	box.WriteString(borderStyle.Render("╰"))
	box.WriteString(borderStyle.Render(strings.Repeat("─", boxWidth)))
	box.WriteString(borderStyle.Render("╯"))

	return box.String()
}
