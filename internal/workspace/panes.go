package workspace

// Split panes: each tab holds a binary tree whose leaves are shells.

import (
	"errors"

	"github.com/madkoding/super-shell-tui/internal/shell"
)

// Minimum inner size of a pane created by a split.
const (
	minPaneCols = 10
	minPaneRows = 3
)

// ErrNoRoom is returned by Split when the new panes would be too small.
var ErrNoRoom = errors.New("not enough room to split")

// node is a pane (sess set) or a split of a and b. A vertical split puts a
// left of b; otherwise a is above b. ratio is a's share (0 means half).
type node struct {
	sess     *shell.Session
	vertical bool
	ratio    float64
	a, b     *node
	parent   *node
}

// splitAt returns the size of a's side out of total cells, keeping both
// sides at least minSide when there is room for it.
func (n *node) splitAt(total, minSide int) int {
	ratio := n.ratio
	if ratio == 0 {
		ratio = 0.5
	}
	a := int(float64(total) * ratio)
	if minSide <= total-minSide {
		a = max(minSide, min(a, total-minSide))
	}
	return a
}

// setSplit stores a share of a cells out of total; the half cell keeps
// splitAt from rounding it down.
func (n *node) setSplit(a, total int) {
	n.ratio = (float64(a) + 0.5) / float64(total)
}

// Pane is a shell's box in the pane area, borders included. X and Y are
// relative to the top-left corner of the area.
type Pane struct {
	Sess       *shell.Session
	X, Y, W, H int
	Active     bool
}

// Inner returns the shell's size inside the border.
func (p Pane) Inner() (cols, rows int) { return max(p.W-2, 1), max(p.H-2, 1) }

// layout splits the w x h area at (x, y) among the leaves of n.
func layout(n *node, x, y, w, h int, out []Pane) []Pane {
	if n.sess != nil {
		return append(out, Pane{Sess: n.sess, X: x, Y: y, W: w, H: h})
	}
	if n.vertical {
		wa := n.splitAt(w, minPaneCols+2)
		out = layout(n.a, x, y, wa, h, out)
		return layout(n.b, x+wa, y, w-wa, h, out)
	}
	ha := n.splitAt(h, minPaneRows+2)
	out = layout(n.a, x, y, w, ha, out)
	return layout(n.b, x, y+ha, w, h-ha, out)
}

// find returns the leaf holding s, or nil.
func (n *node) find(s *shell.Session) *node {
	if n == nil {
		return nil
	}
	if n.sess != nil {
		if n.sess == s {
			return n
		}
		return nil
	}
	if f := n.a.find(s); f != nil {
		return f
	}
	return n.b.find(s)
}

// leaves returns the shells of n in layout order.
func (n *node) leaves(out []*shell.Session) []*shell.Session {
	if n.sess != nil {
		return append(out, n.sess)
	}
	return n.b.leaves(n.a.leaves(out))
}

// first returns the top-left leaf.
func (n *node) first() *node {
	for n.sess == nil {
		n = n.a
	}
	return n
}

// remove takes leaf out of the tree rooted at root, its sibling taking the
// parent's place, and returns the new root (nil when leaf was the root).
func remove(root, leaf *node) *node {
	p := leaf.parent
	if p == nil {
		return nil
	}
	sib := p.a
	if sib == leaf {
		sib = p.b
	}
	sib.parent = p.parent
	switch {
	case p.parent == nil:
		return sib
	case p.parent.a == p:
		p.parent.a = sib
	default:
		p.parent.b = sib
	}
	return root
}

// panes lays out tab t in the current area; a zoomed tab shows only its
// focused pane. Caller holds w.mu.
func (w *Workspace) panes(t *tab) []Pane {
	if t.root == nil { // its last pane exited while another watcher relaid it out
		return nil
	}
	if t.zoomed {
		return []Pane{{Sess: t.focus, W: w.width, H: w.height, Active: true}}
	}
	ps := layout(t.root, 0, 0, w.width, w.height, nil)
	for i := range ps {
		ps[i].Active = ps[i].Sess == t.focus
	}
	return ps
}

// Panes lays out the active tab's panes, for rendering and mouse hits.
func (w *Workspace) Panes() []Pane {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.tabs) == 0 {
		return nil
	}
	return w.panes(w.tabs[w.active])
}

// PaneCount returns how many panes the active tab has, hidden ones included.
func (w *Workspace) PaneCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.tabs) == 0 {
		return 0
	}
	return len(w.tabs[w.active].root.leaves(nil))
}

// ToggleZoom shows the focused pane over the whole area, or restores the
// layout. A tab with a single pane is never zoomed.
func (w *Workspace) ToggleZoom() {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return
	}
	t := w.tabs[w.active]
	t.zoomed = !t.zoomed && t.root.sess == nil
	w.mu.Unlock()
	w.relayout(t)
	w.notify()
}

// unzoom restores the layout of t before a change that needs it. Caller
// holds w.mu and must relayout t afterwards.
func unzoom(t *tab) { t.zoomed = false }

// relayout resizes every shell of t to its pane. Caller must not hold w.mu.
func (w *Workspace) relayout(t *tab) {
	w.mu.Lock()
	ps := w.panes(t)
	w.mu.Unlock()
	for _, p := range ps {
		_ = p.Sess.Resize(p.Inner())
	}
}

// Split divides the focused pane of the active tab in two, side by side
// when vertical, and focuses a new shell started in the same directory.
func (w *Workspace) Split(vertical bool) error {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return nil
	}
	t := w.tabs[w.active]
	unzoom(t)
	var cur Pane
	for _, p := range w.panes(t) {
		if p.Active {
			cur = p
		}
	}
	w.mu.Unlock()

	// The first half gets the smaller share; both must stay usable.
	cols, rows := cur.Inner()
	if vertical {
		if cur.W/2-2 < minPaneCols {
			return ErrNoRoom
		}
		cols = cur.W - cur.W/2 - 2
	} else {
		if cur.H/2-2 < minPaneRows {
			return ErrNoRoom
		}
		rows = cur.H - cur.H/2 - 2
	}
	sess, err := shell.Start(w.shellPath, cur.Sess.FreshCwd(), cols, rows, w.scrollback)
	if err != nil {
		return err
	}

	w.mu.Lock()
	leaf := t.root.find(cur.Sess)
	if leaf == nil { // the pane exited meanwhile
		w.mu.Unlock()
		_ = sess.Close()
		return nil
	}
	old := &node{sess: leaf.sess}
	added := &node{sess: sess}
	leaf.sess, leaf.vertical, leaf.a, leaf.b = nil, vertical, old, added
	old.parent, added.parent = leaf, leaf
	t.focus = sess
	w.mu.Unlock()

	go w.watch(t, sess)
	w.relayout(t)
	w.notify()
	return nil
}

// FocusNext moves the focus to the next pane of the active tab.
func (w *Workspace) FocusNext() {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return
	}
	t := w.tabs[w.active]
	zoomed := t.zoomed
	unzoom(t)
	ps := w.panes(t)
	for i, p := range ps {
		if p.Active {
			t.focus = ps[(i+1)%len(ps)].Sess
			break
		}
	}
	w.mu.Unlock()
	if zoomed {
		w.relayout(t)
	}
	w.notify()
}

// Focus makes s the focused pane of the active tab, if it belongs to it.
func (w *Workspace) Focus(s *shell.Session) {
	w.mu.Lock()
	if len(w.tabs) > 0 {
		if t := w.tabs[w.active]; t.root.find(s) != nil {
			t.focus = s
		}
	}
	w.mu.Unlock()
	w.notify()
}

// FocusPane moves the focus to the pane next to the focused one towards
// dir: the one sharing the longest border with it, the topmost or leftmost
// on a tie. Nothing changes when there is no pane that way.
func (w *Workspace) FocusPane(dir Direction) {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return
	}
	t := w.tabs[w.active]
	zoomed := t.zoomed
	unzoom(t)
	ps := w.panes(t)
	var cur Pane
	for _, p := range ps {
		if p.Active {
			cur = p
		}
	}
	best, bestShared := (*Pane)(nil), 0
	for i, p := range ps {
		var touches bool
		var shared int
		switch dir {
		case Left, Right:
			touches = dir == Left && p.X+p.W == cur.X || dir == Right && cur.X+cur.W == p.X
			shared = min(p.Y+p.H, cur.Y+cur.H) - max(p.Y, cur.Y)
		default:
			touches = dir == Up && p.Y+p.H == cur.Y || dir == Down && cur.Y+cur.H == p.Y
			shared = min(p.X+p.W, cur.X+cur.W) - max(p.X, cur.X)
		}
		if touches && shared > bestShared {
			best, bestShared = &ps[i], shared
		}
	}
	if best != nil {
		t.focus = best.Sess
	}
	w.mu.Unlock()
	if zoomed {
		w.relayout(t)
	}
	w.notify()
}

// SwapPane exchanges the focused pane with the next one in layout order
// (the previous one when back is set), wrapping around. The focus stays on
// the moved shell.
func (w *Workspace) SwapPane(back bool) {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return
	}
	t := w.tabs[w.active]
	all := t.root.leaves(nil)
	if len(all) < 2 {
		w.mu.Unlock()
		return
	}
	unzoom(t)
	i := 0
	for i < len(all) && all[i] != t.focus {
		i++
	}
	j := (i + 1) % len(all)
	if back {
		j = (i - 1 + len(all)) % len(all)
	}
	a, b := t.root.find(all[i]), t.root.find(all[j])
	a.sess, b.sess = b.sess, a.sess
	w.mu.Unlock()
	w.relayout(t)
	w.notify()
}
