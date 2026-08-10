package plugin_jv

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

var (
	cursorLineDarkBg  = tcell.NewRGBColor(32, 38, 40)
	cursorLineLightBg = tcell.NewRGBColor(238, 241, 242)
)

// cursorLineStyles returns a deliberately low-contrast cursor-row palette.
// COLORFGBG conventionally ends with the terminal background's ANSI index.
func cursorLineStyles(colorFGBG string) (tcell.Color, tcell.Style) {
	bg := cursorLineDarkBg
	if terminalBackgroundIsLight(colorFGBG) {
		bg = cursorLineLightBg
	}
	return bg, tcell.StyleDefault.Background(bg).Bold(true)
}

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
	return 299*r+587*g+114*b >= 145000
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
