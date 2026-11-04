package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	bright = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	neon   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF87")).Bold(true)
	panel  = lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#00FF87")).
		Padding(1, 2).
		Width(80)
)

func logo() string {
	return neon.Render(strings.Trim(asciiArt, "\n"))
}

type model struct {
	input         textinput.Model
	root          *cobra.Command
	stack         []*cobra.Command // breadcrumb — stack[0] is always the root
	cursor        int
	chosen        *cobra.Command // final command to execute (leaf, no children, or user presses enter on a command with its own Run)
	width, height int
}

func (m model) current() *cobra.Command {
	return m.stack[len(m.stack)-1]
}

func (m model) filtered() []*cobra.Command {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	var out []*cobra.Command
	for _, c := range m.current().Commands() {
		if c.IsAvailableCommand() && strings.Contains(strings.ToLower(c.Name()), q) {
			out = append(out, c)
		}
	}
	return out
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		list := m.filtered()
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if len(m.stack) > 1 {
				// Go back one level.
				m.stack = m.stack[:len(m.stack)-1]
				m.input.SetValue("")
				m.cursor = 0
				return m, nil
			}
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.cursor < len(list)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(list) == 0 {
				return m, nil
			}
			sel := list[m.cursor]
			if sel.HasAvailableSubCommands() {
				m.stack = append(m.stack, sel)
				m.input.SetValue("")
				m.cursor = 0
				return m, nil
			}
			m.chosen = sel
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if n := len(m.filtered()); n > 0 {
		m.cursor = min(m.cursor, n-1)
	} else {
		m.cursor = 0
	}
	return m, cmd
}
func (m model) View() string {
	if m.chosen != nil {
		return ""
	}

	var rows []string
	list := m.filtered()

	if len(list) == 0 {
		rows = append(rows, muted.Render("  (no commands)"))
	}

	for i, c := range list {
		name := fmt.Sprintf("%-14s", c.Name())
		arrow := "  "

		if c.HasAvailableSubCommands() {
			arrow = " ▸"
		}

		if i == m.cursor {
			rows = append(rows,
				neon.Render("▶ "+name)+
					bright.Render(c.Short)+
					dim.Render(arrow),
			)
		} else {
			rows = append(rows,
				muted.Render("  "+name+c.Short)+
					dim.Render(arrow),
			)
		}
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.input.View(),
		"",
		strings.Join(rows, "\n"),
	)

	panelView := panel.Render(body)

	// Header
	header := lipgloss.JoinVertical(
		lipgloss.Center,
		logo(),
		muted.Render(version),
		"",
		white.Render(welcome),
		muted.Render(description),
	)

	// Footer hints
	backHint := bright.Render("esc") + muted.Render(" quit  ")
	if len(m.stack) > 1 {
		backHint = bright.Render("esc") + muted.Render(" back  ")
	}

	hints := bright.Render("↑↓") + muted.Render(" navigate  ") +
		bright.Render("enter") + muted.Render(" select/open  ") +
		backHint

	footer := lipgloss.PlaceHorizontal(
		78,
		lipgloss.Right,
		hints,
	)

	// Main layout
	mainContent := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		panelView,
		"",
		footer,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		mainContent,
	)
}

func CommandPath(c *cobra.Command) []string {
	var path []string
	for cur := c; cur != nil && cur.Name() != "gacor"; cur = cur.Parent() {
		path = append([]string{cur.Name()}, path...)
	}
	return path
}

// Home runs the TUI for navigating nested commands.
// It returns the command selected by the user (leaf), or nil if canceled.
func Home(root *cobra.Command) (*cobra.Command, error) {
	ti := textinput.New()
	ti.Placeholder = "Search commands..."
	ti.Prompt = ""
	ti.Focus()

	final, err := tea.NewProgram(
		model{input: ti, root: root, stack: []*cobra.Command{root}},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return nil, err
	}
	return final.(model).chosen, nil
}
