package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DatabaseEntry struct {
	Name string
	User string
	Port int
}

// DatabaseAction is the action chosen from the database manager menu.
type DatabaseAction int

const (
	DatabaseActionNone DatabaseAction = iota
	DatabaseActionCreate
	DatabaseActionDelete
)

type databaseManagerModel struct {
	engine    string
	version   string
	user      string
	port      int
	databases []DatabaseEntry

	mode   databaseManagerMode
	cursor int

	action       DatabaseAction
	selectedName string
	cancelled    bool

	width, height int
}

type databaseManagerMode int

const (
	modeMenu databaseManagerMode = iota
	modeSelectDelete
)

func (m databaseManagerModel) Init() tea.Cmd { return nil }

func (m databaseManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch m.mode {
		case modeMenu:
			return m.updateMenu(msg)
		case modeSelectDelete:
			return m.updateSelectDelete(msg)
		}
	}
	return m, nil
}

func (m databaseManagerModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const optionCount = 3 // Buat, Hapus, Kembali
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < optionCount-1 {
			m.cursor++
		}
	case "enter":
		switch m.cursor {
		case 0:
			m.action = DatabaseActionCreate
			return m, tea.Quit
		case 1:
			if len(m.databases) == 0 {
				return m, nil
			}
			m.mode = modeSelectDelete
			m.cursor = 0
			return m, nil
		case 2:
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m databaseManagerModel) updateSelectDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		// kembali ke menu utama, bukan keluar
		m.mode = modeMenu
		m.cursor = 1
		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.databases)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.databases) == 0 {
			return m, nil
		}
		m.action = DatabaseActionDelete
		m.selectedName = m.databases[m.cursor].Name
		return m, tea.Quit
	}
	return m, nil
}

func (m databaseManagerModel) View() string {
	if m.mode == modeSelectDelete {
		return m.viewSelectDelete()
	}
	return m.viewMenu()
}

func (m databaseManagerModel) databaseListRows() []string {
	rows := []string{
		titleStyle.Render("Database yang tersedia"),
		fmt.Sprintf("  %-32s %-24s %s", "NAMA", "USERNAME", "PORT"),
	}
	if len(m.databases) == 0 {
		rows = append(rows, muted.Render("  (belum ada database aplikasi)"))
	}
	for _, database := range m.databases {
		rows = append(rows, fmt.Sprintf("  %-32s %-24s %d", database.Name, database.User, database.Port))
	}
	return rows
}

func (m databaseManagerModel) viewMenu() string {
	rows := []string{
		titleStyle.Render(fmt.Sprintf("Database aktif: %s %s", m.engine, m.version)),
		muted.Render(fmt.Sprintf("Host: 127.0.0.1    Port: %d    User GACOR: %s", m.port, m.user)),
		"",
	}
	rows = append(rows, m.databaseListRows()...)

	options := []string{"Buat database", "Hapus database", "Kembali ke menu awal GACOR"}
	rows = append(rows, "", titleStyle.Render("Menu"))
	for i, option := range options {
		label := option
		if i == 1 && len(m.databases) == 0 {
			label += " (tidak ada database)"
		}
		if i == m.cursor {
			rows = append(rows, neon.Render("▶ "+label))
		} else {
			rows = append(rows, muted.Render("  "+label))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	hints := bright.Render("↑↓") + muted.Render(" pilih  ") +
		bright.Render("enter") + muted.Render(" konfirmasi  ") +
		bright.Render("esc") + muted.Render(" kembali")
	mainContent := lipgloss.JoinVertical(lipgloss.Center,
		logo(), "",
		panel.Render(body),
		lipgloss.PlaceHorizontal(78, lipgloss.Right, hints),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, mainContent)
}

func (m databaseManagerModel) viewSelectDelete() string {
	rows := []string{
		titleStyle.Render("Pilih database yang akan dihapus"),
		muted.Render("Tindakan ini tidak dapat dibatalkan."),
		"",
	}
	for i, database := range m.databases {
		label := fmt.Sprintf("%-32s %-24s %d", database.Name, database.User, database.Port)
		if i == m.cursor {
			rows = append(rows, neon.Render("▶ "+label))
		} else {
			rows = append(rows, muted.Render("  "+label))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	hints := bright.Render("↑↓") + muted.Render(" pilih  ") +
		bright.Render("enter") + muted.Render(" hapus  ") +
		bright.Render("esc") + muted.Render(" batal")
	mainContent := lipgloss.JoinVertical(lipgloss.Center,
		logo(), "",
		panel.Render(body),
		lipgloss.PlaceHorizontal(78, lipgloss.Right, hints),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, mainContent)
}

// DatabaseManagerResult carries the user's choice out of the database manager screen.
type DatabaseManagerResult struct {
	Action       DatabaseAction
	DatabaseName string // populated only when Action == DatabaseActionDelete
}

// DatabaseManager shows the active server and its available databases,
// and lets the user create a new one, delete an existing one, or return
// to the main GACOR menu.
func DatabaseManager(engine, version, user string, port int, databases []DatabaseEntry) (DatabaseManagerResult, error) {
	result, err := tea.NewProgram(
		databaseManagerModel{
			engine:    engine,
			version:   version,
			user:      user,
			port:      port,
			databases: databases,
		},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return DatabaseManagerResult{}, err
	}
	final := result.(databaseManagerModel)
	if final.cancelled || final.action == DatabaseActionNone {
		return DatabaseManagerResult{}, nil
	}
	return DatabaseManagerResult{Action: final.action, DatabaseName: final.selectedName}, nil
}