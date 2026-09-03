package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDetectEnvOverride(t *testing.T) {
	cases := []struct {
		vTheme    string
		colorFGBG string
		want      bool
	}{
		{"", "0;15", true},      // COLORFGBG says light
		{"", "15;0", false},     // COLORFGBG says dark
		{"", "", false},         // unknown -> dark default
		{"light", "15;0", true}, // V_THEME wins over COLORFGBG
		{"dark", "0;15", false}, // V_THEME wins over COLORFGBG
		{"LIGHT", "15;0", true}, // case-insensitive
		{" bogus ", "0;15", true},
	}
	for _, tc := range cases {
		t.Setenv("V_THEME", tc.vTheme)
		t.Setenv("JV_THEME", "")
		t.Setenv("COLORFGBG", tc.colorFGBG)
		if got := Detect(false); got != tc.want {
			t.Errorf("Detect(V_THEME=%q, COLORFGBG=%q) = %v, want %v",
				tc.vTheme, tc.colorFGBG, got, tc.want)
		}
	}
}

func TestDetectJVThemeAlias(t *testing.T) {
	t.Setenv("V_THEME", "")
	t.Setenv("JV_THEME", "light")
	t.Setenv("COLORFGBG", "15;0")
	if !Detect(false) {
		t.Error("Detect(JV_THEME=light) = false, want true")
	}
	t.Setenv("JV_THEME", "dark")
	t.Setenv("COLORFGBG", "0;15")
	if Detect(false) {
		t.Error("Detect(JV_THEME=dark) = true, want false")
	}
}

func TestTerminalBackgroundIsLight(t *testing.T) {
	cases := []struct {
		colorFGBG string
		want      bool
	}{
		{"15;0", false},
		{"0;15", true},
		{"0;7", true},
		{"15;8", false},
		{"0;9", false},
		{"0;10", true},
		{"0;231", true},
		{"15;232", false},
		{"", false},
		{"0;unknown", false},
	}
	for _, tc := range cases {
		if got := terminalBackgroundIsLight(tc.colorFGBG); got != tc.want {
			t.Errorf("terminalBackgroundIsLight(%q) = %v, want %v", tc.colorFGBG, got, tc.want)
		}
	}
}

func TestIsLightColor(t *testing.T) {
	cases := []struct {
		c    tcell.Color
		want bool
	}{
		{tcell.ColorWhite, true},
		{rgb(238, 241, 242), true}, // light cursor line
		{rgb(184, 213, 234), true}, // light selection
		{tcell.ColorBlack, false},
		{tcell.ColorNavy, false},
		{rgb(32, 38, 40), false}, // dark cursor line
		{tcell.ColorDarkSlateGray, false},
	}
	for _, tc := range cases {
		if got := IsLightColor(tc.c); got != tc.want {
			t.Errorf("IsLightColor(%06x) = %v, want %v", tc.c.Hex()&0xffffff, got, tc.want)
		}
	}
}

func TestPalettesDiffer(t *testing.T) {
	light, dark := For(true), For(false)
	if light.Light == dark.Light {
		t.Error("For(true) and For(false) must disagree on Light")
	}
	if light.SelBg == dark.SelBg {
		t.Errorf("light SelBg = dark SelBg = %v", light.SelBg)
	}
	if light.PanelBg == dark.PanelBg {
		t.Errorf("light PanelBg = dark PanelBg = %v", light.PanelBg)
	}
	if light.CursorLineBg == dark.CursorLineBg {
		t.Errorf("light CursorLineBg = dark CursorLineBg = %v", light.CursorLineBg)
	}
}

func TestHex(t *testing.T) {
	cases := []struct {
		c    tcell.Color
		want string
	}{
		{tcell.ColorBlack, "#000000"},
		{tcell.ColorWhite, "#ffffff"},
		{rgb(46, 204, 113), "#2ecc71"},
	}
	for _, tc := range cases {
		if got := Hex(tc.c); got != tc.want {
			t.Errorf("Hex(%06x) = %q, want %q", tc.c.Hex()&0xffffff, got, tc.want)
		}
	}
}
