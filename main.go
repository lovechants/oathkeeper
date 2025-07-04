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
	ticker        *time.Ticker
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
	ta.Placeholder = "Notes to stay on track"
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
		if m.notes.Focused() {
			if msg.Type == tea.KeyEsc {
				m.notes.Blur()
				return m, nil
			}
			m.notes, cmd = m.notes.Update(msg)
			return m, cmd
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

		switch m.mode {
		case modeInput:
			if msg.Type == tea.KeyEnter {
				d, err := time.ParseDuration(m.input.Value())
				if err == nil && d > 0 {
					m.duration = d
					m.remaining = d
					m.mode = modeTimer
					m.paused = false
					m.ticker = time.NewTicker(time.Second)
					cmds = append(cmds, waitForTick(m.ticker.C))
				}
				return m, tea.Batch(cmds...)
			}
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		case modeTimer:
			switch msg.String() {
			case "p": // Pause
				if m.ticker != nil {
					m.paused = true
					m.ticker.Stop()
					m.ticker = nil
				}
			case "r": // Resume
				if m.paused && m.remaining > 0 {
					m.paused = false
					m.ticker = time.NewTicker(time.Second)
					cmds = append(cmds, waitForTick(m.ticker.C))
				}
			case "e": // Edit/Reset
				if m.ticker != nil {
					m.ticker.Stop()
					m.ticker = nil
				}
				m.mode = modeInput
				m.input.Focus()
			case "n": // Edit Notes
				m.notes.Focus()
				cmds = append(cmds, textarea.Blink)
			}
		}

	case tickMsg:
		if m.paused || m.ticker == nil {
			break
		}
		m.remaining -= time.Second
		if m.remaining <= 0 {
			m.ticker.Stop()
			m.ticker = nil
			m.remaining = 0
		} else {
			cmds = append(cmds, waitForTick(m.ticker.C))
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

		help := "p: pause | r: resume | n: notes (esc to exit) | e: edit | esc: quit"
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
