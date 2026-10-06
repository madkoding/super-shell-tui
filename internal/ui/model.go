// Package ui lays out the Super Shell TUI: header, optional sidebar and the
// shell pane. Keyboard input does not go through Bubble Tea (see package
// input); the model only reacts to resizes, screen updates and actions.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/madkoding/super-shell-tui/internal/input"
	"github.com/madkoding/super-shell-tui/internal/shell"
)

// ActionMsg carries a prefix-key action from the input pump.
type ActionMsg input.Action

type screenMsg struct{}

type exitMsg struct{ err error }

// Model is the root Bubble Tea model.
type Model struct {
	sess      *shell.Session
	shellPath string

	width, height int
	showSidebar   bool
	showHelp      bool
	cwd           string

	Err error
}

// New builds the model around a running shell session.
func New(sess *shell.Session, shellPath string) *Model {
	return &Model{sess: sess, shellPath: shellPath, showSidebar: true}
}

func (m *Model) Init() tea.Cmd { return m.waitScreen() }

func (m *Model) waitScreen() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.sess.Updates():
			return screenMsg{}
		case <-m.sess.Done():
			return exitMsg{err: m.sess.Err()}
		}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeShell()
	case screenMsg:
		m.cwd = m.sess.Cwd()
		return m, m.waitScreen()
	case exitMsg:
		m.Err = msg.err
		return m, tea.Quit
	case ActionMsg:
		switch input.Action(msg) {
		case input.ActionQuit:
			return m, tea.Quit
		case input.ActionToggleSidebar:
			m.showSidebar = !m.showSidebar
			m.resizeShell()
		case input.ActionToggleHelp:
			m.showHelp = !m.showHelp
			if m.showHelp && !m.showSidebar {
				m.showSidebar = true
				m.resizeShell()
			}
		}
	}
	return m, nil
}

// paneSize returns the inner size (cells) available to the shell.
func (m *Model) paneSize() (cols, rows int) {
	cols = m.width - 2 // pane border
	if m.showSidebar {
		cols -= sidebarWidth + 2
	}
	rows = m.height - 2 - 2 // header + status, pane border
	return max(cols, 1), max(rows, 1)
}

func (m *Model) resizeShell() {
	if m.width == 0 || m.height == 0 {
		return
	}
	_ = m.sess.Resize(m.paneSize())
}

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	cols, rows := m.paneSize()

	title := "Super Shell"
	if t := m.sess.Title(); t != "" {
		title += " · " + t
	}
	header := headerStyle.Width(m.width).MaxWidth(m.width).Render(title)

	pane := paneStyle.Width(cols).Height(rows).Render(m.sess.Render(true))
	body := pane
	if m.showSidebar {
		side := sidebarStyle.Width(sidebarWidth).Height(rows).Render(m.sidebar())
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, pane)
	}

	hint := "Ctrl+] ? ayuda · Ctrl+] s panel · Ctrl+] q salir · Shift+PgUp historial"
	status := statusStyle.Width(m.width).MaxWidth(m.width).Render(hint)
	if off, history := m.sess.ScrollOffset(); off > 0 {
		status = scrollStyle.Width(m.width).MaxWidth(m.width).
			Render(fmt.Sprintf("Historial: %d/%d líneas arriba · Shift+PgDn bajar · cualquier tecla vuelve", off, history))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m *Model) sidebar() string {
	if m.showHelp {
		return strings.Join([]string{
			labelStyle.Render("Atajos (prefijo Ctrl+])"),
			"",
			"?  mostrar/ocultar ayuda",
			"s  mostrar/ocultar panel",
			"q  salir",
			"",
			labelStyle.Render("Historial"),
			"",
			"Shift+PgUp / Shift+PgDn",
			"desplazan el historial.",
			"]  enviar Ctrl+] al shell",
			"",
			labelStyle.Render("En el shell"),
			"",
			"Todo lo demás va en bruto",
			"al PTY: Tab, ↑/↓, Ctrl+R,",
			"Ctrl+C, Alt+…, etc.",
		}, "\n")
	}
	cols, rows := m.sess.Size()
	w := sidebarWidth - 2
	line := func(k, v string) string {
		return labelStyle.Render(k) + "\n" + valueStyle.Render(truncLeft(v, w)) + "\n"
	}
	return line("Shell", m.shellPath) +
		line("PID", fmt.Sprint(m.sess.Pid())) +
		line("Tamaño", fmt.Sprintf("%dx%d", cols, rows)) +
		line("Directorio", m.cwd)
}

// truncLeft keeps the tail of s so long paths show their last segments.
func truncLeft(s string, w int) string {
	r := []rune(s)
	if len(r) <= w || w < 2 {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}
