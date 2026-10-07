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
	"github.com/madkoding/super-shell-tui/internal/workspace"
)

// ActionMsg carries a prefix-key action from the input pump.
type ActionMsg input.Action

// MouseMsg carries a mouse report from the input pump.
type MouseMsg input.MouseEvent

type clearFlashMsg struct{ id int }

type screenMsg struct{}

// Options are the user settings the model needs.
type Options struct {
	ShellPath    string
	PrefixLabel  string // e.g. "Ctrl+]"
	ShowSidebar  bool
	SidebarWidth int
	Accent       string // hex, empty for the default
	Muted        string
}

// Model is the root Bubble Tea model.
type Model struct {
	ws     *workspace.Workspace
	sess   *shell.Session // active tab, refreshed on every update
	opts   Options
	styles styles

	width, height int
	showSidebar   bool
	showHelp      bool
	prefixArmed   bool
	cwd           string
	selecting     bool   // left button held for a selection
	flash         string // transient status message
	flashID       int

	// Clipboard copies text to the system clipboard (OSC 52 by default).
	Clipboard func(string)
}

// New builds the model around a running shell session.
func New(ws *workspace.Workspace, opts Options) *Model {
	return &Model{
		ws:          ws,
		sess:        ws.Active(),
		opts:        opts,
		styles:      newStyles(opts.Accent, opts.Muted),
		showSidebar: opts.ShowSidebar,
	}
}

func (m *Model) Init() tea.Cmd { return m.waitScreen() }

func (m *Model) waitScreen() tea.Cmd {
	return func() tea.Msg {
		<-m.ws.Changed()
		return screenMsg{}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.sess = m.ws.Active(); m.sess == nil {
		return m, tea.Quit // the last shell exited
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeShell()
	case screenMsg:
		m.cwd = m.sess.Cwd()
		return m, m.waitScreen()
	case MouseMsg:
		return m, m.handleMouse(input.MouseEvent(msg))
	case clearFlashMsg:
		if msg.id == m.flashID {
			m.flash = ""
		}
	case ActionMsg:
		switch input.Action(msg) {
		case input.ActionPrefixArmed:
			m.prefixArmed = true
		case input.ActionPrefixDone:
			m.prefixArmed = false
		case input.ActionQuit:
			return m, tea.Quit
		case input.ActionNewTab:
			m.selecting = false
			if err := m.ws.NewTab(); err != nil {
				return m, m.setFlash("No se pudo abrir la pestaña: " + err.Error())
			}
		case input.ActionNextTab:
			m.switchTab(m.ws.Next)
		case input.ActionPrevTab:
			m.switchTab(m.ws.Prev)
		default:
			if i := input.Action(msg).SelectedTab(); i >= 0 {
				m.switchTab(func() { m.ws.Select(i) })
			}
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
		cols -= m.opts.SidebarWidth + 2
	}
	rows = m.height - 2 - 2 // header + status, pane border
	return max(cols, 1), max(rows, 1)
}

func (m *Model) resizeShell() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.ws.Resize(m.paneSize())
}

func (m *Model) View() string {
	if m.width == 0 || m.sess == nil {
		return ""
	}
	cols, rows := m.paneSize()

	st, pfx := m.styles, m.opts.PrefixLabel
	bar := func(style lipgloss.Style, text string) string {
		return style.Width(m.width).MaxWidth(m.width).Render(text)
	}

	header := m.tabBar()

	pane := st.pane.Width(cols).Height(rows).Render(m.sess.Render(true))
	body := pane
	if m.showSidebar {
		side := st.sidebar.Width(m.opts.SidebarWidth).Height(rows).Render(m.sidebar())
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, pane)
	}

	status := bar(st.status, fmt.Sprintf("%[1]s ? ayuda · %[1]s s panel · %[1]s q salir · Shift+PgUp historial", pfx))
	if m.flash != "" {
		status = bar(st.scroll, m.flash)
	} else if m.prefixArmed {
		status = bar(st.armed, fmt.Sprintf("%[1]s … ?  ayuda · c  nueva pestaña · n/p  cambiar · 1-9  ir · s  panel · q  salir · %[1]s  enviar %[1]s", pfx))
	} else if off, history := m.sess.ScrollOffset(); off > 0 {
		status = bar(st.scroll, fmt.Sprintf("Historial: %d/%d líneas arriba · Shift+PgDn bajar · cualquier tecla vuelve", off, history))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m *Model) sidebar() string {
	st, pfx := m.styles, m.opts.PrefixLabel
	if m.showHelp {
		return strings.Join([]string{
			st.label.Render("Atajos (prefijo " + pfx + ")"),
			"",
			"?  mostrar/ocultar ayuda",
			"s  mostrar/ocultar panel",
			"q  salir",
			"c  nueva pestaña",
			"n / p  siguiente / anterior",
			"1-9  ir a la pestaña",
			pfx + "  enviarlo al shell",
			"",
			st.label.Render("Historial"),
			"",
			"Shift+PgUp / Shift+PgDn",
			"o la rueda del mouse.",
			"",
			st.label.Render("Copiar"),
			"",
			"Arrastra con el mouse;",
			"se copia al soltar.",
			"",
			st.label.Render("En el shell"),
			"",
			"Todo lo demás va en bruto",
			"al PTY: Tab, ↑/↓, Ctrl+R,",
			"Ctrl+C, Alt+…, etc.",
		}, "\n")
	}
	cols, rows := m.sess.Size()
	w := m.opts.SidebarWidth - 2
	line := func(k, v string) string {
		return st.label.Render(k) + "\n" + st.value.Render(truncLeft(v, w)) + "\n"
	}
	return line("Pestaña", fmt.Sprintf("%d de %d", m.ws.ActiveIndex()+1, m.ws.Len())) +
		line("Shell", m.opts.ShellPath) +
		line("PID", fmt.Sprint(m.sess.Pid())) +
		line("Tamaño", fmt.Sprintf("%dx%d", cols, rows)) +
		line("Directorio", m.cwd)
}

// switchTab runs a tab change and drops any in-progress mouse selection.
func (m *Model) switchTab(change func()) {
	m.selecting = false
	m.sess.ClearSelection()
	change()
	m.sess = m.ws.Active()
	m.cwd = m.sess.Cwd()
}

// tabBar renders the header: the app name followed by one label per tab.
// Background tabs with new output are marked with a dot.
func (m *Model) tabBar() string {
	st := m.styles
	parts := []string{"Super Shell"}
	for i, t := range m.ws.Tabs() {
		label := fmt.Sprintf("%d:%s", i+1, t.Title)
		switch {
		case t.Active:
			label = "[" + label + "]"
		case t.Activity:
			label = " " + label + "•"
		default:
			label = " " + label + " "
		}
		parts = append(parts, label)
	}
	return st.header.Width(m.width).MaxWidth(m.width).Render(strings.Join(parts, " "))
}

// truncLeft keeps the tail of s so long paths show their last segments.
func truncLeft(s string, w int) string {
	r := []rune(s)
	if len(r) <= w || w < 2 {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}
