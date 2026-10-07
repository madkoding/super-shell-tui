// Package ui lays out the Super Shell TUI: header, optional sidebar and the
// shell pane. Keyboard input does not go through Bubble Tea (see package
// input); the model only reacts to resizes, screen updates and actions.
package ui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	Keys         input.Bindings
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
	resizeMode    bool // arrows and H/J/K/L resize until another key
	cwd           string
	selecting     bool // left button held for a selection
	dragging      *workspace.Divider
	flash         string // transient status message
	flashID       int
	renaming      bool
	renameBuf     []rune
	confirmClose  bool // waiting for y/n before closing the active tab
	searching     bool // typing a scrollback search
	searchBuf     []rune
	searchMiss    bool // the last search found nothing

	// Clipboard copies text to the system clipboard (OSC 52 by default).
	Clipboard func(string)
	// SetCapture diverts keyboard input to the UI (as TextMsg) while true.
	SetCapture func(bool)
}

// New builds the model around a running shell session.
func New(ws *workspace.Workspace, opts Options) *Model {
	if opts.Keys.Empty() {
		opts.Keys = input.DefaultBindings()
	}
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
	case TextMsg:
		switch {
		case m.renaming:
			m.handleRenameInput(msg)
		case m.confirmClose:
			m.handleCloseInput(msg)
		case m.searching:
			m.handleSearchInput(msg)
		}
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
			m.resizeMode = false
		case input.ActionResizeMode:
			m.resizeMode = true
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
		case input.ActionRenameTab:
			m.startRename()
		case input.ActionCloseTab:
			m.startClose()
		case input.ActionSearch:
			return m, m.startSearch()
		case input.ActionSplitRight, input.ActionSplitDown:
			return m, m.split(input.Action(msg) == input.ActionSplitRight)
		case input.ActionNextPane:
			m.changeFocus(m.ws.FocusNext)
		case input.ActionFocusLeft:
			m.changeFocus(func() { m.ws.FocusPane(workspace.Left) })
		case input.ActionFocusRight:
			m.changeFocus(func() { m.ws.FocusPane(workspace.Right) })
		case input.ActionFocusUp:
			m.changeFocus(func() { m.ws.FocusPane(workspace.Up) })
		case input.ActionFocusDown:
			m.changeFocus(func() { m.ws.FocusPane(workspace.Down) })
		case input.ActionResizeLeft:
			m.ws.ResizePane(workspace.Left)
		case input.ActionResizeRight:
			m.ws.ResizePane(workspace.Right)
		case input.ActionResizeUp:
			m.ws.ResizePane(workspace.Up)
		case input.ActionResizeDown:
			m.ws.ResizePane(workspace.Down)
		case input.ActionSwapPaneNext, input.ActionSwapPanePrev:
			m.ws.SwapPane(input.Action(msg) == input.ActionSwapPanePrev)
		case input.ActionEqualizePanes:
			m.ws.EqualizePanes()
		case input.ActionZoomPane:
			if m.ws.PaneCount() < 2 {
				return m, m.setFlash("El zoom necesita al menos dos paneles (" + m.opts.PrefixLabel + " " + m.key(input.ActionSplitRight) + " para dividir)")
			}
			m.ws.ToggleZoom()
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

// areaSize returns the size of the pane area, pane borders included.
func (m *Model) areaSize() (width, height int) {
	width = m.width
	if m.showSidebar {
		width -= m.opts.SidebarWidth + 2
	}
	return max(width, 3), max(m.height-2, 3) // header + status
}

// areaOrigin returns the screen cell of the pane area's top-left corner.
func (m *Model) areaOrigin() (x, y int) {
	if m.showSidebar {
		x = m.opts.SidebarWidth + 2
	}
	return x, 1 // below the header
}

func (m *Model) resizeShell() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.ws.Resize(m.areaSize())
}

// renderPanes draws the active tab's panes, each in its own box; the focused
// one has the accent border and the cursor.
func (m *Model) renderPanes() string {
	width, height := m.areaSize()
	panes := m.ws.Panes()
	lines := make([][]string, len(panes))
	for i, p := range panes {
		style := m.styles.pane
		if !p.Active && len(panes) > 1 {
			style = m.styles.paneIdle
		}
		cols, rows := p.Inner()
		box := style.Width(cols).Height(rows).Render(p.Sess.Render(p.Active))
		lines[i] = strings.Split(box, "\n")
		if len(panes) > 1 { // a lone pane is already named by its tab
			lines[i][0] = topBorder(style, p.W, p.Sess.Label())
		}
	}
	// The panes tile the area, so each screen row is the concatenation of
	// the boxes crossing it, left to right (layout order already is).
	out := make([]string, height)
	for y := range out {
		var b strings.Builder
		for i, p := range panes {
			if y >= p.Y && y < p.Y+p.H && y-p.Y < len(lines[i]) {
				b.WriteString(lines[i][y-p.Y])
			}
		}
		out[y] = b.String()
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(out, "\n"))
}

// topBorder draws a pane's top border w cells wide with label set in it,
// cut to fit.
func topBorder(style lipgloss.Style, w int, label string) string {
	b := style.GetBorderStyle()
	inner := max(w-2, 0)
	line := strings.Repeat(b.Top, inner)
	if label != "" && inner >= 5 {
		label = " " + ansi.Truncate(label, inner-3, "…") + " "
		line = b.Top + label + strings.Repeat(b.Top, inner-1-ansi.StringWidth(label))
	}
	return lipgloss.NewStyle().Foreground(style.GetBorderTopForeground()).Render(b.TopLeft + line + b.TopRight)
}

func (m *Model) View() string {
	if m.width == 0 || m.sess == nil {
		return ""
	}
	_, height := m.areaSize()

	st, pfx := m.styles, m.opts.PrefixLabel
	bar := func(style lipgloss.Style, text string) string {
		return style.Width(m.width).MaxWidth(m.width).Render(text)
	}

	header := m.tabBar()

	pane := m.renderPanes()
	body := pane
	if m.showSidebar {
		side := st.sidebar.Width(m.opts.SidebarWidth).Height(height - 2).Render(m.sidebar())
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, pane)
	}

	status := bar(st.status, fmt.Sprintf("%[1]s ? ayuda · %[1]s s panel · %[1]s q salir · Shift+PgUp historial", pfx))
	if m.renaming {
		status = bar(st.armed, "Nombre de la pestaña: "+string(m.renameBuf)+"█  · Enter guardar · Esc cancelar · vacío = automático")
	} else if m.confirmClose {
		what := fmt.Sprintf("la pestaña %d", m.ws.ActiveIndex()+1)
		if m.ws.PaneCount() > 1 {
			what = "este panel"
		}
		running := "su shell"
		if cmd := m.sess.ForegroundCommand(); cmd != "" {
			running = cmd
		}
		status = bar(st.armed, "¿Cerrar "+what+" y terminar "+running+"? y = sí · cualquier otra tecla = no")
	} else if m.searching {
		miss := ""
		if m.searchMiss {
			miss = "  (sin coincidencias)"
		}
		status = bar(st.armed, "Buscar: "+string(m.searchBuf)+"█"+miss+"  · ↑/↓ anterior/siguiente · Enter quedarse aquí · Esc cancelar")
	} else if m.flash != "" {
		status = bar(st.scroll, m.flash)
	} else if m.resizeMode {
		status = bar(st.armed, "Tamaño: flechas o H/J/K/L mueven el borde · Enter/Esc o cualquier otra tecla terminan")
	} else if m.prefixArmed {
		k := m.key
		status = bar(st.armed, fmt.Sprintf("%[1]s … %s  ayuda · %s  nueva · %s  cerrar · %s/%s  cambiar · 1-9  ir · %s  renombrar · %s  buscar · %s/%s  dividir · %s/%s%s%s%s  panel · %s  zoom · %s  igualar · %s/%s  mover · flechas  tamaño · %s  panel · %s  salir · %[1]s  enviar %[1]s",
			pfx, k(input.ActionToggleHelp), k(input.ActionNewTab), k(input.ActionCloseTab), k(input.ActionNextTab), k(input.ActionPrevTab),
			k(input.ActionRenameTab), k(input.ActionSearch), k(input.ActionSplitRight), k(input.ActionSplitDown),
			k(input.ActionNextPane), k(input.ActionFocusLeft), k(input.ActionFocusDown), k(input.ActionFocusUp), k(input.ActionFocusRight),
			k(input.ActionZoomPane), k(input.ActionEqualizePanes), k(input.ActionSwapPanePrev), k(input.ActionSwapPaneNext),
			k(input.ActionToggleSidebar), k(input.ActionQuit)))
	} else if off, history := m.sess.ScrollOffset(); off > 0 {
		status = bar(st.scroll, fmt.Sprintf("Historial: %d/%d líneas arriba · Shift+PgDn bajar · cualquier tecla vuelve", off, history))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

// key returns the key bound to a for help text, "·" when it has none.
func (m *Model) key(a input.Action) string {
	if k := m.opts.Keys.Key(a); k != "" {
		return k
	}
	return "·"
}

func (m *Model) sidebar() string {
	st, pfx, k := m.styles, m.opts.PrefixLabel, m.key
	if m.showHelp {
		return strings.Join([]string{
			st.label.Render("Atajos (prefijo " + pfx + ")"),
			"",
			k(input.ActionToggleHelp) + "  mostrar/ocultar ayuda",
			k(input.ActionToggleSidebar) + "  mostrar/ocultar panel",
			k(input.ActionQuit) + "  salir",
			k(input.ActionNewTab) + "  nueva pestaña",
			k(input.ActionNextTab) + " / " + k(input.ActionPrevTab) + "  siguiente / anterior",
			"1-9  ir a la pestaña",
			k(input.ActionRenameTab) + "  renombrar pestaña",
			k(input.ActionCloseTab) + "  cerrar panel o pestaña",
			k(input.ActionSplitRight) + "  dividir a la derecha",
			k(input.ActionSplitDown) + "  dividir hacia abajo",
			k(input.ActionNextPane) + "  siguiente panel",
			k(input.ActionFocusLeft) + " " + k(input.ActionFocusDown) + " " + k(input.ActionFocusUp) + " " + k(input.ActionFocusRight) + "  panel en esa dirección",
			k(input.ActionZoomPane) + "  zoom del panel",
			k(input.ActionEqualizePanes) + "  paneles del mismo tamaño",
			k(input.ActionSwapPanePrev) + " " + k(input.ActionSwapPaneNext) + "  mover el panel",
			"flechas  cambiar tamaño",
			"(se repiten sin prefijo;",
			"Esc termina, o arrastra",
			"el borde con el mouse)",
			k(input.ActionSearch) + "  buscar en el historial",
			pfx + "  enviarlo al shell",
			"",
			st.label.Render("Historial"),
			"",
			"Shift+PgUp / Shift+PgDn",
			"o la rueda del mouse.",
			"Prefijo + " + k(input.ActionSearch) + " busca texto;",
			"↑/↓ salta entre resultados.",
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
	tabInfo := fmt.Sprintf("%d de %d", m.ws.ActiveIndex()+1, m.ws.Len())
	if n := m.ws.PaneCount(); n > 1 {
		tabInfo += fmt.Sprintf(" · %d paneles", n)
	}
	return line("Pestaña", tabInfo) +
		line("Shell", m.opts.ShellPath) +
		line("PID", fmt.Sprint(m.sess.Pid())) +
		line("Tamaño", fmt.Sprintf("%dx%d", cols, rows)) +
		line("Directorio", m.cwd)
}

// split divides the focused pane, side by side when right is set.
func (m *Model) split(right bool) tea.Cmd {
	var err error
	m.changeFocus(func() { err = m.ws.Split(right) })
	switch {
	case errors.Is(err, workspace.ErrNoRoom):
		return m.setFlash("No hay espacio para dividir este panel")
	case err != nil:
		return m.setFlash("No se pudo dividir el panel: " + err.Error())
	}
	return nil
}

// changeFocus runs a pane focus change and drops the old pane's selection.
func (m *Model) changeFocus(change func()) {
	m.selecting = false
	m.sess.ClearSelection()
	change()
	m.sess = m.ws.Active()
	m.cwd = m.sess.Cwd()
}

// switchTab runs a tab change and drops any in-progress mouse selection.
func (m *Model) switchTab(change func()) {
	if m.renaming {
		m.endRename(false)
	}
	if m.confirmClose {
		m.endClose(false)
	}
	if m.searching {
		m.endSearch(false)
	}
	m.selecting = false
	m.sess.ClearSelection()
	change()
	m.sess = m.ws.Active()
	m.cwd = m.sess.Cwd()
}

// tabBar renders the header: the app name followed by one label per tab.
// Background tabs with new output are marked with a dot.
func (m *Model) tabBar() string {
	text, _ := tabLayout(m.ws.Tabs())
	return m.styles.header.Width(m.width).MaxWidth(m.width).Render(text)
}

// headerPad is the header's left padding, where tab labels start counting.
const headerPad = 1

// tabLayout builds the header text and the screen column range [from, to)
// of each tab label, used to switch tabs with a click.
func tabLayout(tabs []workspace.Tab) (string, [][2]int) {
	var b strings.Builder
	b.WriteString("Super Shell")
	spans := make([][2]int, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%d:%s", i+1, t.Title)
		if t.Zoomed {
			label += " (zoom)"
		}
		switch {
		case t.Active:
			label = "[" + label + "]"
		case t.Activity:
			label = " " + label + "•"
		default:
			label = " " + label + " "
		}
		b.WriteString(" ")
		from := headerPad + lipgloss.Width(b.String())
		b.WriteString(label)
		spans[i] = [2]int{from, from + lipgloss.Width(label)}
	}
	return b.String(), spans
}

// tabAt returns the tab under header column x, or -1.
func (m *Model) tabAt(x int) int {
	_, spans := tabLayout(m.ws.Tabs())
	for i, sp := range spans {
		if x >= sp[0] && x < sp[1] {
			return i
		}
	}
	return -1
}

// truncLeft keeps the tail of s so long paths show their last segments.
func truncLeft(s string, w int) string {
	r := []rune(s)
	if len(r) <= w || w < 2 {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}
