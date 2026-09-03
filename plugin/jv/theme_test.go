package plugin_jv

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"v/internal/theme"
)

func TestCursorLineStyles(t *testing.T) {
	tests := []struct {
		light bool
		want  tcell.Color
	}{
		{false, theme.For(false).CursorLineBg},
		{true, theme.For(true).CursorLineBg},
	}
	for _, tc := range tests {
		bg, gutter := cursorLineStyles(tc.light)
		if bg != tc.want {
			t.Errorf("cursorLineStyles(%v) background = %v, want %v", tc.light, bg, tc.want)
		}
		fg, gutterBG, attrs := gutter.Decompose()
		if fg != tcell.ColorDefault || gutterBG != tc.want || attrs&tcell.AttrBold == 0 {
			t.Errorf("cursorLineStyles(%v) gutter = fg %v, bg %v, attrs %v", tc.light, fg, gutterBG, attrs)
		}
	}
}

func TestApplyTheme(t *testing.T) {
	dark, light := theme.For(false), theme.For(true)

	// Light theme: accents must come from the light palette.
	applyTheme(true)
	if _, bg, _ := stSelection.Decompose(); bg != light.SelBg {
		t.Errorf("light stSelection background = %v, want %v", bg, light.SelBg)
	}
	if _, bg, _ := stPanel.Decompose(); bg != light.PanelBg {
		t.Errorf("light stPanel background = %v, want %v", bg, light.PanelBg)
	}
	if _, bg, _ := stPanelDim.Decompose(); bg != light.PanelBg {
		t.Errorf("light stPanelDim background = %v, want %v", bg, light.PanelBg)
	}
	if _, bg, _ := stBox.Decompose(); bg != light.BoxBg {
		t.Errorf("light stBox background = %v, want %v", bg, light.BoxBg)
	}

	// Dark theme restores the original palette.
	applyTheme(false)
	if _, bg, _ := stSelection.Decompose(); bg != dark.SelBg {
		t.Errorf("dark stSelection background = %v, want %v", bg, dark.SelBg)
	}
	if _, bg, _ := stPanel.Decompose(); bg != dark.PanelBg {
		t.Errorf("dark stPanel background = %v, want %v", bg, dark.PanelBg)
	}
	if _, bg, _ := stPanelDim.Decompose(); bg != dark.PanelBg {
		t.Errorf("dark stPanelDim background = %v, want %v", bg, dark.PanelBg)
	}
	if _, bg, _ := stBox.Decompose(); bg != dark.BoxBg {
		t.Errorf("dark stBox background = %v, want %v", bg, dark.BoxBg)
	}

	applyTheme(true) // leave the light palette in place for draw tests below
}

func TestDrawUsesLightTheme(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(40, 8)

	applyTheme(true)
	v := newViewer(nil, `{"name":"jv"}`, "test", false)
	v.curLineBg, v.curGutter = cursorLineStyles(true)
	v.SetRect(0, 0, 40, 8)
	v.Draw(s)

	_, currentBG, _ := cellStyle(s, 10, 0).Decompose()
	if currentBG != theme.For(true).CursorLineBg {
		t.Errorf("current row background = %v, want the light cursor-line color", currentBG)
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
