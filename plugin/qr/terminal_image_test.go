package plugin_qr

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qrcode "github.com/skip2/go-qrcode"
)

func TestParseQRTerminalImageOptions(t *testing.T) {
	opts, _, err := parseQRArgs([]string{"hello", "-terminal", "-save"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.terminal || !opts.save || opts.protocol != "auto" || opts.image {
		t.Fatalf("unexpected terminal image options: %+v", opts)
	}
	for _, protocol := range []string{"auto", "iterm2", "kitty", "text"} {
		opts, _, err = parseQRArgs([]string{"hello", "-protocol", protocol})
		if err != nil || !opts.terminal || opts.protocol != protocol {
			t.Errorf("protocol %q: options=%+v, err=%v", protocol, opts, err)
		}
	}
}

func TestTerminalImageDetection(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	for _, program := range []string{"iTerm.app", "WezTerm", "vscode"} {
		t.Setenv("TERM_PROGRAM", program)
		if got := terminalImageProtocol("auto", false, true); got != "iterm2" {
			t.Errorf("%s: protocol=%q, want iterm2", program, got)
		}
	}
	for _, program := range []string{"kitty", "ghostty"} {
		t.Setenv("TERM_PROGRAM", program)
		if got := terminalImageProtocol("auto", false, true); got != "kitty" {
			t.Errorf("%s: protocol=%q, want kitty", program, got)
		}
	}
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	if got := terminalImageProtocol("auto", false, true); got != "text" {
		t.Errorf("Apple Terminal: protocol=%q, want text", got)
	}
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-kitty")
	if got := terminalImageProtocol("auto", false, true); got != "kitty" {
		t.Errorf("xterm-kitty: protocol=%q, want kitty", got)
	}
}

func TestTerminalImageDetectionRespectsOutputAndMultiplexers(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TMUX", "")
	for _, requested := range []string{"auto", "iterm2", "kitty"} {
		if got := terminalImageProtocol(requested, false, false); got != "text" {
			t.Errorf("redirected %s: protocol=%q, want text", requested, got)
		}
		if got := terminalImageProtocol(requested, true, true); got != "text" {
			t.Errorf("raw %s: protocol=%q, want text", requested, got)
		}
	}
	t.Setenv("TMUX", "/tmp/test-tmux,123,0")
	if got := terminalImageProtocol("auto", false, true); got != "text" {
		t.Errorf("tmux auto: protocol=%q, want text", got)
	}
	if got := terminalImageProtocol("iterm2", false, true); got != "iterm2" {
		t.Errorf("tmux explicit: protocol=%q, want iterm2", got)
	}
	t.Setenv("TMUX", "")
	t.Setenv("TERM", "screen-256color")
	if got := terminalImageProtocol("auto", false, true); got != "text" {
		t.Errorf("screen auto: protocol=%q, want text", got)
	}
}

func TestITermImageContainsOriginalPNG(t *testing.T) {
	code, err := qrcode.New("终端二维码 🌏", qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	data, err := code.PNG(256)
	if err != nil {
		t.Fatal(err)
	}
	sequence, err := terminalImageSequence(data, 256, "iterm2", false)
	if err != nil {
		t.Fatal(err)
	}
	header, payload, ok := strings.Cut(sequence, ":")
	if !ok || !strings.HasPrefix(header, "\x1b]1337;File=") || !strings.HasSuffix(payload, "\a\n") {
		t.Fatal("invalid iTerm2 image framing")
	}
	for _, option := range []string{"inline=1", "width=256px", "height=256px", "preserveAspectRatio=1", fmt.Sprintf("size=%d", len(data))} {
		if !strings.Contains(header, option) {
			t.Errorf("missing image option %q", option)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(payload, "\a\n"))
	if err != nil || !bytes.Equal(decoded, data) {
		t.Fatalf("image payload corrupted: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(decoded))
	if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("terminal payload is not the expected PNG: %v", err)
	}
}

func TestKittyImageChunksReassembleToOriginalPNG(t *testing.T) {
	var content strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&content, "%04x", i)
	}
	code, err := qrcode.New(content.String(), qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	data, err := code.PNG(1024)
	if err != nil {
		t.Fatal(err)
	}
	sequence, err := terminalImageSequence(data, 1024, "kitty", false)
	if err != nil {
		t.Fatal(err)
	}
	chunks := strings.Split(strings.TrimSuffix(sequence, "\x1b\\\n"), "\x1b\\")
	if len(chunks) < 2 {
		t.Fatal("fixture must exercise multi-chunk image transmission")
	}
	var payload strings.Builder
	for i, chunk := range chunks {
		header, data, ok := strings.Cut(chunk, ";")
		if !ok || !strings.HasPrefix(header, "\x1b_G") || !strings.Contains(header, "q=2") {
			t.Fatalf("chunk %d has invalid framing or enables terminal responses", i)
		}
		if len(data) > 4096 || len(data)%4 != 0 {
			t.Fatalf("chunk %d has invalid base64 length %d", i, len(data))
		}
		if i == 0 && (!strings.Contains(header, "a=T") || !strings.Contains(header, "f=100") || !strings.Contains(header, "t=d")) {
			t.Fatal("first chunk does not transmit and display a PNG directly")
		}
		more := "m=1"
		if i == len(chunks)-1 {
			more = "m=0"
		}
		if !strings.Contains(header, more) {
			t.Errorf("chunk %d missing %s", i, more)
		}
		payload.WriteString(data)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil || !bytes.Equal(decoded, data) {
		t.Fatalf("chunking corrupted the PNG: %v", err)
	}
}

func TestTerminalImageTmuxPassthrough(t *testing.T) {
	for _, protocol := range []string{"iterm2", "kitty"} {
		plain, err := terminalImageSequence([]byte("small payload"), 256, protocol, false)
		if err != nil {
			t.Fatal(err)
		}
		wrapped, err := terminalImageSequence([]byte("small payload"), 256, protocol, true)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(wrapped, "\x1bPtmux;\x1b\x1b") || !strings.HasSuffix(wrapped, "\x1b\\\n") {
			t.Fatalf("%s: invalid tmux passthrough framing", protocol)
		}
		unwrapped := strings.TrimSuffix(strings.TrimPrefix(wrapped, "\x1bPtmux;"), "\x1b\\\n")
		unwrapped = strings.ReplaceAll(unwrapped, "\x1b\x1b", "\x1b") + "\n"
		if unwrapped != plain {
			t.Errorf("%s: tmux escaping changed the image sequence", protocol)
		}
	}
}

func TestQRTerminalImageRedirectFallsBackWithoutCreatingImage(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	output := captureQRTerminalOutput(t, []string{"hello", "-protocol", "kitty"})
	if strings.Contains(output, "\x1b_G") || strings.Contains(output, "\x1b]1337") || !strings.ContainsAny(output, "█▀▄") {
		t.Fatal("redirected image should fall back to a character QR")
	}
	files, err := os.ReadDir(temp)
	if err != nil || len(files) != 0 {
		t.Fatalf("terminal display created temporary files: %v, %v", files, err)
	}
}

func TestQRTerminalImageCanAlsoSavePNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qr.png")
	output := captureQRTerminalOutput(t, []string{"你好", "-terminal", "-raw", "-out", path})
	if !strings.Contains(output, path) || !strings.ContainsAny(output, "█▀▄") || strings.Contains(output, "\x1b") {
		t.Fatal("combined mode must report the saved path and print a plain text QR")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("combined mode did not save a valid PNG: %v", err)
	}
}

func captureQRTerminalOutput(t *testing.T, args []string) string {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = file
	defer func() {
		os.Stdout = original
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := (&QR{}).Run(args); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
