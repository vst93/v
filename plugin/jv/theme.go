package plugin_jv

import (
	"github.com/gdamore/tcell/v2"

	"v/internal/theme"
)

// applyTheme maps the shared palette onto the viewer's style variables.
// Syntax styles (stKey, stString, …) use plain ANSI palette colors and are
// theme-independent; only the accent surfaces swap between light and dark.
func applyTheme(light bool) {
	p := theme.For(light)

	stSelection = tcell.StyleDefault.Foreground(p.SelFg).Background(p.SelBg)

	stPanel = tcell.StyleDefault.Foreground(p.PanelFg).Background(p.PanelBg)
	stPanelDim = tcell.StyleDefault.Foreground(p.PanelDimFg).Background(p.PanelBg)

	stBox = tcell.StyleDefault.Foreground(p.PanelFg).Background(p.BoxBg)
	stBoxKey = tcell.StyleDefault.Foreground(p.BoxKeyFg).Background(p.BoxBg).Bold(true)
	stBoxTitle = tcell.StyleDefault.Foreground(p.BoxTitleFg).Background(p.BoxBg).Bold(true)
}

// cursorLineStyles returns a deliberately low-contrast cursor-row palette
// (the foreground stays at the terminal default).
func cursorLineStyles(light bool) (tcell.Color, tcell.Style) {
	bg := theme.For(light).CursorLineBg
	return bg, tcell.StyleDefault.Background(bg).Bold(true)
}
