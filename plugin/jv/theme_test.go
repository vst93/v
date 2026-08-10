package plugin_jv

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

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

func TestCursorLineStyles(t *testing.T) {
	tests := []struct {
		colorFGBG string
		want      tcell.Color
	}{
		{"15;0", cursorLineDarkBg},
		{"0;15", cursorLineLightBg},
	}
	for _, tc := range tests {
		bg, gutter := cursorLineStyles(tc.colorFGBG)
		if bg != tc.want {
			t.Errorf("cursorLineStyles(%q) background = %v, want %v", tc.colorFGBG, bg, tc.want)
		}
		fg, gutterBG, attrs := gutter.Decompose()
		if fg != tcell.ColorDefault || gutterBG != tc.want || attrs&tcell.AttrBold == 0 {
			t.Errorf("cursorLineStyles(%q) gutter = fg %v, bg %v, attrs %v", tc.colorFGBG, fg, gutterBG, attrs)
		}
	}
}

func TestDrawUsesCursorLineTheme(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(40, 8)

	v := newViewer(nil, `{"name":"jv"}`, "test", false)
	v.curLineBg, v.curGutter = cursorLineStyles("0;15")
	v.SetRect(0, 0, 40, 8)
	v.Draw(s)

	_, currentBG, _ := cellStyle(s, 10, 0).Decompose()
	if currentBG != cursorLineLightBg {
		t.Errorf("current row background = %v, want %v", currentBG, cursorLineLightBg)
	}
	_, nextBG, _ := cellStyle(s, 10, 1).Decompose()
	if nextBG != tcell.ColorDefault {
		t.Errorf("non-current row background = %v, want default", nextBG)
	}
}

func cellStyle(s tcell.Screen, x, y int) tcell.Style {
	_, _, style, _ := s.GetContent(x, y)
	return style
}
