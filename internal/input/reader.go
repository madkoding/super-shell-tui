package input

import (
	"errors"
	"io"
	"os"
)

// Pump reads raw bytes from r, forwards them to w (the PTY) and reports
// prefix actions through onAction. It returns when r fails or is closed.
func Pump(r io.Reader, w io.Writer, tr *Translator, onAction func(Action)) error {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			out, actions := tr.Feed(buf[:n])
			if len(out) > 0 {
				if _, werr := w.Write(out); werr != nil {
					return werr
				}
			}
			for _, a := range actions {
				onAction(a)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				return nil
			}
			return err
		}
	}
}
