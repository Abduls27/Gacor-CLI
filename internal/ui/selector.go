package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type selectionModel struct {
	input         textinput.Model
	title         string
	items         []string
	searchable    bool
	cursor        int
	chosen        int
	cancelled     bool
	width, height int
}

func (m selectionModel) filtered() []int {
	query := ""
	if m.searchable {
		query = strings.ToLower(strings.TrimSpace(m.input.Value()))
	}
	var matches []int
	for i, item := range m.items {
		if strings.Contains(strings.ToLower(item), query) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (m selectionModel) Init() tea.Cmd {
	if m.searchable {
		return textinput.Blink
	}
	return nil
}

func (m selectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		matches := m.filtered()
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.cursor < len(matches)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(matches) > 0 {
				m.chosen = matches[m.cursor]
				return m, tea.Quit
			}
			return m, nil
		}
		if !m.searchable {
			return m, nil
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

func (m selectionModel) View() string {
	var rows []string
	matches := m.filtered()
	if len(matches) == 0 {
		rows = append(rows, muted.Render("  (tidak ada pilihan yang cocok)"))
	}
	for i, itemIndex := range matches {
		if i == m.cursor {
			rows = append(rows, neon.Render("▶ ")+bright.Render(m.items[itemIndex]))
		} else {
			rows = append(rows, muted.Render("  "+m.items[itemIndex]))
		}
	}

	body := titleStyle.Render(m.title) + "\n\n" + strings.Join(rows, "\n")
	if m.searchable {
		body = lipgloss.JoinVertical(lipgloss.Left,
			m.input.View(), "",
			body)
	}
	hints := bright.Render("↑↓") + muted.Render(" pilih  ") +
		bright.Render("enter") + muted.Render(" konfirmasi  ") +
		bright.Render("esc") + muted.Render(" batal")
	mainContent := lipgloss.JoinVertical(lipgloss.Center,
		logo(), "",
		panel.Render(body),
		lipgloss.PlaceHorizontal(78, lipgloss.Right, hints),
	)

	footer := muted.Render(version)
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, mainContent),
			lipgloss.PlaceHorizontal(m.width, lipgloss.Right, footer),
		),
	)
}

// SelectIndexed displays a searchable picker using the home screen style.
// It returns -1 when the user cancels.
func SelectIndexed(title string, items []string) (int, error) {
	return SelectIndexedWithSearch(title, items, true)
}

// SelectIndexedWithSearch optionally displays a text input for filtering choices.
func SelectIndexedWithSearch(title string, items []string, searchable bool) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("tidak ada pilihan yang tersedia")
	}

	ti := textinput.New()
	ti.Placeholder = "Cari..."
	ti.Prompt = ""
	if searchable {
		ti.Focus()
	}

	result, err := tea.NewProgram(
		selectionModel{input: ti, title: title, items: items, searchable: searchable},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return -1, err
	}

	final := result.(selectionModel)
	if final.cancelled {
		return -1, nil
	}
	return final.chosen, nil
}
