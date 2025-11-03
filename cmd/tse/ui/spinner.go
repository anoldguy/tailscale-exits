package ui

import (
	"fmt"
	"math/rand"
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
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("spinner error: %w", err)
	}

	// Get the final model state (with the error if any)
	final, ok := finalModel.(spinnerModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// If the operation failed, return the error
	if final.err != nil {
		return final.err
	}

	return nil
}

// rotatingSpinnerModel shows different messages while waiting for a background check
type rotatingSpinnerModel struct {
	spinner       spinner.Model
	messages      []string
	currentIndex  int
	done          bool
	err           error
	quitting      bool
	nextRotation  time.Time
}

func newRotatingSpinnerModel(messages []string) rotatingSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = InfoStyle
	return rotatingSpinnerModel{
		spinner:      s,
		messages:     messages,
		currentIndex: 0,
		nextRotation: time.Now().Add(randomRotationDelay()),
	}
}

// randomRotationDelay returns a random duration between 3-6 seconds
func randomRotationDelay() time.Duration {
	min := 3000 // 3 seconds in milliseconds
	max := 6000 // 6 seconds in milliseconds
	ms := min + rand.Intn(max-min+1)
	return time.Duration(ms) * time.Millisecond
}

// rotateMsg is sent when it's time to rotate to the next message
type rotateMsg struct{}

func (m rotatingSpinnerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(randomRotationDelay(), func(time.Time) tea.Msg {
			return rotateMsg{}
		}),
	)
}

func (m rotatingSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case rotateMsg:
		// Rotate to next message
		m.currentIndex = (m.currentIndex + 1) % len(m.messages)
		// Schedule next rotation
		return m, tea.Tick(randomRotationDelay(), func(time.Time) tea.Msg {
			return rotateMsg{}
		})

	case doneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	default:
		return m, nil
	}
}

func (m rotatingSpinnerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		if m.err != nil {
			return fmt.Sprintf("%s %s\n", Cross(), m.messages[0])
		}
		return fmt.Sprintf("%s %s\n", Checkmark(), m.messages[0])
	}

	return fmt.Sprintf("%s %s", m.spinner.View(), m.messages[m.currentIndex])
}

// WithRotatingMessages runs an operation with rotating snarky messages.
// Displays messages from the slice, rotating every 3-6 seconds randomly.
// The checkFunc is called repeatedly in the background until it returns true or timeout (2 minutes).
// First message in the slice is used for the final checkmark/cross display.
func WithRotatingMessages(messages []string, checkFunc func() error) error {
	if len(messages) == 0 {
		return fmt.Errorf("no messages provided")
	}

	m := newRotatingSpinnerModel(messages)
	p := tea.NewProgram(m)

	// Background checker goroutine
	go func() {
		time.Sleep(50 * time.Millisecond) // Let spinner start

		timeout := time.After(2 * time.Minute)
		ticker := time.NewTicker(1 * time.Second) // Check every second
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				// Timeout reached, return error
				p.Send(doneMsg{err: fmt.Errorf("timeout waiting for propagation")})
				return

			case <-ticker.C:
				// Try the check
				err := checkFunc()
				if err == nil {
					// Check succeeded!
					p.Send(doneMsg{err: nil})
					return
				}
				// Check failed, keep waiting
			}
		}
	}()

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("spinner error: %w", err)
	}

	// Get the final model state (with the error if any)
	final, ok := finalModel.(rotatingSpinnerModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// If the operation failed, return the error
	if final.err != nil {
		return final.err
	}

	return nil
}
