package shell

import (
	"strings"
	"unicode"
)

// Search directions for Find.
const (
	SearchHere  = 0  // the current match if it still matches, else an older one
	SearchOlder = 1  // the previous match, towards the oldest history line
	SearchNewer = -1 // the next match, towards the live screen
)

// lineText returns the text of an absolute line and, for each rune, the
// column of its first and last cell. Caller holds s.mu.
func (s *Session) lineText(line int) ([]rune, [][2]int) {
	var runes []rune
	var cols [][2]int
	for x := 0; x < s.cols; x++ {
		c := s.cellAtAbs(x, line)
		content, width := " ", 1
		if c != nil {
			if c.IsZero() { // second half of a wide char
				continue
			}
			if c.Content != "" {
				content = c.Content
			}
			width = max(c.Width, 1)
		}
		for _, r := range content {
			runes = append(runes, r)
			cols = append(cols, [2]int{x, min(x+width-1, s.cols-1)})
		}
	}
	return runes, cols
}

// matchesIn returns the start index of every match of q in text.
func matchesIn(text, q []rune, fold bool) []int {
	var out []int
	for i := 0; i+len(q) <= len(text); i++ {
		ok := true
		for j, r := range q {
			t := text[i+j]
			if fold {
				t = unicode.ToLower(t)
			}
			if t != r {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// Find searches history and screen for query and highlights the match,
// scrolling it into view. Matching ignores case unless query has an upper
// case letter. dir is SearchHere, SearchOlder or SearchNewer, relative to
// the current match (or to the bottom of the screen when there is none).
// It reports whether a match was found; when not, the view is unchanged.
func (s *Session) Find(query string, dir int) bool {
	if query == "" {
		s.EndSearch(true)
		return false
	}
	fold := strings.ToLower(query) == query
	q := []rune(query)

	s.mu.Lock()
	history := s.emu.ScrollbackLen()
	total := history + s.rows
	cur := s.found
	if !s.searching {
		cur = point{X: s.cols, Line: total - 1} // start after the last cell
	}

	var hit point
	var hitEnd int
	found := false
	if dir == SearchNewer {
		for line := cur.Line; line < total && !found; line++ {
			text, cols := s.lineText(line)
			for _, i := range matchesIn(text, q, fold) {
				p := point{cols[i][0], line}
				if cur.before(p) {
					hit, hitEnd, found = p, cols[i+len(q)-1][1], true
					break
				}
			}
		}
	} else {
		for line := min(cur.Line, total-1); line >= 0 && !found; line-- {
			text, cols := s.lineText(line)
			ms := matchesIn(text, q, fold)
			for k := len(ms) - 1; k >= 0; k-- {
				i := ms[k]
				p := point{cols[i][0], line}
				if p.before(cur) || (dir == SearchHere && s.searching && p == cur) {
					hit, hitEnd, found = p, cols[i+len(q)-1][1], true
					break
				}
			}
		}
	}
	if found {
		s.searching = true
		s.found = hit
		s.sel = selection{active: true, anchor: hit, head: point{hitEnd, hit.Line}}
		// Scroll only when the match is off screen, centering it.
		if top := history - s.scroll; hit.Line < top || hit.Line >= top+s.rows {
			s.scroll = max(0, min(history, history-(hit.Line-s.rows/2)))
		}
	}
	s.mu.Unlock()
	if found {
		s.notify()
	}
	return found
}

// EndSearch drops the search highlight. With keep false the view also
// returns to the live screen.
func (s *Session) EndSearch(keep bool) {
	s.mu.Lock()
	s.searching = false
	s.sel = selection{}
	if !keep {
		s.scroll = 0
	}
	s.mu.Unlock()
	s.notify()
}
