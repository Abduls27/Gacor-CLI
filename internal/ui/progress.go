package ui

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progDoneMsg struct{ err error }

type progModel struct {
	title  string
	stage  string
	done   int64
	total  int64
	bar    progress.Model
	err    error
	quit   bool
	cancel context.CancelFunc
	w, h   int
}

func (m progModel) Init() tea.Cmd { return nil }

func (m progModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		if s := msg.String(); s == "ctrl+c" || s == "esc" {
			m.quit = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case ProgressEvent:
		m.stage = msg.Stage
		if msg.Label != "" {
			m.title = msg.Label
		}
		m.done, m.total = msg.Done, msg.Total
	case progDoneMsg:
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m progModel) View() string {
	if m.title == "" {
		m.title = "Preparing..."
	}

	var pct float64
	if m.total > 0 {
		pct = float64(m.done) / float64(m.total)
	}
	mb := func(b int64) float64 { return float64(b) / 1024 / 1024 }

	body := bright.Render(m.title) + "\n\n"
	switch m.stage {
	case "extract":
		body += neon.Render("Extracting ") + muted.Render(m.title+"...")
	case "initialize":
		body += neon.Render("Initializing ") + muted.Render(m.title+"...")
	case "configure":
		body += neon.Render("Configuring...")
	case "start":
		body += neon.Render("Starting ") + muted.Render(m.title+"...")
	default:
		body += neon.Render("Downloading") + "\n" + m.bar.ViewAs(pct) + "\n" +
			muted.Render(fmt.Sprintf("%.0f%%  %.1f/%.1f MB", pct*100, mb(m.done), mb(m.total)))
	}

	hints := bright.Render("esc") + muted.Render(" cancel")
	mainContent := lipgloss.JoinVertical(lipgloss.Center,
		logo(), "",
		panel.Render(body),
		lipgloss.PlaceHorizontal(68, lipgloss.Right, hints),
	)

	footer := muted.Render(version)

	return lipgloss.Place(
		m.w, m.h,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.Place(m.w, m.h-1, lipgloss.Center, lipgloss.Center, mainContent),
			lipgloss.PlaceHorizontal(m.w, lipgloss.Right, footer),
		),
	)
}

// RunWithProgress executes a task that sends ProgressEvent values through onEvent
// while displaying the progress screen. The task must call onEvent(ProgressEvent{...}).
func RunWithProgress(task func(onEvent func(ProgressEvent)) error) error {
	return runWithProgress(func(_ context.Context, onEvent func(ProgressEvent)) error {
		return task(onEvent)
	}, false)
}

// RunWithProgressContext lets long-running work stop when the user cancels the progress screen.
func RunWithProgressContext(task func(context.Context, func(ProgressEvent)) error) error {
	return runWithProgress(task, true)
}

func runWithProgress(task func(context.Context, func(ProgressEvent)) error, cancellable bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	m := progModel{bar: progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(56),
		progress.WithoutPercentage(),
	), cancel: nil}
	if cancellable {
		m.cancel = cancel
	}
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		var taskErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					taskErr = fmt.Errorf("panic: %v", r)
				}
			}()
			taskErr = task(ctx, func(e ProgressEvent) { p.Send(e) })
		}()
		done <- taskErr
		p.Send(progDoneMsg{taskErr})
	}()

	final, err := p.Run()
	if err != nil {
		if cancellable {
			cancel()
			<-done
		}
		return err
	}
	fm := final.(progModel)
	if fm.quit {
		if cancellable {
			cancel()
			taskErr := <-done
			if taskErr != nil && !errors.Is(taskErr, context.Canceled) {
				return taskErr
			}
		}
		fmt.Println(muted.Render("Canceled."))
		return fmt.Errorf("canceled")
	}
	if fm.err != nil {
		return fm.err
	}
	fmt.Println(neon.Render("✓ ") + bright.Render(fm.title+" complete"))
	return nil
}
