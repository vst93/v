//go:build !unix

package theme

import (
	"github.com/gdamore/tcell/v2"
)

// queryTerminalBackground is a no-op on platforms without /dev/tty (e.g.
// Windows); theme detection there falls back to COLORFGBG and V_THEME.
func queryTerminalBackground() (tcell.Color, bool) {
	return 0, false
}
