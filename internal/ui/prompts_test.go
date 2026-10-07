package ui

import "testing"

func TestHandleRenameInput(t *testing.T) {
	var captured []bool
	m := &Model{renaming: true, SetCapture: func(on bool) { captured = append(captured, on) }}

	m.handleRenameInput([]byte("apí"))
	m.handleRenameInput([]byte("\x1b[A")) // arrow: ignored
	m.handleRenameInput([]byte{0x7f})     // backspace
	m.handleRenameInput([]byte("x"))
	if string(m.renameBuf) != "apx" || !m.renaming {
		t.Fatalf("buf %q renaming %v", string(m.renameBuf), m.renaming)
	}
	m.handleRenameInput([]byte{0x15}) // Ctrl+U
	if len(m.renameBuf) != 0 {
		t.Fatalf("Ctrl+U should clear, got %q", string(m.renameBuf))
	}
	for range 30 {
		m.handleRenameInput([]byte("a"))
	}
	if len(m.renameBuf) != maxTabName {
		t.Fatalf("name should be capped, got %d", len(m.renameBuf))
	}
	m.handleRenameInput([]byte{0x1b}) // Esc cancels without saving
	if m.renaming || len(captured) != 1 || captured[0] {
		t.Fatalf("renaming %v captured %v", m.renaming, captured)
	}
}

func TestHandleCloseInputCancels(t *testing.T) {
	var captured []bool
	m := &Model{SetCapture: func(on bool) { captured = append(captured, on) }}
	m.startClose()
	m.handleCloseInput([]byte("n")) // any key but y/s cancels; ws is never touched
	if m.confirmClose || len(captured) != 2 || captured[1] {
		t.Fatalf("confirm %v captured %v", m.confirmClose, captured)
	}
}
