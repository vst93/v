package theme

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// probeBudget caps how long the OSC 11 background query may take.
const probeBudget = 200 * time.Millisecond

var (
	current      Palette
	probeAllowed bool
)

// Init selects the palette for the running terminal. With probe=true a live
// OSC 11 background query is attempted (interactive TUI entry points should
// pass true; tests and non-TUI paths pass false or nothing at all).
// Detection chain: V_THEME/JV_THEME override > OSC 11 probe > COLORFGBG >
// dark. Init must run before widgets are created; calling it again simply
// re-resolves the palette.
func Init(probe bool) {
	probeAllowed = probe
	current = For(Detect(probe))
}

// Current returns the active palette. If Init has not been called it lazily
// resolves via the environment only (never probing the terminal), so tests
// and CLI-only paths are safe.
func Current() Palette {
	if current.Text == 0 {
		Init(false)
	}
	return current
}

// Light reports whether the active palette targets a light terminal.
func Light() bool { return Current().Light }

// ApplyTView overrides tview's global default style so stock widgets
// (TextView, List, InputField, …) match the detected theme instead of the
// hardcoded black terminal. Call after Init, before building widgets.
func ApplyTView() {
	p := Current()
	tview.Styles.PrimitiveBackgroundColor = p.PrimitiveBG
	tview.Styles.ContrastBackgroundColor = p.ContrastBG
	tview.Styles.MoreContrastBackgroundColor = p.MoreContrastBG
	tview.Styles.PrimaryTextColor = p.Text
	tview.Styles.SecondaryTextColor = p.TextDim
	tview.Styles.TertiaryTextColor = p.Accent
	tview.Styles.InverseTextColor = p.Text
	tview.Styles.ContrastSecondaryTextColor = p.TextDim
	tview.Styles.BorderColor = p.Border
	tview.Styles.TitleColor = p.Text
	tview.Styles.GraphicsColor = p.TextDim
}

// --- detection ---------------------------------------------------------

// Detect reports whether the terminal background is light, running the
// chain fresh: V_THEME/JV_THEME (light|dark) override > OSC 11 probe (if
// probe is true) > COLORFGBG > dark.
func Detect(probe bool) bool {
	for _, key := range []string{"V_THEME", "JV_THEME"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "light":
			return true
		case "dark":
			return false
		}
	}
	if probe {
		if c, ok := queryTerminalBackground(); ok {
			return IsLightColor(c)
		}
	}
	return terminalBackgroundIsLight(os.Getenv("COLORFGBG"))
}

// IsLightLuminance reports whether an RGB triple reads as a light background
// (ITU-R BT.601 relative luminance, threshold ≈ 57%).
func IsLightLuminance(r, g, b int) bool {
	return 299*r+587*g+114*b >= 145000
}

// IsLightColor reports whether a tcell color reads as a light background.
func IsLightColor(c tcell.Color) bool {
	h := c.Hex()
	return IsLightLuminance(int(h>>16)&0xff, int(h>>8)&0xff, int(h)&0xff)
}

// terminalBackgroundIsLight interprets COLORFGBG ("fg;bg", bg last).
func terminalBackgroundIsLight(colorFGBG string) bool {
	parts := strings.Split(colorFGBG, ";")
	if len(parts) < 2 {
		return false
	}
	index, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	if err != nil || index < 0 || index > 255 {
		return false
	}
	r, g, b := ansiColorRGB(index)
	return IsLightLuminance(r, g, b)
}

func ansiColorRGB(index int) (int, int, int) {
	ansi16 := [16][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	if index < len(ansi16) {
		return ansi16[index][0], ansi16[index][1], ansi16[index][2]
	}
	if index >= 232 {
		gray := 8 + (index-232)*10
		return gray, gray, gray
	}
	levels := [6]int{0, 95, 135, 175, 215, 255}
	index -= 16
	return levels[index/36], levels[(index/6)%6], levels[index%6]
}
