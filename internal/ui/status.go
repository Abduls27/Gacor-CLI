package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServiceStatus holds data for a service (PHP, Nginx, or a database) displayed on the status screen.
type ServiceStatus struct {
	Label   string // "PHP", "Nginx", "MySQL"
	Version string
	Running bool
	PID     int    // 0 if no PID is available (e.g. the database is not running)
	Detail  string // Optional, e.g. "127.0.0.1:3306" for a database; leave empty if unused.
}

type statusModel struct {
	services      []ServiceStatus
	width, height int
	quit          bool
}

func (m statusModel) Init() tea.Cmd { return nil }

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q", "enter":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func statusCard(s ServiceStatus) string {
	borderColor := lipgloss.Color("#FF5F87")
	dot := red.Render("●")
	state := red.Render("STOPPED")
	pidLine := ""
	if s.Running {
		borderColor = lipgloss.Color("#00FF87")
		dot = green.Render("●")
		state = green.Render("RUNNING")
		if s.PID > 0 {
			pidLine = muted.Render(fmt.Sprintf("PID %d", s.PID))
		}
	}

	title := muted.Render("/" + s.Label + "─")
	name := bold.Render(s.Label)
	if s.Version != "" {
		name += " " + muted.Render(s.Version)
	}

	lines := []string{title, name, dot + " " + state}
	if pidLine != "" {
		lines = append(lines, pidLine)
	}
	if s.Detail != "" {
		lines = append(lines, muted.Render(s.Detail))
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 2).
		Width(26)

	body := ""
	for i, l := range lines {
		if i > 0 {
			body += "\n"
		}
		body += l
	}
	return style.Render(body)
}

func (m statusModel) View() string {
	if m.quit {
		return ""
	}

	header := lipgloss.JoinVertical(
		lipgloss.Center,
		logo(),
		muted.Render(version),
		"",
		white.Render("Service Status"),
		muted.Render("Live status of PHP, Nginx, and database"),
	)

	var cards []string
	for _, s := range m.services {
		cards = append(cards, statusCard(s))
	}

	// Arrange cards in rows of two to match the OpenCode grid layout.
	var rows []string
	for i := 0; i < len(cards); i += 2 {
		if i+1 < len(cards) {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i], "  ", cards[i+1]))
		} else {
			rows = append(rows, cards[i])
		}
	}

	grid := lipgloss.JoinVertical(lipgloss.Center, rows...)

	hints := bright.Render("esc/q/enter") + muted.Render(" close")
	footer := lipgloss.PlaceHorizontal(78, lipgloss.Right, hints)

	mainContent := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		grid,
		"",
		footer,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, mainContent)
}

// ShowStatus displays a full-screen service status TUI, similar to the layout in home.go.
func ShowStatus(services []ServiceStatus) error {
	_, err := tea.NewProgram(
		statusModel{services: services},
		tea.WithAltScreen(),
	).Run()
	return err
}
