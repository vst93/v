package plugin_qr

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func terminalImageProtocol(requested string, raw, isTTY bool) string {
	// Never send image control sequences into a file or the next pipe stage.
	if raw || !isTTY {
		return "text"
	}
	if requested != "auto" {
		return requested
	}
	term := strings.ToLower(os.Getenv("TERM"))
	// Multiplexers can inherit the outer terminal's environment without passing
	// through its image protocol. Require an explicit protocol in that case.
	if os.Getenv("TMUX") != "" || strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") || term == "dumb" {
		return "text"
	}
	switch strings.ToLower(os.Getenv("TERM_PROGRAM")) {
	case "iterm.app", "wezterm", "vscode":
		return "iterm2"
	case "kitty", "ghostty":
		return "kitty"
	}
	if term == "xterm-kitty" || term == "xterm-ghostty" || os.Getenv("KITTY_WINDOW_ID") != "" {
		return "kitty"
	}
	return "text"
}

// terminalImageSequence embeds the PNG itself, so display works over SSH and
// needs neither a temporary file nor access to the terminal's local filesystem.
func terminalImageSequence(data []byte, size int, protocol string, tmux bool) (string, error) {
	payload := base64.StdEncoding.EncodeToString(data)
	wrap := func(sequence string) string {
		if tmux {
			return "\x1bPtmux;" + strings.ReplaceAll(sequence, "\x1b", "\x1b\x1b") + "\x1b\\"
		}
		return sequence
	}
	var out strings.Builder
	switch protocol {
	case "iterm2":
		// inline=1 displays the file; omitting it would download the image instead.
		name := base64.StdEncoding.EncodeToString([]byte("qr.png"))
		out.WriteString(wrap(fmt.Sprintf("\x1b]1337;File=name=%s;size=%d;inline=1;width=%dpx;height=%dpx;preserveAspectRatio=1:%s\a", name, len(data), size, size, payload)))
	case "kitty":
		// Kitty limits each base64 payload to 4096 bytes. Suppress responses so
		// acknowledgements do not appear as keystrokes at the shell prompt.
		for start := 0; start < len(payload); start += 4096 {
			end := min(start+4096, len(payload))
			more := 0
			if end < len(payload) {
				more = 1
			}
			header := "q=2"
			if start == 0 {
				header = "a=T,f=100,t=d,q=2"
			}
			out.WriteString(wrap(fmt.Sprintf("\x1b_G%s,m=%d;%s\x1b\\", header, more, payload[start:end])))
		}
	default:
		return "", fmt.Errorf("unsupported terminal image protocol %q", protocol)
	}
	// Both protocols move the cursor past the image; start subsequent text on a
	// fresh line without clearing the image or entering an alternate screen.
	out.WriteByte('\n')
	return out.String(), nil
}
