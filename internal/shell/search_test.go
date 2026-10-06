package shell

import "testing"

func TestFind(t *testing.T) {
	// 6 lines on a 2-row screen: lines 0-3 are history.
	s := newTestSession(t, 12, 2, "foo one\r\nbar\r\nFoo two\r\nbaz\r\nfoo three\r\nend")

	if !s.Find("foo", SearchHere) || s.found != (point{0, 4}) {
		t.Fatalf("first match at %+v, want line 4", s.found)
	}
	if off, _ := s.ScrollOffset(); off != 0 {
		t.Fatalf("visible match should not scroll, offset %d", off)
	}
	if !s.Find("foo", SearchOlder) || s.found.Line != 2 {
		t.Fatalf("older match at %+v, want line 2 (case folded)", s.found)
	}
	if off, _ := s.ScrollOffset(); off == 0 {
		t.Fatal("match in history should scroll the view")
	}
	if !s.Find("foo", SearchOlder) || s.found.Line != 0 {
		t.Fatalf("older match at %+v, want line 0", s.found)
	}
	if s.Find("foo", SearchOlder) || s.found.Line != 0 {
		t.Fatal("no match older than line 0")
	}
	if !s.Find("foo", SearchNewer) || s.found.Line != 2 {
		t.Fatalf("newer match at %+v, want line 2", s.found)
	}
	if !s.Find("Foo", SearchHere) || s.found.Line != 2 {
		t.Fatalf("upper case query is exact, got %+v", s.found)
	}
	if got := s.SelectedText(); got != "Foo" {
		t.Fatalf("match highlight covers %q", got)
	}
	if s.Find("nope", SearchHere) {
		t.Fatal("unexpected match")
	}
	s.EndSearch(false)
	if off, _ := s.ScrollOffset(); off != 0 || s.sel.active {
		t.Fatalf("cancel should return live and clear, offset %d", off)
	}
}

func TestFindWideChars(t *testing.T) {
	s := newTestSession(t, 10, 2, "漢字 ok")
	if !s.Find("ok", SearchHere) || s.found != (point{5, 0}) {
		t.Fatalf("match at %+v, want x=5", s.found)
	}
}
