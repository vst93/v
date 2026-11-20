package plugin_qr

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qrcode "github.com/skip2/go-qrcode"
)

func TestParseQRArgsOutputModes(t *testing.T) {
	opts, help, err := parseQRArgs([]string{"hello", "-image", "-save", "-out", "qr.png", "-size", "512", "-raw"})
	if err != nil || help {
		t.Fatalf("parse: help=%v, err=%v", help, err)
	}
	if !opts.image || !opts.save || opts.out != "qr.png" || opts.size != 512 || !opts.raw {
		t.Fatalf("unexpected output options: %+v", opts)
	}
	defaults, _, err := parseQRArgs([]string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.image || defaults.save || defaults.out != "" || defaults.size != 256 {
		t.Fatalf("unexpected default output: %+v", defaults)
	}
}

func TestParseQRArgsRejectsInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"-file"}, {"-out"}, {"-size"}, {"-pipe"}, {"-protocol"}, {"-protocol", "invalid"},
		{"-file", "-save"}, {"-out", ""}, {"-unknown"},
		{"-size", "0"}, {"-size", "-1"}, {"-size", "4097"}, {"-size", "1.5"}, {"-size", "huge"},
	} {
		if _, _, err := parseQRArgs(args); err == nil {
			t.Errorf("accepted invalid args %q", args)
		}
	}
}

func TestQRHelpAliases(t *testing.T) {
	for _, flag := range []string{"-h", "-help", "--help"} {
		if _, help, err := parseQRArgs([]string{flag}); err != nil || !help {
			t.Errorf("%s: help=%v, err=%v", flag, help, err)
		}
	}
}

func TestQRInputPriorityAndWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	fileText := "  你好，世界\nsecond line\t "
	if err := os.WriteFile(path, []byte(fileText), 0o600); err != nil {
		t.Fatal(err)
	}
	opts, _, err := parseQRArgs([]string{"positional", "-clip", "-file", path, "-pipe", "-save\n"})
	if err != nil {
		t.Fatal(err)
	}
	text, err := resolveQRInput(opts)
	if err != nil || text != "-save\n" {
		t.Fatalf("pipe should win and preserve flag-like data: %q, %v", text, err)
	}
	opts.hasPipe = false
	text, err = resolveQRInput(opts)
	if err != nil || text != fileText {
		t.Fatalf("file should win over argument and clipboard: %q, %v", text, err)
	}
	opts.file = ""
	text, err = resolveQRInput(opts)
	if err != nil || text != "positional" {
		t.Fatalf("argument should win over clipboard: %q, %v", text, err)
	}
}

func TestQREmptyInputDoesNotFallBack(t *testing.T) {
	q := &QR{}
	for _, args := range [][]string{{""}, {"fallback", "-pipe", ""}} {
		if err := q.Run(args); err == nil || !strings.Contains(err.Error(), "no text") {
			t.Errorf("%q: want empty input error, got %v", args, err)
		}
	}
}

func TestQRFileErrorIsWrapped(t *testing.T) {
	_, err := resolveQRInput(qrOptions{file: filepath.Join(t.TempDir(), "missing.txt"), positional: []string{"fallback"}})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want file error without falling back, got %v", err)
	}
}

func TestTerminalQRPreservesModulesAndQuietZone(t *testing.T) {
	code, err := qrcode.New("https://example.com/你好", qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	text := terminalQR(code, true)
	if strings.Contains(text, "\x1b") {
		t.Fatal("raw terminal output contains ANSI escapes")
	}
	bits := code.Bitmap()
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) != (len(bits)+1)/2 {
		t.Fatalf("got %d terminal rows for %d module rows", len(lines), len(bits))
	}
	for row, line := range lines {
		cells := []rune(line)
		if len(cells) != len(bits) {
			t.Fatalf("row %d width=%d, want %d", row, len(cells), len(bits))
		}
		for x, cell := range cells {
			var topDark, bottomDark bool
			switch cell {
			case ' ':
				topDark, bottomDark = true, true
			case '█':
			case '▀':
				bottomDark = true
			case '▄':
				topDark = true
			default:
				t.Fatalf("unexpected terminal glyph %q", cell)
			}
			y := row * 2
			if topDark != bits[y][x] || (y+1 < len(bits) && bottomDark != bits[y+1][x]) {
				t.Fatalf("terminal output changed QR modules at (%d, %d)", x, y)
			}
		}
	}
}

func TestQRRunSavesPNGWithSelectedContent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nested folder", "二维码.png")
	input := "  你好 🌏\nhttps://example.com?a=1&b=2\n"
	q := &QR{}
	// Explicit output overrides Downloads, and pipe input overrides arguments.
	if err := q.Run([]string{"ignored", "-save", "-out", out, "-size", "256", "-pipe", input}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a PNG: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("unexpected PNG bounds: %v", img.Bounds())
	}
	code, err := qrcode.New(input, qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	bits := code.Bitmap()
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			want := uint32(0xffff)
			if bits[y*len(bits)/256][x*len(bits)/256] {
				want = 0
			}
			if r != want || g != want || b != want || a != 0xffff {
				t.Fatalf("incorrect PNG module/color at (%d, %d): %x %x %x %x", x, y, r, g, b, a)
			}
		}
	}
	// A standard QR quiet zone is four white modules on every side.
	margin := 4 * 256 / len(bits)
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			if x >= margin && x < 256-margin && y >= margin && y < 256-margin {
				continue
			}
			r, g, b, _ := img.At(x, y).RGBA()
			if r != 0xffff || g != 0xffff || b != 0xffff {
				t.Fatalf("quiet zone is not white at (%d, %d)", x, y)
			}
		}
	}
}

func TestQRRunRejectsUnrenderableContentBeforeWriting(t *testing.T) {
	out := filepath.Join(t.TempDir(), "qr.png")
	q := &QR{}
	if err := q.Run([]string{"hello", "-size", "8", "-out", out}); err == nil {
		t.Fatal("accepted an image too small to fit the QR code")
	}
	if err := q.Run([]string{strings.Repeat("a", 5000), "-out", out}); err == nil {
		t.Fatal("accepted input beyond QR capacity")
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid request created an output file: %v", err)
	}
}

func TestQRImageNamesAreUnique(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Downloads")
	first, err := writeUniquePNG(dir, []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeUniquePNG(dir, []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second || filepath.Dir(first) != dir || filepath.Ext(first) != ".png" {
		t.Fatalf("unexpected image paths: %q, %q", first, second)
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != "first" {
		t.Fatalf("second save replaced first image: %q, %v", data, err)
	}
}

func TestQRPreviewFileRemainsReadable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	path, err := saveQRImage([]byte("preview"), qrOptions{image: true})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("preview path %q is outside temporary directory %q", path, dir)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "preview" {
		t.Fatalf("viewer cannot read generated file: %q, %v", data, err)
	}
}

func TestQRSaveReportsFilesystemErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := saveQRImage([]byte("png"), qrOptions{out: filepath.Join(file, "qr.png")}); err == nil {
		t.Fatal("save swallowed directory creation error")
	}
}

func TestXDGDownloadsDir(t *testing.T) {
	config := t.TempDir()
	userDir := t.TempDir()
	path := filepath.Join(config, "user-dirs.dirs")
	if err := os.WriteFile(path, []byte("# comment\nXDG_DESKTOP_DIR=\"$HOME/Desktop\"\nXDG_DOWNLOAD_DIR=\"$HOME/我的下载\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := xdgDownloadsDir(userDir, config); got != filepath.Join(userDir, "我的下载") {
		t.Fatalf("download directory=%q", got)
	}
	if err := os.WriteFile(path, []byte("XDG_DOWNLOAD_DIR=\"relative/path\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := xdgDownloadsDir(userDir, config); got != "" {
		t.Fatalf("accepted relative download directory: %q", got)
	}
}
