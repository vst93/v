package plugin_jv

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEditCursorVisible(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)

	v := newViewer(nil, `{"name":"hello"}`, "test", false)
	v.SetRect(0, 0, 80, 24)

	// Normal mode: cursor hidden.
	v.Draw(s)
	_, _, vis := s.GetCursor()
	if vis {
		t.Error("cursor should be hidden in normal mode")
	}

	// Edit mode at line 1: cursor visible at first non-whitespace.
	v.cursor = 1
	v.enterEdit(-1)
	v.Draw(s)
	cx, cy, vis := s.GetCursor()
	if !vis {
		t.Fatal("cursor should be visible in edit mode")
	}
	if cy != 1 {
		t.Errorf("cursor y = %d, want 1", cy)
	}
	// Line 1: `  "name": "hello"` - first non-ws at col 2.
	if cx != v.gutterW+2 {
		t.Errorf("cursor x = %d, want %d (gutterW+2)", cx, v.gutterW+2)
	}

	// Type 'X' - cursor moves right by 1.
	v.editKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	v.Draw(s)
	cx, _, vis = s.GetCursor()
	if !vis {
		t.Error("cursor should be visible after insert")
	}
	if cx != v.gutterW+3 {
		t.Errorf("cursor x after insert = %d, want %d", cx, v.gutterW+3)
	}
}

func TestEditMoveHorizBoundary(t *testing.T) {
	v := newViewer(nil, `{"a":1,"bb":22}`, "test", false)
	v.editorH = 10
	// Pretty-printed: {, "a": 1,, "bb": 22, }
	v.cursor = 1
	v.enterEdit(-1) // first non-ws: col 2
	if v.editCol != 2 {
		t.Fatalf("enterEdit(-1) editCol = %d, want 2", v.editCol)
	}

	// Move left to col 0, then left again -> end of previous line (line 0: "{").
	for i := 0; i < 3; i++ {
		v.editKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if v.editLine != 0 {
		t.Errorf("after 3 Left: editLine = %d, want 0", v.editLine)
	}
	if v.editCol != 1 {
		t.Errorf("after 3 Left: editCol = %d, want 1 (end of '{')", v.editCol)
	}

	// Move right -> should go to beginning of line 1 (col 0).
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.editLine != 1 {
		t.Errorf("after Right at EOL: editLine = %d, want 1", v.editLine)
	}
	if v.editCol != 0 {
		t.Errorf("after Right at EOL: editCol = %d, want 0", v.editCol)
	}
}

func TestEditMoveHorizEOL(t *testing.T) {
	v := newViewer(nil, `{"a":1}`, "test", false)
	v.editorH = 10
	// Line 1: `  "a": 1` (9 chars). Enter edit at first non-ws (col 2).
	v.cursor = 1
	v.enterEdit(-1)

	// Move right to end of line (col 8 = len of line `  "a": 1`).
	for i := 0; i < 6; i++ {
		v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	if v.editCol != 8 {
		t.Fatalf("editCol = %d, want 8 (EOL)", v.editCol)
	}
	if v.editLine != 1 {
		t.Errorf("should still be on line 1 at EOL, got line %d", v.editLine)
	}

	// Press right again -> should move to next line (line 2: "}").
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.editLine != 2 {
		t.Errorf("after Right at EOL: editLine = %d, want 2", v.editLine)
	}
	if v.editCol != 0 {
		t.Errorf("after Right at EOL: editCol = %d, want 0", v.editCol)
	}

	// Backspace at col 0 of line 2 -> should join with line 1.
	v.editKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if len(v.textLines) != 2 { // was 3, joined -> 2
		t.Errorf("after backspace at line start: lines = %d, want 2", len(v.textLines))
	}
}

func TestEditNewlineIndent(t *testing.T) {
	v := newViewer(nil, `{"a":1}`, "test", false)
	v.editorH = 10
	// Line 1: `  "a": 1` - move cursor to middle, press Enter.
	v.cursor = 1
	v.enterEdit(-1) // col 2
	// Move right to col 7 (after "a": 1)
	for i := 0; i < 5; i++ {
		v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	v.editKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// New line should have indentation carried over.
	if len(v.textLines) != 4 { // was 3, +1 from newline
		t.Fatalf("lines = %d, want 4", len(v.textLines))
	}
	newLine := v.textLines[2]
	if newLine[:2] != "  " {
		t.Errorf("new line should start with indentation, got %q", newLine)
	}
	if v.editCol != 2 {
		t.Errorf("editCol after newline = %d, want 2 (after indent)", v.editCol)
	}
}

func TestEditCursorWideChar(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)

	v := newViewer(nil, `{"x":"ab"}`, "test", false)
	v.SetRect(0, 0, 80, 24)

	v.cursor = 1
	v.enterEdit(-1) // col 2
	for i := 0; i < 8; i++ {
		v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	v.Draw(s)
	cx, _, vis := s.GetCursor()
	if !vis {
		t.Fatal("cursor should be visible")
	}

	// Insert a wide char (width 2) - cursor should advance 2 cells.
	v.editKey(tcell.NewEventKey(tcell.KeyRune, 0x4e2d, tcell.ModNone))
	v.Draw(s)
	cx2, _, vis2 := s.GetCursor()
	if !vis2 {
		t.Fatal("cursor should be visible after wide char")
	}
	if cx2 != cx+2 {
		t.Errorf("cursor advanced %d cells, want 2 (%d -> %d)", cx2-cx, cx, cx2)
	}
}
