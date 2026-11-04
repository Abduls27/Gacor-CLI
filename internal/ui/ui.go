package ui

import (
	"fmt"
	"strings"

	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorPrimary = lipgloss.Color("#00FF87")
	colorDim     = lipgloss.Color("#4D4D4D")
	colorCyan    = lipgloss.Color("#00D7FF")
	colorRed     = lipgloss.Color("#FF5F87")
	colorGreen   = lipgloss.Color("#00FF87")
	colorYellow  = lipgloss.Color("#FFD700")
	colorMuted   = lipgloss.Color("#888888")
	colorWhite   = lipgloss.Color("#FFFFFF")
)

var (
	primary = lipgloss.NewStyle().Foreground(colorPrimary)
	dim     = lipgloss.NewStyle().Foreground(colorDim)
	cyan    = lipgloss.NewStyle().Foreground(colorCyan)
	red     = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	green   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	yellow  = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	bold    = lipgloss.NewStyle().Bold(true)
	muted   = lipgloss.NewStyle().Foreground(colorMuted)
	white   = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 2).
			Width(28)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)
)

const asciiArt = `
  ██████╗  █████╗  ██████╗ ██████╗ ██████╗ 
 ██╔════╝ ██╔══██╗██╔════╝██╔═══██╗██╔══██╗
 ██║  ███╗███████║██║     ██║   ██║██████╔╝
 ██║   ██║██╔══██║██║     ██║   ██║██╔══██╗
 ╚██████╔╝██║  ██║╚██████╗╚██████╔╝██║  ██║
  ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═╝`

var (
	versionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))
)

const version = "v1.0.0"
const welcome = "Welcome to Gacor CLI"
const description = "Manage PHP, Nginx, and databases from your terminal"

func Banner() {
	fmt.Println(primary.Render(asciiArt))
	fmt.Println()

	fmt.Println(versionStyle.Render(version))

	fmt.Println()
	fmt.Println()
	fmt.Println(white.Render(welcome))
	fmt.Println(white.Render(description))
	fmt.Println()

	// Tip line di bawah seperti opencode
	dot := primary.Render("●")
	tipLabel := cyan.Copy().Bold(true).Render("Tip")
	tipText := muted.Render("Run ") + primary.Render("gacor run") + muted.Render(" to start PHP & Nginx · ") + primary.Render("gacor --help") + muted.Render(" for commands")
	fmt.Println(" " + dot + " " + tipLabel + " " + tipText)
	fmt.Println()
}

func StatusBox(title, content string, running bool) string {
	borderColor := lipgloss.Color("#FF5F87")
	if running {
		borderColor = lipgloss.Color("#00FF87")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 2).
		Width(28)

	header := muted.Render("/" + title + "─")
	return style.Render(header + "\n" + content)
}

func PrintStatus(phpVersion, nginxVersion string, phpRunning, nginxRunning bool, phpPid, nginxPid int) {
	// PHP box
	phpStatus := ""
	if phpRunning {
		phpStatus = green.Render("● RUNNING") + muted.Render(fmt.Sprintf("  PID: %d", phpPid))
	} else {
		phpStatus = red.Render("● STOPPED")
	}
	phpContent := bold.Render("PHP "+phpVersion) + "\n" + phpStatus
	phpBox := StatusBox("PHP", phpContent, phpRunning)

	// Nginx box
	nginxStatus := ""
	if nginxRunning {
		nginxStatus = green.Render("● RUNNING") + muted.Render(fmt.Sprintf("  PID: %d", nginxPid))
	} else {
		nginxStatus = red.Render("● STOPPED")
	}
	nginxContent := bold.Render("Nginx "+nginxVersion) + "\n" + nginxStatus
	nginxBox := StatusBox("Nginx", nginxContent, nginxRunning)

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, phpBox, "  ", nginxBox))
	fmt.Println()
}

func PrintDatabaseStatus(name, version string, port int, running bool) {
	status := red.Render("● STOPPED")
	if running {
		status = green.Render("● RUNNING")
	}
	content := bold.Render(name+" "+version) + "\n" +
		status + "\n" +
		muted.Render(fmt.Sprintf("127.0.0.1:%d", port))
	fmt.Println(StatusBox("Database", content, running))
	fmt.Println()
}

func Confirm(message string) (bool, error) {
	answer, err := PromptInput(message+" (y/N):", false)
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func Success(msg string) {
	fmt.Println(green.Render("✓ " + msg))
}

func Error(msg string) {
	fmt.Println(red.Render("✗ " + msg))
}

func Info(msg string) {
	fmt.Println(cyan.Render("→ " + msg))
}

func Warn(msg string) {
	fmt.Println(yellow.Render("! " + msg))
}

func Label(label, value string) {
	fmt.Printf("%s %s\n", bold.Render(label+":"), value)
}

func StatusDot(running bool) string {
	if running {
		return green.Render("●")
	}
	return red.Render("●")
}

func Divider() {
	fmt.Println(dim.Render(strings.Repeat("─", 60)))
}

func Info2() string {
	return cyan.Render("→")
}

func Cyan(s string) string {
	return cyan.Render(s)
}

// Run and Stop Command UI
var (
	infoIcon   = lipgloss.NewStyle().Foreground(colorGreen)
	stepStyle  = lipgloss.NewStyle().Bold(true)
	clockStyle = lipgloss.NewStyle().Foreground(colorPrimary)
	utcStyle   = lipgloss.NewStyle().Foreground(colorMuted)
)

// StepTimed menampilkan satu baris log dengan jam (putih) dan offset UTC (muted) di depan.
func StepTimed(msg string) {
	clock := time.Now().Format("15:04:05")
	offset := time.Now().Format("-07:00") + " UTC"

	line := clockStyle.Render(clock) + " " + utcStyle.Render(offset)
	ts := lipgloss.NewStyle().Width(22).Render(line)

	fmt.Println(ts + infoIcon.Render("●") + "  " + stepStyle.Render(msg))
}

// StepDetail menampilkan baris detail di bawah StepTimed, align dengan kolom pesan
// (bukan kolom timestamp) — kosongkan posisi timestamp dengan spasi, bukan ulang jam.
func StepDetail(msg string) {
	indent := strings.Repeat(" ", 22+3) // sejajar dengan lebar timestamp + ikon + spasi
	fmt.Println(indent + muted.Render(msg))
}
