package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeInput mode = iota
	modeTimer
)

type tickMsg time.Time

type model struct {
	mode          mode
	duration      time.Duration
	remaining     time.Duration
	timer         *time.Timer
	paused        bool
	input         textinput.Model
	notes         textarea.Model
	width, height int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "e.g., 30m, 1h15m, 90s"
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 30

	ta := textarea.New()
	ta.Placeholder = "Notes to keep you on task..."
	ta.SetWidth(50)
	ta.SetHeight(5)

	return model{
		mode:   modeInput,
		input:  ti,
		notes:  ta,
		paused: false,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

		if m.notes.Focused() {
			m.notes, cmd = m.notes.Update(msg)
			return m, cmd
		}

		switch m.mode {
		case modeInput:
			if msg.Type == tea.KeyEnter {
				d, err := time.ParseDuration(m.input.Value())
				if err == nil {
					m.duration = d
					m.remaining = d
					m.mode = modeTimer
					m.notes.Focus()
				}
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		case modeTimer:
			switch msg.String() {
			case "s": // Start
				if m.timer == nil && m.remaining > 0 {
					m.paused = false
					m.timer = time.NewTimer(m.remaining)
					cmds = append(cmds, waitForTick(m.timer.C))
				}
			case "p": // Pause
				if m.timer != nil && !m.paused {
					m.paused = true
					m.timer.Stop()
				}
			case "r": // Resume
				if m.timer != nil && m.paused {
					m.paused = false
					m.timer.Reset(m.remaining)
					cmds = append(cmds, waitForTick(m.timer.C))
				}
			case "e": // Edit/Reset
				if m.timer != nil {
					m.timer.Stop()
				}
				m.timer = nil
				m.mode = modeInput
				m.input.Focus()
			}
		}

	case tickMsg:
		m.remaining -= time.Second
		if m.remaining <= 0 {
			if m.timer != nil {
				m.timer.Stop()
			}
			m.timer = nil
			m.remaining = 0
		} else {
			m.timer.Reset(time.Second)
			cmds = append(cmds, waitForTick(m.timer.C))
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}

	return m, tea.Batch(cmds...)
}

func waitForTick(c <-chan time.Time) tea.Cmd {
	return func() tea.Msg {
		return tickMsg(<-c)
	}
}

func (m model) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if m.mode == modeInput {
		b.WriteString(titleStyle.Render("Set Timer Duration") + "\n")
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString(helpStyle.Render("Press <enter> to confirm."))
	} else {
		timerStr := formatDuration(m.remaining)
		timerStyle := lipgloss.NewStyle().
			Bold(true).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

		if m.remaining == 0 && m.duration > 0 {
			timerStyle = timerStyle.Foreground(lipgloss.Color("196")).
				BorderForeground(lipgloss.Color("196"))
			timerStr = "Time's Up!"
		} else if m.paused {
			timerStyle = timerStyle.Foreground(lipgloss.Color("220"))
		}

		b.WriteString(timerStyle.Render(timerStr) + "\n\n")

		help := "s: start | p: pause | r: resume | e: edit/reset | esc: quit"
		b.WriteString(helpStyle.Render(help))
	}

	b.WriteString("\n\n" + m.notes.View())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		b.String(),
	)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
