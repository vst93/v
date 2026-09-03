//go:build unix

package theme

import (
	"bytes"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// queryTerminalBackground asks the terminal for its background color via the
// OSC 11 device query ("\x1b]11;?\x1b\\"). /dev/tty is opened directly so
// piped stdin does not matter, switched to raw mode for the exchange, and
// the reply is read with a hard deadline. Returns ok=false when the terminal
// is unavailable or does not answer.
func queryTerminalBackground() (tcell.Color, bool) {
	fd, err := syscall.Open("/dev/tty", syscall.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return 0, false
	}
	defer syscall.Close(fd)

	state, err := term.GetState(fd)
	if err != nil {
		return 0, false
	}
	if _, err := term.MakeRaw(fd); err != nil {
		return 0, false
	}
	defer term.Restore(fd, state)

	if _, err := syscall.Write(fd, []byte("\x1b]11;?\x1b\\")); err != nil {
		return 0, false
	}

	buf := make([]byte, 0, 128)
	tmp := make([]byte, 64)
	deadline := time.Now().Add(probeBudget)
	for {
		n, err := syscall.Read(fd, tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if c, ok := parseOSC11(buf); ok {
				return c, true
			}
		}
		if err != nil && err != syscall.EAGAIN && err != syscall.EINTR {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Last chance: a reply that arrived without a terminator.
	return parseOSC11(buf)
}

// parseOSC11 extracts the background color from an "OSC 11 ; <value>" reply.
// Accepts rgb:R/G/B with 1-4 hex digits per channel, BEL or ST terminated
// (or unterminated), and a bare palette index.
func parseOSC11(buf []byte) (tcell.Color, bool) {
	start := bytes.Index(buf, []byte("\x1b]11;"))
	if start < 0 {
		return 0, false
	}
	rest := buf[start+5:]
	val := rest
	if i := bytes.IndexByte(rest, 0x07); i >= 0 { // BEL terminator
		val = rest[:i]
	} else if i := bytes.Index(rest, []byte("\x1b\\")); i >= 0 { // ST terminator
		val = rest[:i]
	}
	val = bytes.TrimSpace(val)

	if color, ok := parseRGBReply(string(bytes.ToLower(val))); ok {
		return color, true
	}
	// Some terminals reply with a plain palette index instead of rgb.
	if n, err := strconv.Atoi(string(val)); err == nil && n >= 0 && n <= 255 {
		r, g, b := ansiColorRGB(n)
		return tcell.NewRGBColor(int32(r), int32(g), int32(b)), true
	}
	return 0, false
}

// parseRGBReply parses "rgb:RR/GG/BB" style payloads. Channels may use 1-4
// hex digits; longer ones are scaled down to 0-255.
func parseRGBReply(val string) (tcell.Color, bool) {
	if !strings.HasPrefix(val, "rgb:") {
		return 0, false
	}
	parts := strings.SplitN(val[4:], "/", 3)
	if len(parts) != 3 {
		return 0, false
	}
	var rgbv [3]int
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 32)
		if err != nil {
			return 0, false
		}
		switch len(p) {
		case 1:
			rgbv[i] = int(v) * 17
		case 2:
			rgbv[i] = int(v)
		case 3:
			rgbv[i] = int(v >> 4)
		default:
			rgbv[i] = int(v >> 8)
		}
		if rgbv[i] > 255 {
			rgbv[i] = 255
		}
	}
	return tcell.NewRGBColor(int32(rgbv[0]), int32(rgbv[1]), int32(rgbv[2])), true
}
