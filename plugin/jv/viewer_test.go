package plugin_jv

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const sampleJSON = `{
	"name": "jv",
	"tags": ["json", "viewer"],
	"meta": {"version": 2, "stable": true, "note": null},
	"empty": {},
	"list": []
}`

func newTestViewer(t *testing.T, input string) *Viewer {
	t.Helper()
	v := newViewer(nil, input, "test", false)
	v.editorH = 10
	return v
}

func TestLexDocument(t *testing.T) {
	lines, containers := lexDocument(splitLines(FormatJSON(mustDecode(t, sampleJSON), 2, false)))

	// Expect: root open, name, tags open, 2 elems, tags close,
	// meta open, 3 elems, meta close, empty, list, root close = 14 lines.
	if got, want := len(lines), 14; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if lines[0].kind != kindOpen {
		t.Errorf("first line kind = %v, want kindOpen", lines[0].kind)
	}
	if lines[len(lines)-1].kind != kindClose {
		t.Errorf("last line kind = %v, want kindClose", lines[len(lines)-1].kind)
	}
	if plain := strings.TrimSpace(lines[1].plain); plain != `"name": "jv",` {
		t.Errorf("line 1 = %q", plain)
	}
	// 5 containers total (root, tags, meta + 2 single-line {} and []);
	// 3 of them span multiple lines.
	if got := len(containers); got != 5 {
		t.Fatalf("containers = %d, want 5", got)
	}
	multi := 0
	for id, c := range containers {
		if c.closeLine > c.openLine {
			multi++
			if lines[c.openLine].kind != kindOpen || lines[c.closeLine].kind != kindClose {
				t.Errorf("container %d range [%d,%d] kind mismatch", id, c.openLine, c.closeLine)
			}
		}
	}
	if multi != 3 {
		t.Errorf("multi-line containers = %d, want 3", multi)
	}
	// Paths.
	if lines[1].path != "name" {
		t.Errorf("line 1 path = %q, want %q", lines[1].path, "name")
	}
	if lines[4].path != "tags[1]" {
		t.Errorf("line 4 path = %q, want %q", lines[4].path, "tags[1]")
	}
}

func TestLexInvalidJSON(t *testing.T) {
	// Unbalanced/invalid text still lexes without panic.
	lines, _ := lexDocument(splitLines(`{ "a": 1, "b": `))
	if len(lines) != 1 {
		t.Fatalf("invalid input line count = %d, want 1", len(lines))
	}
}

func TestLexEscapedStrings(t *testing.T) {
	tree, _ := DecodeJSON(`{"k":"中文"}`)
	esc := FormatJSONEscape(tree, 2, true)
	if want := "\\" + "u4e2d"; !strings.Contains(esc, want) {
		t.Errorf("escaped output should contain %s: %q", want, esc)
	}
	plain := FormatJSON(tree, 2, false)
	if !strings.Contains(plain, "中文") {
		t.Errorf("plain output should keep UTF-8: %q", plain)
	}
}

func mustDecode(t *testing.T, input string) interface{} {
	t.Helper()
	data, err := DecodeJSON(input)
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	return data
}

func TestFoldVisibility(t *testing.T) {
	v := newTestViewer(t, sampleJSON)
	total := len(v.visible)
	if total != 14 {
		t.Fatalf("visible = %d, want 14", total)
	}

	// Fold the "tags" array (full line 2).
	v.toggleFoldLine(2)
	if len(v.visible) != total-3 { // hides 2 elements + close line
		t.Fatalf("after fold visible = %d, want %d", len(v.visible), total-3)
	}
	if v.visible[2] != 2 {
		t.Errorf("visible[2] = %d, want 2 (folded open line)", v.visible[2])
	}

	v.toggleFoldLine(2)
	if len(v.visible) != total {
		t.Fatalf("after unfold visible = %d, want %d", len(v.visible), total)
	}
}

func TestCollapseExpandAll(t *testing.T) {
	v := newTestViewer(t, sampleJSON)
	v.collapseAll()
	if len(v.visible) != 1 {
		t.Fatalf("collapseAll visible = %d, want 1", len(v.visible))
	}
	v.expandAll()
	if len(v.visible) != 14 {
		t.Fatalf("expandAll visible = %d, want 14", len(v.visible))
	}
}

func TestFoldPreservesSearchUnfold(t *testing.T) {
	v := newTestViewer(t, sampleJSON)
	v.collapseAll()
	v.searchInput.set("stable")
	v.runSearch()
	if len(v.matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(v.matches))
	}
	v.jumpToMatch(0)
	if len(v.visible) == 1 {
		t.Fatalf("jumpToMatch did not unfold ancestors")
	}
	pos := v.visiblePosOf(v.matches[0].line)
	if v.visible[pos] != v.matches[0].line {
		t.Errorf("match line not visible after jump")
	}
}

func TestSearchModes(t *testing.T) {
	v := newTestViewer(t, sampleJSON)

	v.searchInput.set("TAGS")
	v.runSearch()
	if len(v.matches) != 1 {
		t.Errorf("ci search matches = %d, want 1", len(v.matches))
	}

	v.optCase = true
	v.runSearch()
	if len(v.matches) != 0 {
		t.Errorf("case search matches = %d, want 0", len(v.matches))
	}

	v.optCase = false
	v.optWord = true
	v.searchInput.set("tag")
	v.runSearch()
	if len(v.matches) != 0 {
		t.Errorf("word search should not match inside %q, got %d", `"tags"`, len(v.matches))
	}

	v.optWord = false
	v.optRegex = true
	v.searchInput.set(`"s\w+le"`)
	v.runSearch()
	if len(v.matches) != 1 {
		t.Errorf("regex matches = %d, want 1", len(v.matches))
	}

	v.searchInput.set(`[bad`)
	v.runSearch()
	if v.matchErr == "" {
		t.Errorf("expected regex error for unbalanced pattern")
	}
}

func TestEvalFilter(t *testing.T) {
	root := mustDecode(t, sampleJSON)

	cases := []struct {
		expr string
		want string
	}{
		{"", `{"name":"jv","tags":["json","viewer"],"meta":{"version":2,"stable":true,"note":null},"empty":{},"list":[]}`},
		{".name", `"jv"`},
		{".tags[1]", `"viewer"`},
		{".tags[-1]", `"viewer"`},
		{".meta.stable", `true`},
		{".tags.length", `2`},
		{".meta.length", `3`},
		{`.meta["version"]`, `2`},
		{".tags.map(.length)", `[4,6]`},
		{"this.tags[0]", `"json"`},
	}
	for _, c := range cases {
		res, err := evalFilter(root, c.expr)
		if err != nil {
			t.Errorf("evalFilter(%q): %v", c.expr, err)
			continue
		}
		if got := CompactJSON(res, false); got != c.want {
			t.Errorf("evalFilter(%q) = %s, want %s", c.expr, got, c.want)
		}
	}

	errs := []string{".missing", ".name.x", ".tags[9]", ".name.length.ok", ".tags.map(", "foo"}
	for _, e := range errs {
		if _, err := evalFilter(root, e); err == nil {
			t.Errorf("evalFilter(%q): expected error", e)
		}
	}
}

func TestApplyFilterRestores(t *testing.T) {
	v := newTestViewer(t, sampleJSON)
	v.filterInput.set(".meta")
	v.applyFilter()
	if !v.filtered {
		t.Fatalf("expected filtered state")
	}
	if len(v.lines) != 5 {
		t.Fatalf("filtered lines = %d, want 5", len(v.lines))
	}

	v.filterInput.clear()
	v.applyFilter()
	if v.filtered {
		t.Fatalf("clearing the filter should restore the document")
	}
	if len(v.lines) != 14 {
		t.Fatalf("restored lines = %d, want 14", len(v.lines))
	}
}

func TestWheelScrollKeepsCursor(t *testing.T) {
	v := newTestViewer(t, sampleJSON)
	v.editorH = 4
	v.cursor = 2
	for i := 0; i < 10; i++ {
		v.wheelScroll(1)
	}
	if v.cursor != 2 {
		t.Errorf("wheel scroll moved cursor to %d", v.cursor)
	}
	if v.scroll != len(v.visible)-v.editorH {
		t.Errorf("scroll = %d, want max %d", v.scroll, len(v.visible)-v.editorH)
	}
}

func TestInvalidJSONOpensAsText(t *testing.T) {
	v := newViewer(nil, "not json at all\nsecond line", "bad", false)
	if v.tree != nil {
		t.Errorf("tree should be nil for invalid JSON")
	}
	if v.errLine == 0 {
		t.Errorf("errLine should be set for invalid JSON")
	}
	if len(v.textLines) != 2 {
		t.Fatalf("textLines = %d, want 2", len(v.textLines))
	}
	if v.textLines[0] != "not json at all" {
		t.Errorf("textLines[0] = %q", v.textLines[0])
	}
}

func TestEditMode(t *testing.T) {
	// Startup pretty-prints {"a":1} into 3 lines: {, "a": 1, }.
	v := newTestViewer(t, `{"a":1}`)
	if len(v.textLines) != 3 {
		t.Fatalf("startup lines = %d, want 3", len(v.textLines))
	}

	// Edit the value line (line 1: `  "a": 1`).
	v.cursor = 1
	v.enterEdit(0)
	if !v.editing {
		t.Fatal("expected editing mode")
	}
	if v.editLine != 1 {
		t.Fatalf("editLine = %d, want 1", v.editLine)
	}

	// Insert 'X' at col 0 -> breaks JSON.
	v.editKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	if v.tree != nil {
		t.Errorf("tree should be nil after breaking JSON")
	}

	// Undo restores valid JSON.
	v.editKey(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone))
	if v.tree == nil {
		t.Errorf("tree should be valid after undo")
	}

	// Newline splits the value line.
	before := len(v.textLines)
	v.editKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if len(v.textLines) != before+1 {
		t.Fatalf("after newline lines = %d, want %d", len(v.textLines), before+1)
	}

	v.exitEdit()
	if v.editing {
		t.Error("exitEdit should clear editing flag")
	}
}

func TestReformat(t *testing.T) {
	v := newTestViewer(t, `{"a":1,"b":2}`)
	// Compact single-line input gets pretty-printed on startup already.
	v.reformat()
	if len(v.textLines) != 4 {
		t.Fatalf("after reformat lines = %d, want 4", len(v.textLines))
	}
}

func TestToggleButtonStyle(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(100, 24)

	v := newViewer(nil, sampleJSON, "test", false)
	v.SetRect(0, 0, 100, 24)

	findU := func() tcell.Style {
		v.Draw(s)
		for x := 99; x >= 0; x-- {
			c, _, st, _ := s.GetContent(x, 23)
			if c == '\\' {
				return st
			}
		}
		t.Fatal("\\u button not found in toolbar")
		return tcell.StyleDefault
	}

	before := findU()
	if _, bg, _ := before.Decompose(); bg != tcell.ColorDefault {
		t.Errorf("\\u button bg = %v while off, want default", bg)
	}

	v.toggleEscape()
	after := findU()
	if _, bg, _ := after.Decompose(); bg != tcell.ColorTeal {
		t.Errorf("\\u button bg = %v while on, want teal", bg)
	}
}

// TestDrawSmoke renders the viewer onto a simulation screen to catch
// drawing panics and verify basic content.
func TestDrawSmoke(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)

	v := newViewer(nil, sampleJSON, "test", false)
	v.SetRect(0, 0, 80, 24)
	v.Draw(s)
	s.Show()

	ch, _, _, _ := s.GetContent(3, 0)
	if ch != '1' {
		t.Errorf("gutter cell = %q, want '1'", ch)
	}
	cells := make([]rune, 0, 10)
	for x := 1; x < 5; x++ {
		c, _, _, _ := s.GetContent(x, 23)
		cells = append(cells, c)
	}
	if string(cells) != "this" {
		t.Errorf("bar label = %q, want %q", string(cells), "this")
	}

	// Fold root, then the placeholder pill should render on line 1.
	v.toggleFoldLine(0)
	v.Draw(s)
	found := false
	for x := 0; x < 80; x++ {
		c, _, _, _ := s.GetContent(x, 0)
		if c == '…' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("fold placeholder not rendered after folding root")
	}
}

func TestSelectionShiftArrows(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(2) // col 2 = opening quote of key

	// Shift+Right selects one char.
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if !v.hasSelection() {
		t.Fatal("expected selection after Shift+Right")
	}
	if got := v.selectedText(); got != `"` {
		t.Errorf("selectedText = %q, want %q", got, `"`)
	}

	// Shift+Right again extends.
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if got := v.selectedText(); got != `"a` {
		t.Errorf("selectedText = %q, want %q", got, `"a`)
	}

	// Plain Right clears selection.
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if v.hasSelection() {
		t.Error("selection should be cleared after plain Right")
	}
}

func TestSelectAll(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	want := strings.Join(v.textLines, "\n")
	v.enterEdit(0)
	v.editKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone))
	if !v.hasSelection() {
		t.Fatal("expected selection after Ctrl-A")
	}
	if got := v.selectedText(); got != want {
		t.Errorf("selectedText = %q, want %q", got, want)
	}
}

func TestDeleteSelection(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(2) // `"`
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	// Selected `"a` from cols 2-4.
	v.editKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if got, want := v.textLines[1], `  ": 1`; got != want {
		t.Errorf("line 1 = %q, want %q", got, want)
	}
	if v.hasSelection() {
		t.Error("selection should be cleared after delete")
	}
}

func TestTypeReplacesSelection(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(3) // 'a'
	v.selectWord()
	if got := v.selectedText(); got != "a" {
		t.Fatalf("selectedText = %q, want %q", got, "a")
	}
	v.editKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone))
	if !strings.Contains(v.textLines[1], `"b"`) {
		t.Errorf("line 1 = %q, expected 'b' replacing 'a'", v.textLines[1])
	}
	if v.hasSelection() {
		t.Error("selection should be cleared after typing")
	}
}

func TestSelectWord(t *testing.T) {
	v := newTestViewer(t, `{"name":"jv"}`)
	v.cursor = 1
	v.enterEdit(3) // 'n' in name
	v.selectWord()
	if !v.hasSelection() {
		t.Fatal("expected selection after selectWord")
	}
	if got := v.selectedText(); got != "name" {
		t.Errorf("selectedText = %q, want %q", got, "name")
	}
}

func TestWordMovement(t *testing.T) {
	v := newTestViewer(t, `{"name":"jv"}`)
	v.cursor = 1
	v.enterEdit(3) // 'n' in name

	// Ctrl+Right moves past "name" to the closing quote (col 7).
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl))
	if v.editCol != 7 {
		t.Errorf("after Ctrl+Right editCol = %d, want 7", v.editCol)
	}

	// Ctrl+Left moves back to 'n' (col 3).
	v.editKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl))
	if v.editCol != 3 {
		t.Errorf("after Ctrl+Left editCol = %d, want 3", v.editCol)
	}
}

func TestShiftHomeEnd(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(5) // ':'

	v.editKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModShift))
	if !v.hasSelection() {
		t.Fatal("expected selection after Shift+Home")
	}
	if v.editCol != 0 {
		t.Errorf("editCol = %d, want 0", v.editCol)
	}

	v.editKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModShift))
	end := len([]rune(v.textLines[v.editLine]))
	if v.editCol != end {
		t.Errorf("editCol = %d, want %d", v.editCol, end)
	}
}

func TestSelectionMultiLine(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	// textLines: ["{", "  \"a\": 1", "}"]
	v.enterEdit(0)
	// Select from line 0 col 0 to line 2 col 1.
	v.selActive = true
	v.selAnchorLine = 0
	v.selAnchorCol = 0
	v.editLine = 2
	v.editCol = 1
	want := "{\n  \"a\": 1\n}"
	if got := v.selectedText(); got != want {
		t.Errorf("selectedText = %q, want %q", got, want)
	}
}

func TestEditInsertText(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(3) // 'a'
	v.editInsertText("XX")
	if !strings.Contains(v.textLines[1], `"XXa"`) {
		t.Errorf("line 1 = %q, expected 'XXa'", v.textLines[1])
	}
}

func TestEditInsertTextMultiLine(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(3) // 'a'
	before := len(v.textLines)
	v.editInsertText("x\ny")
	if len(v.textLines) != before+1 {
		t.Errorf("lines = %d, want %d", len(v.textLines), before+1)
	}
	if v.textLines[1] != `  "x` {
		t.Errorf("line 1 = %q, want %q", v.textLines[1], `  "x`)
	}
	if v.textLines[2] != `ya": 1` {
		t.Errorf("line 2 = %q, want %q", v.textLines[2], `ya": 1`)
	}
}

func TestSelectionDraw(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)

	v := newViewer(nil, `{"a":1}`, "test", false)
	v.SetRect(0, 0, 80, 24)
	v.cursor = 1
	v.enterEdit(2)
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))

	v.Draw(s)
	s.Show()

	_, selBg, _ := stSelection.Decompose()
	found := false
	for x := 0; x < 40; x++ {
		_, _, st, _ := s.GetContent(x, 1)
		_, bg, _ := st.Decompose()
		if bg == selBg {
			found = true
			break
		}
	}
	if !found {
		t.Error("no selection-highlighted cells found on line 1")
	}
}

func TestUndoClearsSelection(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(2)
	// Type something to create an undo checkpoint.
	v.editKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	// Select.
	v.editKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if !v.hasSelection() {
		t.Fatal("expected selection")
	}
	// Undo restores pre-edit state and clears selection.
	v.editKey(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone))
	if v.hasSelection() {
		t.Error("selection should be cleared after undo")
	}
}

func TestPasteHandler(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(3) // 'a'

	v.handlePaste("hello")

	if !strings.Contains(v.textLines[1], "hello") {
		t.Errorf("line 1 = %q, expected 'hello' pasted", v.textLines[1])
	}
}

func TestPasteMultiLine(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(0)
	before := len(v.textLines)

	v.handlePaste("x\ny\nz")

	if len(v.textLines) != before+2 {
		t.Errorf("lines = %d, want %d", len(v.textLines), before+2)
	}
	if v.textLines[1] != "x" {
		t.Errorf("line 1 = %q, want %q", v.textLines[1], "x")
	}
	if v.textLines[3] != `z  "a": 1` {
		t.Errorf("line 3 = %q, want %q", v.textLines[3], `z  "a": 1`)
	}
}

func TestPasteAutoEnterEdit(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	// Not in edit mode - handlePaste should auto-enter edit mode.
	v.handlePaste("hello")
	if !v.editing {
		t.Fatal("expected editing mode after paste")
	}
	if !strings.Contains(v.textLines[1], "hello") {
		t.Errorf("line 1 = %q, expected 'hello' pasted", v.textLines[1])
	}
}

func TestCtrlJAsNewline(t *testing.T) {
	v := newTestViewer(t, `{"a":1}`)
	v.cursor = 1
	v.enterEdit(0) // start of line 1
	before := len(v.textLines)
	// KeyCtrlJ (tcell's mapping for \n) should act as Enter.
	v.editKey(tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl))
	if len(v.textLines) != before+1 {
		t.Errorf("lines = %d, want %d", len(v.textLines), before+1)
	}
}

func TestGutterSeparator(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)

	v := newViewer(nil, sampleJSON, "test", false)
	v.SetRect(0, 0, 80, 24)
	v.Draw(s)
	s.Show()

	sepX := v.gutterW - 1
	ch, _, _, _ := s.GetContent(sepX, 0)
	if ch != '│' {
		t.Errorf("gutter separator at x=%d = %q, want '│'", sepX, ch)
	}

	// Content should start after the separator.
	ch, _, _, _ = s.GetContent(v.gutterW, 0)
	if ch == '│' {
		t.Error("content area should not start with separator")
	}
}
