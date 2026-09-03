// Package theme provides terminal light/dark background detection and a
// shared semantic palette for v's interactive (TUI) plugins.
//
// It is the standard for plugin TUIs:
//
//   - At TUI startup, call theme.Init(true) (enables the live OSC 11
//     background probe) followed by theme.ApplyTView() when the plugin is
//     built on tview, so stock widgets stop assuming a black terminal.
//   - Never hardcode theme-sensitive colors; pull slots from
//     theme.Current() (or theme.Hex() for tview "[#...]"" tags) instead.
//     Colors that carry meaning in both themes (red = error, yellow = match
//     highlight) may stay literal.
//   - Manual-draw (tcell.SetContent) viewers map palette slots onto their
//     own style variables, like plugin/jv does.
//
// Detection chain: V_THEME (light|dark, JV_THEME accepted as an alias) >
// live OSC 11 query > COLORFGBG > dark.
package theme

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Palette holds the semantic color slots shared by v's TUI plugins.
type Palette struct {
	Light bool

	// Text on the default (terminal) background.
	Text    tcell.Color
	TextDim tcell.Color // hints, counts, secondary text

	// Accent: selection highlight, focused borders, primary chips.
	Accent   tcell.Color
	AccentFg tcell.Color // text drawn on Accent

	// Status colors.
	Success tcell.Color
	Warn    tcell.Color
	Error   tcell.Color
	Info    tcell.Color

	Border tcell.Color

	// Manual-draw surfaces (jv).
	CursorLineBg tcell.Color
	SelBg        tcell.Color
	SelFg        tcell.Color
	PanelBg      tcell.Color
	PanelFg      tcell.Color
	PanelDimFg   tcell.Color
	BoxBg        tcell.Color
	BoxKeyFg     tcell.Color
	BoxTitleFg   tcell.Color
	MatchBg      tcell.Color
	MatchCurBg   tcell.Color

	// Input fields.
	FieldBg tcell.Color
	FieldFg tcell.Color

	// tview global style overrides (see ApplyTView).
	PrimitiveBG    tcell.Color
	ContrastBG     tcell.Color
	MoreContrastBG tcell.Color
}

// For returns the palette for the given terminal background family. The
// dark palette mirrors the historical defaults of the existing plugins.
func For(light bool) Palette {
	if light {
		return Palette{
			Light:   true,
			Text:    tcell.ColorBlack,
			TextDim: rgb(100, 106, 112),

			Accent:   rgb(26, 127, 74),
			AccentFg: tcell.ColorBlack,

			Success: rgb(26, 127, 74),
			Warn:    rgb(154, 103, 0),
			Error:   rgb(207, 34, 46),
			Info:    rgb(1, 116, 116),

			Border: rgb(150, 156, 162),

			CursorLineBg: rgb(238, 241, 242),
			SelBg:        rgb(184, 213, 234),
			SelFg:        tcell.ColorBlack,
			PanelBg:      rgb(232, 235, 236),
			PanelFg:      tcell.ColorBlack,
			PanelDimFg:   rgb(100, 106, 112),
			BoxBg:        rgb(232, 235, 236),
			BoxKeyFg:     tcell.ColorTeal,
			BoxTitleFg:   tcell.ColorBlue,
			MatchBg:      tcell.ColorYellow,
			MatchCurBg:   tcell.ColorOrange,

			FieldBg: tcell.ColorWhite,
			FieldFg: tcell.ColorBlack,

			PrimitiveBG:    tcell.ColorDefault,
			ContrastBG:     tcell.ColorWhite,
			MoreContrastBG: tcell.ColorSilver,
		}
	}
	return Palette{
		Text:    tcell.ColorWhite,
		TextDim: tcell.ColorGray,

		Accent:   rgb(46, 204, 113),
		AccentFg: tcell.ColorBlack,

		Success: rgb(80, 250, 123),
		Warn:    rgb(255, 170, 0),
		Error:   rgb(255, 85, 85),
		Info:    rgb(139, 233, 253),

		Border: tcell.ColorGray,

		CursorLineBg: rgb(32, 38, 40),
		SelBg:        tcell.ColorNavy,
		SelFg:        tcell.ColorWhite,
		PanelBg:      tcell.ColorDarkSlateGray,
		PanelFg:      tcell.ColorWhite,
		PanelDimFg:   tcell.ColorGray,
		BoxBg:        tcell.ColorDarkSlateGray,
		BoxKeyFg:     tcell.ColorTeal,
		BoxTitleFg:   tcell.ColorYellow,
		MatchBg:      tcell.ColorYellow,
		MatchCurBg:   tcell.ColorOrange,

		FieldBg: tcell.ColorBlack,
		FieldFg: tcell.ColorWhite,

		PrimitiveBG:    tcell.ColorBlack,
		ContrastBG:     tcell.ColorBlue,
		MoreContrastBG: tcell.ColorGreen,
	}
}

// Hex renders a color as a "#rrggbb" string for tview dynamic color tags.
// It must not be called on tcell.ColorDefault.
func Hex(c tcell.Color) string {
	return fmt.Sprintf("#%06x", c.Hex()&0xffffff)
}

func rgb(r, g, b int) tcell.Color {
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}
