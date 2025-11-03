package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// spinnerModel is the bubbletea model for our spinner
type spinnerModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
	quitting bool
}

func newSpinnerModel(message string) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = InfoStyle
	return spinnerModel{
		spinner: s,
		message: message,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Allow Ctrl+C to cancel
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case doneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		if m.err != nil {
			return fmt.Sprintf("%s %s\n", Cross(), m.message)
		}
		return fmt.Sprintf("%s %s\n", Checkmark(), m.message)
	}

	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// doneMsg is sent when the operation completes
type doneMsg struct {
	err error
}

// WithSpinner runs an operation with a spinner, showing the message while running.
// On completion, it persists the message with a ✓ checkmark.
// On error, it persists the message with a ✗ and returns the error.
func WithSpinner(message string, operation func() error) error {
	m := newSpinnerModel(message)
	p := tea.NewProgram(m)

	// Run the operation in a goroutine
	go func() {
		// Give the spinner a moment to start rendering
		time.Sleep(50 * time.Millisecond)
		err := operation()
		p.Send(doneMsg{err: err})
	}()

	// Run the TUI
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("spinner error: %w", err)
	}

	// If the operation failed, return the error
	if m.err != nil {
		return m.err
	}

	return nil
}
