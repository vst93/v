package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestParseRGBReply(t *testing.T) {
	cases := []struct {
		val  string
		want tcell.Color
		ok   bool
	}{
		{"rgb:ffff/ffff/ffff", tcell.NewRGBColor(255, 255, 255), true}, // 4 digits
		{"rgb:ff/ff/ff", tcell.NewRGBColor(255, 255, 255), true},       // 2 digits
		{"rgb:f/f/f", tcell.NewRGBColor(255, 255, 255), true},          // 1 digit, max
		{"rgb:1/1/1", tcell.NewRGBColor(17, 17, 17), true},             // 1 digit scaled
		{"rgb:fff/000/842", tcell.NewRGBColor(255, 0, 132), true},      // 3 digits
		{"rgb:1c1c/1c1c/1c1c", tcell.NewRGBColor(28, 28, 28), true},    // VS Code dark
		{"rgb:ffff/8080/0000", tcell.NewRGBColor(255, 128, 0), true},
		{"rgb:ff/ff", 0, false},
		{"rgb:zz/ff/ff", 0, false},
		{"rgb:", 0, false},
		{"", 0, false},
		{"rgba:ff/ff/ff/ff", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseRGBReply(tc.val)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseRGBReply(%q) = %v, %v; want %v, %v", tc.val, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseOSC11(t *testing.T) {
	white := tcell.NewRGBColor(255, 255, 255)
	black := tcell.NewRGBColor(0, 0, 0)
	cases := []struct {
		buf  string
		want tcell.Color
		ok   bool
	}{
		{"\x1b]11;rgb:ffff/ffff/ffff\x1b\\", white, true},                      // ST terminator
		{"\x1b]11;rgb:ffff/ffff/ffff\x07", white, true},                        // BEL terminator
		{"\x1b]11;rgb:eeee/eeee/eeee", tcell.NewRGBColor(238, 238, 238), true}, // unterminated
		{"noise\x1b]11;rgb:00/00/00\x07", black, true},                         // leading garbage
		{"\x1b]11;15\x07", white, true},                                        // palette index reply
		{"\x1b]11;0\x07", black, true},                                         // palette index 0
		{"\x1b]10;rgb:ff/ff/ff\x07", 0, false},                                 // fg query, not bg
		{"", 0, false},
		{"nothing here", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseOSC11([]byte(tc.buf))
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseOSC11(%q) = %v, %v; want %v, %v", tc.buf, got, ok, tc.want, tc.ok)
		}
	}
}
