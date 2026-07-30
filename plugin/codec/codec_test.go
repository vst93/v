package plugin_codec

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestTransformRoundTrip checks that every codec's encode output decodes back
// to the original input.
func TestTransformRoundTrip(t *testing.T) {
	inputs := []string{
		"Hello World",
		"你好，世界",
		`<a href="x">&amp;</a>`,
		"hello world&foo=bar+baz",
		"emoji 🎉 astral",
	}

	for _, c := range codecs {
		for _, in := range inputs {
			encoded, err := doTransform(c.enc, in)
			if err != nil {
				t.Errorf("%s encode(%q) failed: %v", c.label, in, err)
				continue
			}
			decoded, err := doTransform(c.dec, encoded)
			if err != nil {
				t.Errorf("%s decode(%q) failed: %v", c.label, encoded, err)
				continue
			}
			if decoded != in {
				t.Errorf("%s round-trip mismatch: got %q, want %q", c.label, decoded, in)
			}
		}
	}
}

// TestTransformKnownValues pins the wire format of each encoder so a refactor
// cannot silently change it.
func TestTransformKnownValues(t *testing.T) {
	cases := []struct{ mode, in, want string }{
		{"base64enc", "Hello World", "SGVsbG8gV29ybGQ="},
		{"base64dec", "SGVsbG8gV29ybGQ=", "Hello World"},
		{"base32enc", "Hi", "JBUQ===="},
		{"urlenc", "hello world", "hello+world"},
		{"urldec", "hello+world", "hello world"},
		{"hexenc", "Hello", "48656c6c6f"},
		{"hexdec", "48656c6c6f", "Hello"},
		{"htmlesc", "<b>", "&lt;b&gt;"},
		{"htmlunesc", "&lt;b&gt;", "<b>"},
		{"unienc", "你好", `\u4f60\u597d`},
		{"unidec", `你好`, "你好"},
	}

	for _, c := range cases {
		got, err := doTransform(c.mode, c.in)
		if err != nil {
			t.Errorf("%s(%q) returned error: %v", c.mode, c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.mode, c.in, got, c.want)
		}
	}
}

// TestTransformDecodeErrors verifies malformed input is reported rather than
// silently producing garbage.
func TestTransformDecodeErrors(t *testing.T) {
	for _, mode := range []string{"base64dec", "base32dec", "hexdec"} {
		if _, err := doTransform(mode, "!!!not valid!!!"); err == nil {
			t.Errorf("%s accepted invalid input without an error", mode)
		}
	}
	if _, err := doTransform("nope", "x"); err == nil {
		t.Error("unknown mode did not return an error")
	}
}

// drawTUI renders the TUI at the given size and returns the screen rows as
// strings.
func drawTUI(t *testing.T, w, h int, input string) []string {
	t.Helper()

	ui := &codecTUI{}
	ui.buildWidgets()
	ui.buildLayout(w < narrowCols)
	ui.focusRing = []tview.Primitive{}

	if input != "" {
		ui.input.SetText(input, true)
	}
	ui.transform()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	screen.SetSize(w, h)

	ui.root.SetRect(0, 0, w, h)
	ui.root.Draw(screen)

	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			r, _, _, _ := screen.GetContent(x, y)
			sb.WriteRune(r)
		}
		rows[y] = sb.String()
	}
	return rows
}

// contains reports whether any rendered row contains the given substring.
func contains(rows []string, substr string) bool {
	for _, r := range rows {
		if strings.Contains(r, substr) {
			return true
		}
	}
	return false
}

// TestTUILayout renders the TUI headlessly and checks that the codec list,
// mode selector and output area are all visible. The default codec is Base64
// in decode mode, so feeding valid base64 should produce the decoded text on
// screen.
func TestTUILayout(t *testing.T) {
	rows := drawTUI(t, 80, 24, "SGVsbG8gV29ybGQ=")

	for _, want := range []string{"Codec", "Mode", "Input", "Output"} {
		if !contains(rows, want) {
			t.Errorf("TUI missing %q in rendered output", want)
		}
	}
	// Base64-decoding "SGVsbG8gV29ybGQ=" yields "Hello World".
	if !contains(rows, "Hello World") {
		t.Errorf("TUI output does not contain decoded text; got:\n%s", strings.Join(rows, "\n"))
	}
}

// TestTUILayoutNarrow checks the reflowed layout used under narrow terminals:
// the selectors move to a row above the editor instead of a left column.
func TestTUILayoutNarrow(t *testing.T) {
	rows := drawTUI(t, 50, 30, "SGVsbG8gV29ybGQ=")
	if !contains(rows, "Codec") {
		t.Errorf("narrow TUI missing %q", "Codec")
	}
	if !contains(rows, "Hello World") {
		t.Errorf("narrow TUI output does not contain decoded text; got:\n%s", strings.Join(rows, "\n"))
	}
}
