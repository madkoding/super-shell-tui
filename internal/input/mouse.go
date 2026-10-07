package input

// MouseEvent is a decoded SGR (1006) mouse report from the real terminal.
type MouseEvent struct {
	// Code is the raw button code: bits 0-1 button, 4 shift, 8 alt,
	// 16 ctrl, 32 motion, 64 wheel.
	Code int
	// X, Y are 0-based cell coordinates on the real terminal.
	X, Y int
	// Release is true for a button release ('m' terminator).
	Release bool
}

// Button returns 0 (left), 1 (middle), 2 (right) or 3 (none).
func (e MouseEvent) Button() int { return e.Code & 3 }

// Motion reports a drag or hover event.
func (e MouseEvent) Motion() bool { return e.Code&32 != 0 }

// Wheel returns -1 for wheel up, 1 for wheel down, 0 otherwise.
func (e MouseEvent) Wheel() int {
	if e.Code&64 == 0 {
		return 0
	}
	if e.Code&1 == 0 {
		return -1
	}
	return 1
}

// parseSGRMouse decodes ESC [ < Cb ; Cx ; Cy (M|m) at the start of b and
// returns the event and the number of bytes consumed.
func parseSGRMouse(b []byte) (MouseEvent, int, bool) {
	if len(b) < 9 || b[0] != 0x1b || b[1] != '[' || b[2] != '<' {
		return MouseEvent{}, 0, false
	}
	var nums [3]int
	n := 0
	for i := 3; i < len(b); i++ {
		c := b[i]
		switch {
		case c >= '0' && c <= '9':
			nums[n] = nums[n]*10 + int(c-'0')
		case c == ';':
			n++
			if n > 2 {
				return MouseEvent{}, 0, false
			}
		case (c == 'M' || c == 'm') && n == 2:
			return MouseEvent{
				Code:    nums[0],
				X:       nums[1] - 1,
				Y:       nums[2] - 1,
				Release: c == 'm',
			}, i + 1, true
		default:
			return MouseEvent{}, 0, false
		}
	}
	return MouseEvent{}, 0, false
}
