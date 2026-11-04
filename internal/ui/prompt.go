package ui

import (
	"errors"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ErrPromptCancelled = errors.New("prompt canceled")

type promptModel struct {
	input         textinput.Model
	title         string
	value         string
	cancelled     bool
	width, height int
}

func (m promptModel) Init() tea.Cmd { return textinput.Blink }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			m.value = m.input.Value()
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m promptModel) View() string {
	body := titleStyle.Render(m.title) + "\n\n" + m.input.View()
	hints := bright.Render("enter") + muted.Render(" confirm  ") +
		bright.Render("esc") + muted.Render(" cancel")
	mainContent := lipgloss.JoinVertical(lipgloss.Center,
		logo(), "",
		panel.Render(body),
		lipgloss.PlaceHorizontal(78, lipgloss.Right, hints),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, mainContent),
			lipgloss.PlaceHorizontal(m.width, lipgloss.Right, muted.Render(version)),
		),
	)
}

// PromptInput asks for one value using the same full-screen style as the home menu.
func PromptInput(title string, secret bool) (string, error) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Focus()
	ti.CharLimit = 256
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
		ti.Placeholder = "Password"
	} else {
		ti.Placeholder = "Type here..."
		ti.Width = 60
	}

	result, err := tea.NewProgram(
		promptModel{input: ti, title: title},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return "", err
	}
	final := result.(promptModel)
	if final.cancelled {
		return "", ErrPromptCancelled
	}
	return final.value, nil
}
