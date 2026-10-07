package workspace

// Resizing panes: moving the divider of a split with the keyboard or by
// dragging it with the mouse.

// rect is a node's area in the pane area.
type rect struct{ x, y, w, h int }

// splits lists every split node of n with its area, outermost first.
func splits(n *node, r rect, out []splitRect) []splitRect {
	if n.sess != nil {
		return out
	}
	out = append(out, splitRect{n, r})
	if n.vertical {
		wa := n.splitAt(r.w, minPaneCols+2)
		out = splits(n.a, rect{r.x, r.y, wa, r.h}, out)
		return splits(n.b, rect{r.x + wa, r.y, r.w - wa, r.h}, out)
	}
	ha := n.splitAt(r.h, minPaneRows+2)
	out = splits(n.a, rect{r.x, r.y, r.w, ha}, out)
	return splits(n.b, rect{r.x, r.y + ha, r.w, r.h - ha}, out)
}

type splitRect struct {
	n *node
	r rect
}

// divider returns the column (vertical split) or row where b starts.
func (s splitRect) divider() int {
	if s.n.vertical {
		return s.r.x + s.n.splitAt(s.r.w, minPaneCols+2)
	}
	return s.r.y + s.n.splitAt(s.r.h, minPaneRows+2)
}

// move puts the divider at pos (b's first column or row), clamped.
func (s splitRect) move(pos int) {
	if s.n.vertical {
		s.n.setSplit(max(minPaneCols+2, min(pos-s.r.x, s.r.w-minPaneCols-2)), s.r.w)
	} else {
		s.n.setSplit(max(minPaneRows+2, min(pos-s.r.y, s.r.h-minPaneRows-2)), s.r.h)
	}
}

// Direction for ResizePane.
type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

// ResizePane moves the nearest divider around the focused pane that runs
// across dir (a vertical one for Left/Right) one step towards dir.
func (w *Workspace) ResizePane(dir Direction) {
	w.mu.Lock()
	if len(w.tabs) == 0 {
		w.mu.Unlock()
		return
	}
	t := w.tabs[w.active]
	leaf := t.root.find(t.focus)
	vertical := dir == Left || dir == Right
	var target *node
	for n := leaf; n != nil && !t.zoomed; n = n.parent {
		if n.sess == nil && n.vertical == vertical {
			target = n
			break
		}
	}
	if target == nil {
		w.mu.Unlock()
		return
	}
	for _, s := range splits(t.root, rect{0, 0, w.width, w.height}, nil) {
		if s.n != target {
			continue
		}
		size := s.r.h
		if vertical {
			size = s.r.w
		}
		step := max(size/10, 1)
		if dir == Left || dir == Up {
			step = -step
		}
		s.move(s.divider() + step)
	}
	w.mu.Unlock()
	w.relayout(t)
	w.notify()
}

// Divider is a split's border, grabbed with the mouse.
type Divider struct {
	t *tab
	n *node
}

// DividerAt returns the divider whose border cells include (x, y) in the
// pane area: the right border of a's side or the left border of b's.
func (w *Workspace) DividerAt(x, y int) (Divider, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.tabs) == 0 || w.tabs[w.active].zoomed {
		return Divider{}, false
	}
	t := w.tabs[w.active]
	ss := splits(t.root, rect{0, 0, w.width, w.height}, nil)
	for i := len(ss) - 1; i >= 0; i-- { // innermost first
		s, d := ss[i], ss[i].divider()
		if s.n.vertical && (x == d-1 || x == d) && y >= s.r.y && y < s.r.y+s.r.h ||
			!s.n.vertical && (y == d-1 || y == d) && x >= s.r.x && x < s.r.x+s.r.w {
			return Divider{t, s.n}, true
		}
	}
	return Divider{}, false
}

// MoveDivider drags d so the border follows (x, y) in the pane area.
func (w *Workspace) MoveDivider(d Divider, x, y int) {
	w.mu.Lock()
	if w.indexOf(d.t) < 0 || d.t.root == nil {
		w.mu.Unlock()
		return
	}
	moved := false
	for _, s := range splits(d.t.root, rect{0, 0, w.width, w.height}, nil) {
		if s.n == d.n {
			if s.n.vertical {
				s.move(x)
			} else {
				s.move(y)
			}
			moved = true
		}
	}
	w.mu.Unlock()
	if moved {
		w.relayout(d.t)
		w.notify()
	}
}
