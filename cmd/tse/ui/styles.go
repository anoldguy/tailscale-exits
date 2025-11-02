package ui

import (
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color profile detected from environment and terminal
	profile colorprofile.Profile

	// Lipgloss renderer
	renderer *lipgloss.Renderer

	// Semantic style palette
	TitleStyle     lipgloss.Style // Main section headers (pink)
	SubheaderStyle lipgloss.Style // Subsection headers (cyan)
	LabelStyle     lipgloss.Style // Keywords/definitions (bold white)
	HighlightStyle lipgloss.Style // URLs, tokens, values to copy (yellow)
	InfoStyle      lipgloss.Style // Instructions, directions (blue)
	SuccessStyle   lipgloss.Style // Success messages (green)
	ErrorStyle     lipgloss.Style // Error messages (red)
	WarningStyle   lipgloss.Style // Warnings (orange)
	SubtleStyle    lipgloss.Style // Dividers, metadata (gray)
	BoldStyle      lipgloss.Style // Generic bold
)

func init() {
	// Detect color profile (respects NO_COLOR, CLICOLOR, CLICOLOR_FORCE)
	profile = colorprofile.Detect(os.Stdout, os.Environ())

	// Create renderer
	renderer = lipgloss.NewRenderer(os.Stdout)

	// Initialize styles based on color profile capabilities
	if profile == colorprofile.Ascii || profile == colorprofile.NoTTY {
		// No colors available - use plain styles with formatting only
		TitleStyle = renderer.NewStyle().Bold(true)
		SubheaderStyle = renderer.NewStyle().Bold(true)
		LabelStyle = renderer.NewStyle().Bold(true)
		HighlightStyle = renderer.NewStyle().Bold(true)
		InfoStyle = renderer.NewStyle()
		SuccessStyle = renderer.NewStyle()
		ErrorStyle = renderer.NewStyle()
		WarningStyle = renderer.NewStyle()
		SubtleStyle = renderer.NewStyle()
		BoldStyle = renderer.NewStyle().Bold(true)
	} else {
		// Colors available - use semantic palette
		TitleStyle = renderer.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")) // Pink/Magenta - main headers

		SubheaderStyle = renderer.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("51")) // Cyan - subsection headers

		LabelStyle = renderer.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")) // Bright white - keywords/labels

		HighlightStyle = renderer.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")) // Yellow - URLs, tokens to copy

		InfoStyle = renderer.NewStyle().
			Foreground(lipgloss.Color("33")) // Blue - instructions/directions

		SuccessStyle = renderer.NewStyle().
			Foreground(lipgloss.Color("42")) // Green - success messages

		ErrorStyle = renderer.NewStyle().
			Foreground(lipgloss.Color("9")) // Red - errors

		WarningStyle = renderer.NewStyle().
			Foreground(lipgloss.Color("214")) // Orange - warnings

		SubtleStyle = renderer.NewStyle().
			Foreground(lipgloss.Color("241")) // Gray - dividers, metadata

		BoldStyle = renderer.NewStyle().Bold(true)
	}
}

// Success renders text in success style (green)
func Success(text string) string {
	return SuccessStyle.Render(text)
}

// Error renders text in error style (red)
func Error(text string) string {
	return ErrorStyle.Render(text)
}

// Warning renders text in warning style (orange)
func Warning(text string) string {
	return WarningStyle.Render(text)
}

// Info renders text in info style (blue)
func Info(text string) string {
	return InfoStyle.Render(text)
}

// Subtle renders text in subtle style (gray)
func Subtle(text string) string {
	return SubtleStyle.Render(text)
}

// Highlight renders text in highlight style (bold yellow)
func Highlight(text string) string {
	return HighlightStyle.Render(text)
}

// Title renders text in title style (bold pink - main headers)
func Title(text string) string {
	return TitleStyle.Render(text)
}

// Subheader renders text in subheader style (bold cyan - subsection headers)
func Subheader(text string) string {
	return SubheaderStyle.Render(text)
}

// Label renders text in label style (bold white - keywords/definitions)
func Label(text string) string {
	return LabelStyle.Render(text)
}

// Bold renders text in bold
func Bold(text string) string {
	return BoldStyle.Render(text)
}

// Checkmark returns a styled checkmark (green ✓)
func Checkmark() string {
	return Success("✓")
}

// Cross returns a styled cross (red ✗)
func Cross() string {
	return Error("✗")
}

// GetProfile returns the detected color profile (useful for tests or conditional logic)
func GetProfile() colorprofile.Profile {
	return profile
}
