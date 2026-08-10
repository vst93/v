package plugin_jv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// Syntax and UI styles. Plain ANSI palette colors are used so the viewer
// stays readable on both light and dark terminal themes.
var (
	stKey    = tcell.StyleDefault.Foreground(tcell.ColorMaroon)
	stString = tcell.StyleDefault.Foreground(tcell.ColorBlue)
	stNumber = tcell.StyleDefault.Foreground(tcell.ColorTeal)
	stBool   = tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	stNull   = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	stPunct  = tcell.StyleDefault

	stGutter     = tcell.StyleDefault.Foreground(tcell.ColorGray)
	stFoldPill   = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGray)
	stMatch      = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
	stMatchCur   = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorOrange)
	stScrollTrak = tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	stScrollThum = tcell.StyleDefault.Foreground(tcell.ColorGray)
	stSelection  = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy)

	stPanel    = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkSlateGray)
	stPanelDim = tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorDarkSlateGray)
	stPanelOn  = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorSilver).Bold(true)

	stBarLabel = tcell.StyleDefault.Foreground(tcell.ColorTeal).Bold(true)
	stBarDim   = tcell.StyleDefault.Foreground(tcell.ColorGray)
	stBarAct   = tcell.StyleDefault.Foreground(tcell.ColorTeal).Bold(true)
	stBarActOn = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorTeal).Bold(true)
	stBarErr   = tcell.StyleDefault.Foreground(tcell.ColorRed)
	stToast    = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen)
)

// styleForClass maps a syntax class to its style.
func styleForClass(c segClass) tcell.Style {
	switch c {
	case clsKey:
		return stKey
	case clsString:
		return stString
	case clsNumber:
		return stNumber
	case clsBool:
		return stBool
	case clsNull:
		return stNull
	default:
		return stPunct
	}
}

// cellWidth returns the terminal cell width of a rune.
func cellWidth(r rune) int {
	switch {
	case r == '\t':
		return 4
	case r < 0x20 || r == 0x7f:
		return 1
	case r < 0x2e80:
		return 1
	default:
		if w := uniseg.StringWidth(string(r)); w > 0 {
			return w
		}
		return 1
	}
}

// cellsWidth returns the cell width of a rune slice.
func cellsWidth(rs []rune) int {
	w := 0
	for _, r := range rs {
		w += cellWidth(r)
	}
	return w
}

// rect is a screen rectangle used for mouse hit-testing.
type rect struct{ x, y, w, h int }

func (r rect) contains(px, py int) bool {
	return px >= r.x && px < r.x+r.w && py >= r.y && py < r.y+r.h
}

// textInput is a minimal single-line text field.
type textInput struct {
	text   []rune
	pos    int // cursor position in runes
	scroll int // first visible rune index
}

func (t *textInput) String() string { return string(t.text) }
func (t *textInput) set(s string)   { t.text = []rune(s); t.pos = len(t.text); t.scroll = 0 }
func (t *textInput) clear()         { t.text = nil; t.pos = 0; t.scroll = 0 }
func (t *textInput) insert(r rune) {
	t.text = append(t.text, 0)
	copy(t.text[t.pos+1:], t.text[t.pos:])
	t.text[t.pos] = r
	t.pos++
}
func (t *textInput) backspace() {
	if t.pos > 0 {
		t.text = append(t.text[:t.pos-1], t.text[t.pos:]...)
		t.pos--
	}
}
func (t *textInput) del() {
	if t.pos < len(t.text) {
		t.text = append(t.text[:t.pos], t.text[t.pos+1:]...)
	}
}
func (t *textInput) killToEnd() { t.text = t.text[:t.pos] }
func (t *textInput) wordBackspace() {
	for t.pos > 0 && t.text[t.pos-1] == ' ' {
		t.backspace()
	}
	for t.pos > 0 && t.text[t.pos-1] != ' ' {
		t.backspace()
	}
}

// handleKey processes an editing key. Returns false for keys it does not
// handle so the caller can apply its own bindings.
func (t *textInput) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyRune:
		t.insert(ev.Rune())
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		t.backspace()
	case tcell.KeyDelete:
		t.del()
	case tcell.KeyLeft:
		if t.pos > 0 {
			t.pos--
		}
	case tcell.KeyRight:
		if t.pos < len(t.text) {
			t.pos++
		}
	case tcell.KeyHome, tcell.KeyCtrlA:
		t.pos = 0
	case tcell.KeyEnd, tcell.KeyCtrlE:
		t.pos = len(t.text)
	case tcell.KeyCtrlU:
		t.clear()
	case tcell.KeyCtrlW:
		t.wordBackspace()
	case tcell.KeyCtrlK:
		t.killToEnd()
	default:
		return false
	}
	return true
}

// draw renders the input into r, keeping the cursor visible.
func (t *textInput) draw(s tcell.Screen, r rect, st tcell.Style, placeholder string, phSt tcell.Style) {
	if t.pos < t.scroll {
		t.scroll = t.pos
	}
	for cellsWidth(t.text[t.scroll:t.pos]) > r.w-1 && t.scroll < t.pos {
		t.scroll++
	}
	if len(t.text) == 0 && placeholder != "" {
		cx := r.x
		for _, ch := range placeholder {
			rw := cellWidth(ch)
			if cx+rw > r.x+r.w {
				break
			}
			s.SetContent(cx, r.y, ch, nil, phSt)
			cx += rw
		}
		return
	}
	cx := r.x
	for i := t.scroll; i < len(t.text); i++ {
		rw := cellWidth(t.text[i])
		if cx+rw > r.x+r.w {
			break
		}
		s.SetContent(cx, r.y, t.text[i], nil, st)
		cx += rw
	}
}

// cursorX returns the screen x of the text cursor within r.
func (t *textInput) cursorX(r rect) int {
	w := cellsWidth(t.text[t.scroll:t.pos])
	if w > r.w-1 {
		w = r.w - 1
	}
	return r.x + w
}

// focusArea identifies which part of the UI owns the keyboard.
type focusArea int

const (
	focusEditor focusArea = iota
	focusSearch
	focusFilter
)

// searchMatch is one search hit: full-line index plus byte column range.
type searchMatch struct{ line, start, end int }

// barAction is a clickable action in the bottom toolbar. on reports
// whether a toggle action is currently active (nil for momentary ones).
type barAction struct {
	label string
	on    func() bool
	run   func()
}

// docSnapshot is an undo checkpoint of the document and text cursor.
type docSnapshot struct {
	lines []string
	line  int
	col   int
}

// Viewer is an editor-style JSON viewer: line numbers, folding, search
// panel, path filter bar, smooth mouse/touchpad scrolling and text
// editing with graceful fallback for invalid JSON.
type Viewer struct {
	*tview.Box
	app    *tview.Application
	source string

	// Document model: textLines is the source of truth; tree is the
	// parsed JSON (nil while the text is not valid JSON).
	textLines []string
	rootLines []string    // unfiltered document (filter restore point)
	tree      interface{} // parsed textLines
	rootTree  interface{} // parsed unfiltered document
	errLine   int         // 1-based parse error line, 0 when valid
	errMsg    string
	filtered  bool
	escape    bool // render strings as \uXXXX

	lines      []displayLine
	containers []containerInfo
	folded     map[int]bool // keyed by container opening line
	visible    []int        // full-line indices currently visible
	cursor     int          // index into visible
	scroll     int          // first visible line shown (index into visible)
	hscroll    int          // horizontal scroll offset in cells
	maxWidth   int          // widest line in cells

	// Edit mode state.
	editing    bool
	editLine   int
	editCol    int // rune column within editLine
	goalCol    int // remembered column for vertical moves
	undo       []docSnapshot
	redo       []docSnapshot
	lastEdit   time.Time
	lastKind   string
	lastRune   rune
	lastRuneAt time.Time

	// Selection state (edit mode).
	selAnchorLine int
	selAnchorCol  int
	selActive     bool

	// search state
	searchOpen  bool
	searchInput textInput
	optCase     bool
	optWord     bool
	optRegex    bool
	matches     []searchMatch
	byLine      map[int][]searchMatch
	matchIdx    int
	matchActive bool // a match has been jumped to
	matchErr    string

	// filter bar state
	filterInput textInput
	filterErr   string

	focus    focusArea
	helpOpen bool
	toast    string
	toastAt  time.Time

	// mouse state
	lastWheel      time.Time
	dragThumb      bool
	grabRow        int
	mouseSelecting bool

	// geometry cached during Draw for hit-testing
	gutterW    int
	editorH    int
	thumbTop   int
	thumbH     int
	panelRect  rect
	inputRect  rect
	hitRects   []hitRect
	filterRect rect
	actionRect []actionRect

	// Theme-dependent cursor-line styles. The foreground stays at the
	// terminal default so it remains readable on both light and dark themes.
	curLineBg tcell.Color
	curGutter tcell.Style
}

type hitRect struct {
	r  rect
	id string
}

type actionRect struct {
	r   rect
	act *barAction
}

// splitLines splits text into lines, normalizing CRLF.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n")
}

// newViewer builds a Viewer from raw input text. Valid JSON is
// pretty-printed; anything else is shown as plain text.
func newViewer(app *tview.Application, input, source string, sortKeys bool) *Viewer {
	curLineBg, curGutter := cursorLineStyles(os.Getenv("COLORFGBG"))
	v := &Viewer{
		Box:       tview.NewBox().SetBackgroundColor(tcell.ColorDefault),
		app:       app,
		source:    source,
		folded:    make(map[int]bool),
		byLine:    make(map[int][]searchMatch),
		curLineBg: curLineBg,
		curGutter: curGutter,
	}
	tree, err := DecodeJSON(input)
	if err == nil {
		if sortKeys {
			if om, ok := tree.(*OrderedMap); ok {
				om.SortKeys()
			}
		}
		v.tree = tree
		v.rootTree = tree
		v.textLines = splitLines(FormatJSON(tree, 2, false))
	} else {
		v.errLine, v.errMsg = jsonErrLine(input, err), err.Error()
		v.textLines = splitLines(input)
	}
	if len(v.textLines) == 0 {
		v.textLines = []string{""}
	}
	v.rootLines = v.textLines
	v.rebuildFromText()
	return v
}

// jsonErrLine extracts the 1-based line number from a JSON error.
func jsonErrLine(text string, err error) int {
	var se *json.SyntaxError
	if errors.As(err, &se) && se.Offset > 0 {
		off := int(se.Offset) - 1
		if off > len(text) {
			off = len(text)
		}
		return strings.Count(text[:off], "\n") + 1
	}
	return 1
}

// RunInteractive launches the interactive editor-style viewer.
func RunInteractive(input, source string, sortKeys bool) error {
	app := tview.NewApplication()
	v := newViewer(app, input, source, sortKeys)
	return app.SetRoot(v, true).EnableMouse(true).Run()
}

// reparse re-parses the document text and updates the tree/error state.
// When unfiltered it also refreshes the filter restore point.
func (v *Viewer) reparse() {
	text := strings.Join(v.textLines, "\n")
	tree, err := DecodeJSON(text)
	if err != nil {
		v.tree = nil
		v.errLine, v.errMsg = jsonErrLine(text, err), err.Error()
	} else {
		v.tree = tree
		v.errLine, v.errMsg = 0, ""
	}
	if !v.filtered {
		v.rootTree = v.tree
		v.rootLines = append([]string(nil), v.textLines...)
	}
}

// rebuildFromText re-lexes the document text, preserving fold state
// (keyed by line number) and clamping the cursor.
func (v *Viewer) rebuildFromText() {
	v.lines, v.containers = lexDocument(v.textLines)
	v.maxWidth = 0
	for i := range v.lines {
		if v.lines[i].width > v.maxWidth {
			v.maxWidth = v.lines[i].width
		}
	}
	v.rebuildVisible()
	if v.cursor >= len(v.visible) {
		v.cursor = len(v.visible) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	v.runSearch()
}

// rebuildVisible recomputes the list of visible full-line indices by
// skipping folded container ranges.
func (v *Viewer) rebuildVisible() {
	v.visible = v.visible[:0]
	skipUntil := -1
	for i := range v.lines {
		if i <= skipUntil {
			continue
		}
		v.visible = append(v.visible, i)
		ln := &v.lines[i]
		if ln.kind == kindOpen && v.folded[i] {
			skipUntil = v.containers[ln.openID].closeLine
		}
	}
	if len(v.visible) == 0 { // defensive: root line is always visible
		v.visible = append(v.visible, 0)
	}
}

// clampState keeps scroll/cursor/hscroll within valid ranges.
func (v *Viewer) clampState() {
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.visible) {
		v.cursor = len(v.visible) - 1
	}
	maxScroll := len(v.visible) - v.editorH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.scroll > maxScroll {
		v.scroll = maxScroll
	}
	if v.scroll < 0 {
		v.scroll = 0
	}
	contentW := v.contentWidth()
	maxH := v.maxWidth - contentW + 4
	if maxH < 0 {
		maxH = 0
	}
	if v.hscroll > maxH {
		v.hscroll = maxH
	}
	if v.hscroll < 0 {
		v.hscroll = 0
	}
}

func (v *Viewer) contentWidth() int {
	_, _, w, _ := v.GetInnerRect()
	cw := w - v.gutterW - 1 // minus scrollbar column
	if cw < 1 {
		return 1
	}
	return cw
}

// ensureCursorVisible scrolls so the cursor line is on screen.
func (v *Viewer) ensureCursorVisible() {
	if v.cursor < v.scroll {
		v.scroll = v.cursor
	}
	if v.cursor >= v.scroll+v.editorH {
		v.scroll = v.cursor - v.editorH + 1
	}
}

// visiblePosOf returns the visible-space index of a full-line index, or
// the nearest following visible line when it is hidden.
func (v *Viewer) visiblePosOf(full int) int {
	pos := sort.SearchInts(v.visible, full)
	if pos >= len(v.visible) {
		return len(v.visible) - 1
	}
	return pos
}

// --- Drawing ---

// Draw renders the whole viewer.
func (v *Viewer) Draw(s tcell.Screen) {
	v.Box.DrawForSubclass(s, v)
	x, y, w, h := v.GetInnerRect()
	if w < 12 || h < 4 {
		return
	}
	v.editorH = h - 1
	v.gutterW = len(strconv.Itoa(len(v.lines))) + 3
	v.clampState()

	v.hitRects = v.hitRects[:0]
	v.actionRect = v.actionRect[:0]

	for row := 0; row < v.editorH; row++ {
		idx := v.scroll + row
		if idx >= len(v.visible) {
			break
		}
		v.drawLine(s, x, y+row, w-1, v.visible[idx], idx == v.cursor)
	}
	v.drawScrollbar(s, x+w-1, y, v.editorH)
	v.drawBar(s, x, y+h-1, w)
	if v.searchOpen {
		v.drawSearchPanel(s, x, y, w)
	}
	if v.helpOpen {
		v.drawHelp(s, x, y, w, h)
	}
	v.placeCursor(s)
}

// drawLine renders one display line with gutter, folding and highlights.
func (v *Viewer) drawLine(s tcell.Screen, x, row, width, full int, isCur bool) {
	ln := &v.lines[full]

	rowBase := tcell.StyleDefault
	if isCur {
		rowBase = rowBase.Background(v.curLineBg)
	}
	for i := 0; i < width; i++ {
		s.SetContent(x+i, row, ' ', nil, rowBase)
	}

	// Gutter: fold chevron + right-aligned line number.
	gutStyle := stGutter
	if isCur {
		gutStyle = v.curGutter
	}
	folded := ln.kind == kindOpen && v.folded[full]
	// In edit mode folding is disabled, so no fold indicators are drawn.
	if ln.kind == kindOpen && !v.editing {
		chev := '▾'
		if folded {
			chev = '▸'
		}
		s.SetContent(x, row, chev, nil, gutStyle)
	}
	num := strconv.Itoa(full + 1)
	for i, ch := range num {
		s.SetContent(x+v.gutterW-1-len(num)+i, row, ch, nil, gutStyle)
	}
	// Separator between gutter and content.
	s.SetContent(x+v.gutterW-1, row, '│', nil, gutStyle)

	// Content with syntax colors, horizontal scroll and match highlights.
	cx := x + v.gutterW
	limit := x + width
	spans := v.byLine[full]
	var curSpan *searchMatch
	if v.matchActive && v.matchIdx < len(v.matches) {
		if m := &v.matches[v.matchIdx]; m.line == full {
			curSpan = m
		}
	}

	col := 0
	byteOff := 0
	runeIdx := 0

	// Selection range for this line (-1 means no selection here).
	selStart, selEnd := -1, -1
	if v.editing && v.hasSelection() {
		sl, sc, el, ec := v.selBounds()
		switch {
		case full > sl && full < el:
			selStart, selEnd = 0, len([]rune(v.textLines[full]))
		case full == sl && full == el:
			selStart, selEnd = sc, ec
		case full == sl:
			selStart, selEnd = sc, len([]rune(v.textLines[full]))
		case full == el:
			selStart, selEnd = 0, ec
		}
	}

	drawRune := func(r rune, st tcell.Style) {
		rw := cellWidth(r)
		sx := cx + col - v.hscroll
		if sx >= cx && sx+rw <= limit {
			if r < 0x20 || r == 0x7f {
				s.SetContent(sx, row, '�', nil, st)
			} else {
				s.SetContent(sx, row, r, nil, st)
			}
		}
		col += rw
	}

	for _, sg := range ln.segs {
		st := styleForClass(sg.cls)
		if isCur {
			st = st.Background(v.curLineBg)
		}
		for _, r := range sg.text {
			rst := st
			for i := range spans {
				if byteOff >= spans[i].start && byteOff < spans[i].end {
					rst = stMatch
					break
				}
			}
			if curSpan != nil && byteOff >= curSpan.start && byteOff < curSpan.end {
				rst = stMatchCur
			}
			if selStart >= 0 && runeIdx >= selStart && runeIdx < selEnd {
				rst = stSelection
			}
			drawRune(r, rst)
			byteOff += utf8.RuneLen(r)
			runeIdx++
		}
	}

	// Folded container: placeholder pill + closing bracket (+ comma).
	if folded {
		for _, r := range " … " {
			drawRune(r, stFoldPill)
		}
		st := stPunct
		if isCur {
			st = st.Background(v.curLineBg)
		}
		c := &v.containers[ln.openID]
		for _, r := range c.closeText {
			drawRune(r, st)
		}
		if c.closeComma {
			drawRune(',', st)
		}
	}
}

// drawScrollbar renders the scroll track and thumb on the right edge.
func (v *Viewer) drawScrollbar(s tcell.Screen, sx, y, height int) {
	v.thumbTop, v.thumbH = -1, 0
	total := len(v.visible)
	if total <= height || height < 2 {
		return
	}
	for row := 0; row < height; row++ {
		s.SetContent(sx, y+row, '│', nil, stScrollTrak)
	}
	thumbH := height * height / total
	if thumbH < 1 {
		thumbH = 1
	}
	thumbTop := y + (height-thumbH)*v.scroll/(total-height)
	for row := thumbTop; row < thumbTop+thumbH; row++ {
		s.SetContent(sx, row, '█', nil, stScrollThum)
	}
	v.thumbTop, v.thumbH = thumbTop-y, thumbH
}

// drawBar renders the bottom filter/status/action bar.
func (v *Viewer) drawBar(s tcell.Screen, x, y, w int) {
	actions := []barAction{
		{"{}", nil, func() { v.copyFormatted() }},
		{"><", nil, func() { v.copyMinified() }},
		{`><\u`, nil, func() { v.copyMinifiedEscaped() }},
		{"▾▾", nil, func() { v.expandAll() }},
		{"▸▸", nil, func() { v.collapseAll() }},
		{`\u`, func() bool { return v.escape }, func() { v.toggleEscape() }},
		{"val", nil, func() { v.copyValue() }},
		{"path", nil, func() { v.copyPath() }},
		{"?", func() bool { return v.helpOpen }, func() { v.helpOpen = !v.helpOpen }},
	}

	// Measure actions from the right edge (cell widths, not bytes).
	ax := x + w
	type drawnAction struct {
		a     *barAction
		start int
	}
	var drawn []drawnAction
	for i := len(actions) - 1; i >= 0; i-- {
		a := &actions[i]
		lw := cellsWidth([]rune(a.label)) + 2
		start := ax - lw - 1
		if start < x+w/3 {
			break
		}
		drawn = append(drawn, drawnAction{a, start})
		ax = start
	}

	// Status text left of the actions.
	parts := []string{fmt.Sprintf("Ln %d/%d", v.visible[v.cursor]+1, len(v.lines))}
	if v.editing {
		parts = append(parts, "EDIT")
	}
	if v.filtered {
		parts = append(parts, "filtered")
	}
	if v.errLine > 0 {
		parts = append(parts, fmt.Sprintf("✗ invalid JSON: line %d", v.errLine))
	}
	status := strings.Join(parts, " · ")
	stStyle := stBarDim
	if v.errLine > 0 {
		stStyle = stBarErr
	}
	statusX := ax - cellsWidth([]rune(status)) - 1
	if statusX < x+12 {
		status = "" // not enough room
		statusX = ax
	}

	// Filter label + input occupy the remaining left region.
	cx := x + 1
	for _, ch := range "this" {
		s.SetContent(cx, y, ch, nil, stBarLabel)
		cx++
	}
	cx++
	inputW := statusX - cx - 1
	if inputW < 8 {
		inputW = 8
	}
	if cx+inputW > x+w-1 {
		inputW = x + w - 1 - cx
	}
	v.filterRect = rect{cx, y, inputW, 1}
	ph := ".key.subkey  [0][1]  .length  .map(.id)"
	v.filterInput.draw(s, v.filterRect, tcell.StyleDefault, ph, stBarDim)

	// Filter error indicator, right after the input text.
	if v.filterErr != "" {
		msg := " ✗ " + v.filterErr
		ex := cx + cellsWidth(v.filterInput.text) + 1
		for _, ch := range msg {
			if ex >= statusX-1 {
				break
			}
			s.SetContent(ex, y, ch, nil, stBarErr)
			ex++
		}
	}

	// Status text.
	scx := statusX
	for _, ch := range status {
		s.SetContent(scx, y, ch, nil, stStyle)
		scx++
	}

	// Toast: transient banner drawn over the filter area.
	if v.toast != "" && time.Since(v.toastAt) < 2*time.Second {
		end := statusX - 1
		if end > ax-1 {
			end = ax - 1
		}
		for cx := x + 1; cx < end; cx++ {
			s.SetContent(cx, y, ' ', nil, stToast)
		}
		tcx := x + 2
		for _, ch := range v.toast {
			if tcx >= end-1 {
				break
			}
			s.SetContent(tcx, y, ch, nil, stToast)
			tcx++
		}
	}

	// Action buttons; active toggles get an inverted background.
	for _, da := range drawn {
		s.SetContent(da.start, y, '[', nil, stBarDim)
		ccx := da.start + 1
		lst := stBarAct
		if da.a.on != nil && da.a.on() {
			lst = stBarActOn
		}
		for _, ch := range da.a.label {
			s.SetContent(ccx, y, ch, nil, lst)
			ccx++
		}
		s.SetContent(ccx, y, ']', nil, stBarDim)
		v.actionRect = append(v.actionRect, actionRect{r: rect{da.start, y, ccx - da.start + 1, 1}, act: da.a})
	}
}

// drawSearchPanel renders the top-right search overlay.
func (v *Viewer) drawSearchPanel(s tcell.Screen, x, y, w int) {
	sw := w - 2
	if sw > 62 {
		sw = 62
	}
	if sw < 24 {
		sw = w
	}
	px := x + w - sw
	v.panelRect = rect{px, y, sw, 1}
	for i := 0; i < sw; i++ {
		s.SetContent(px+i, y, ' ', nil, stPanel)
	}
	if sw < 24 {
		v.drawCompactSearchPanel(s, px, y, sw)
		return
	}

	count := ""
	if v.matchErr != "" {
		count = " bad pattern "
	} else if len(v.matches) == 0 {
		if v.searchInput.String() != "" {
			count = " 0 of 0 "
		}
	} else if v.matchActive {
		count = fmt.Sprintf(" %d of %d ", v.matchIdx+1, len(v.matches))
	} else {
		count = fmt.Sprintf(" ? of %d ", len(v.matches))
	}

	fixed := 2 + 3*4 + cellsWidth([]rune(count)) + 3*3 + 3
	iw := sw - fixed
	if iw < 4 {
		iw = 4
	}

	cx := px + 1
	s.SetContent(cx, y, '>', nil, stPanelDim)
	cx++
	s.SetContent(cx, y, ' ', nil, stPanel)
	cx++
	v.inputRect = rect{cx, y, iw, 1}
	v.searchInput.draw(s, v.inputRect, stPanel, "Search", stPanelDim)
	cx += iw + 1

	// Option toggles.
	toggles := []struct {
		id, label string
		on        bool
	}{
		{"case", "Aa", v.optCase},
		{"word", "ab", v.optWord},
		{"regex", ".*", v.optRegex},
	}
	for _, tg := range toggles {
		st := stPanelDim
		if tg.on {
			st = stPanelOn
		}
		label := "[" + tg.label + "]"
		tx := cx
		for _, ch := range label {
			s.SetContent(tx, y, ch, nil, st)
			tx++
		}
		v.hitRects = append(v.hitRects, hitRect{r: rect{cx, y, tx - cx, 1}, id: tg.id})
		cx = tx
	}

	for _, ch := range count {
		s.SetContent(cx, y, ch, nil, stPanel)
		cx++
	}

	// Navigation / close buttons.
	for _, b := range []struct{ id, label string }{
		{"prev", "↑"}, {"next", "↓"}, {"close", "×"},
	} {
		label := "[" + b.label + "]"
		tx := cx
		for _, ch := range label {
			s.SetContent(tx, y, ch, nil, stPanel)
			tx++
		}
		v.hitRects = append(v.hitRects, hitRect{r: rect{cx, y, tx - cx, 1}, id: b.id})
		cx = tx
	}
	v.hitRects = append(v.hitRects, hitRect{r: v.inputRect, id: "input"})
}

// drawCompactSearchPanel keeps the search field usable without drawing the
// option and navigation buttons outside a narrow terminal viewport.
func (v *Viewer) drawCompactSearchPanel(s tcell.Screen, x, y, width int) {
	cx := x + 1
	s.SetContent(cx, y, '>', nil, stPanelDim)
	cx += 2
	inputW := width - 7 // left prompt, gap, right gap, and [x]
	if inputW < 1 {
		inputW = 1
	}
	v.inputRect = rect{cx, y, inputW, 1}
	v.searchInput.draw(s, v.inputRect, stPanel, "Search", stPanelDim)
	v.hitRects = append(v.hitRects, hitRect{r: v.inputRect, id: "input"})

	closeX := x + width - 3
	ccx := closeX
	for _, ch := range "[×]" {
		s.SetContent(ccx, y, ch, nil, stPanel)
		ccx += cellWidth(ch)
	}
	v.hitRects = append(v.hitRects, hitRect{r: rect{closeX, y, 3, 1}, id: "close"})
}

// drawHelp renders the centered help overlay.
func (v *Viewer) drawHelp(s tcell.Screen, x, y, w, h int) {
	lines := []struct{ key, desc string }{
		{"↑ ↓ / j k", "move cursor"},
		{"PgUp PgDn", "page up / down"},
		{"g / G", "first / last line"},
		{"← →", "scroll horizontally"},
		{"Enter Space o", "fold / unfold at cursor"},
		{"h / l", "fold, jump to parent / unfold"},
		{"e / c", "expand / collapse all"},
		{"i", "inline edit (Esc done, Ctrl-Z/Y undo/redo)"},
		{"edit: select", "Shift+arrows · Ctrl-A all · Ctrl-C/X/V copy/cut/paste · Ctrl+word"},
		{"edit: mouse", "drag select · double-click word · Shift+click extend"},
		{"I", "edit in $EDITOR (vim/vi/nano/notepad); returns on save"},
		{"f", "reformat document"},
		{"/  or  Ctrl-F", "open search panel"},
		{"n / N / F3", "next / previous match"},
		{"search panel", "Enter next · Shift-Enter prev · Alt-C case · Alt-W word · Alt-R regex · Esc close"},
		{"Tab", "filter bar: .key  [0]  [\"k\"]  .length  .map(.k)"},
		{"u", "toggle \\uXXXX display"},
		{"F / M / E", "copy formatted / minified / minified+\\uXXXX JSON"},
		{"y / p", "copy value / path at cursor"},
		{"mouse", "wheel scrolls view · click selects · double-click edits · gutter folds · drag scrollbar"},
		{"?  /  q", "this help / quit"},
	}
	bw := 84
	bh := len(lines) + 2
	if bw > w-4 {
		bw = w - 4
	}
	if bh > h-2 {
		bh = h - 2
	}
	bx := x + (w-bw)/2
	by := y + (h-bh)/2

	stBox := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkSlateGray)
	stBoxKey := tcell.StyleDefault.Foreground(tcell.ColorTeal).Background(tcell.ColorDarkSlateGray).Bold(true)
	stBoxTitle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorDarkSlateGray).Bold(true)

	for row := 0; row < bh; row++ {
		for col := 0; col < bw; col++ {
			s.SetContent(bx+col, by+row, ' ', nil, stBox)
		}
	}
	// Border.
	for col := 0; col < bw; col++ {
		s.SetContent(bx+col, by, '─', nil, stBox)
		s.SetContent(bx+col, by+bh-1, '─', nil, stBox)
	}
	for row := 0; row < bh; row++ {
		s.SetContent(bx, by+row, '│', nil, stBox)
		s.SetContent(bx+bw-1, by+row, '│', nil, stBox)
	}
	s.SetContent(bx, by, '┌', nil, stBox)
	s.SetContent(bx+bw-1, by, '┐', nil, stBox)
	s.SetContent(bx, by+bh-1, '└', nil, stBox)
	s.SetContent(bx+bw-1, by+bh-1, '┘', nil, stBox)
	title := " jv — keys "
	for i, ch := range title {
		s.SetContent(bx+2+i, by, ch, nil, stBoxTitle)
	}
	for i, ln := range lines {
		if i+1 >= bh-1 {
			break
		}
		row := by + 1 + i
		cx := bx + 3
		for _, ch := range ln.key {
			if cx >= bx+bw-2 {
				break
			}
			s.SetContent(cx, row, ch, nil, stBoxKey)
			cx++
		}
		cx = bx + 22
		for _, ch := range ln.desc {
			if cx >= bx+bw-2 {
				break
			}
			s.SetContent(cx, row, ch, nil, stBox)
			cx++
		}
	}
}

// placeCursor positions the hardware cursor in the active input or, in
// edit mode, at the text cursor.
func (v *Viewer) placeCursor(s tcell.Screen) {
	switch v.focus {
	case focusSearch:
		if v.searchOpen {
			s.ShowCursor(v.searchInput.cursorX(v.inputRect), v.inputRect.y)
			return
		}
	case focusFilter:
		s.ShowCursor(v.filterInput.cursorX(v.filterRect), v.filterRect.y)
		return
	}
	if v.editing {
		x, y, _, _ := v.GetInnerRect()
		row := v.visiblePosOf(v.editLine) - v.scroll
		if row >= 0 && row < v.editorH {
			rs := []rune(v.textLines[v.editLine])
			col := v.editCol
			if col > len(rs) {
				col = len(rs)
			}
			cx := x + v.gutterW + cellsWidth(rs[:col]) - v.hscroll
			if cx >= x+v.gutterW {
				s.ShowCursor(cx, y+row)
				return
			}
		}
	}
	s.HideCursor()
}

// --- Keyboard ---

// InputHandler routes key events to the focused area.
func (v *Viewer) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return func(ev *tcell.EventKey, setFocus func(tview.Primitive)) {
		if ev.Key() == tcell.KeyCtrlC && !v.editing {
			v.app.Stop()
			return
		}
		if v.helpOpen {
			v.helpOpen = false
			return
		}
		switch v.focus {
		case focusSearch:
			v.searchKey(ev)
		case focusFilter:
			v.filterKey(ev)
		default:
			if v.editing {
				v.editKey(ev)
			} else {
				v.editorKey(ev)
			}
		}
	}
}

// editorKey handles keys while the editor has focus (normal mode).
func (v *Viewer) editorKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyUp:
		v.moveCursor(-1)
	case tcell.KeyDown:
		v.moveCursor(1)
	case tcell.KeyPgUp:
		v.moveCursor(-v.editorH)
	case tcell.KeyPgDn:
		v.moveCursor(v.editorH)
	case tcell.KeyLeft:
		v.hscroll -= 8
	case tcell.KeyRight:
		v.hscroll += 8
	case tcell.KeyHome:
		v.hscroll = 0
	case tcell.KeyEnd:
		v.hscroll = v.maxWidth
	case tcell.KeyEnter:
		v.toggleFoldAtCursor()
	case tcell.KeyEsc:
		if v.searchOpen {
			v.searchOpen = false
		}
	case tcell.KeyCtrlF:
		v.openSearch()
	case tcell.KeyF3:
		if ev.Modifiers()&tcell.ModShift != 0 {
			v.stepMatch(-1)
		} else {
			v.stepMatch(1)
		}
	case tcell.KeyTab:
		v.focus = focusFilter
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			v.app.Stop()
		case 'j':
			v.moveCursor(1)
		case 'k':
			v.moveCursor(-1)
		case 'g':
			v.cursor = 0
			v.ensureCursorVisible()
		case 'G':
			v.cursor = len(v.visible) - 1
			v.ensureCursorVisible()
		case ' ', 'o':
			v.toggleFoldAtCursor()
		case 'h':
			v.foldOrParent()
		case 'l':
			v.expandAtCursor()
		case 'e':
			v.expandAll()
		case 'c':
			v.collapseAll()
		case 'i':
			v.enterEdit(-1)
			v.setToast("EDIT: Esc done · Shift+arrows select · Ctrl-A/C/X/V · Ctrl-Z/Y")
		case 'I':
			v.editExternal()
		case 'f':
			v.reformat()
		case 'u':
			v.toggleEscape()
		case 'F':
			v.copyFormatted()
		case 'M':
			v.copyMinified()
		case 'E':
			v.copyMinifiedEscaped()
		case 'y':
			v.copyValue()
		case 'p':
			v.copyPath()
		case '/':
			v.openSearch()
		case 'n':
			v.stepMatch(1)
		case 'N':
			v.stepMatch(-1)
		case '?':
			v.helpOpen = true
		}
	}
}

// searchKey handles keys while the search panel has focus.
func (v *Viewer) searchKey(ev *tcell.EventKey) {
	if ev.Modifiers()&tcell.ModAlt != 0 && ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'c', 'C':
			v.optCase = !v.optCase
		case 'w', 'W':
			v.optWord = !v.optWord
		case 'r', 'R':
			v.optRegex = !v.optRegex
		}
		v.runSearch()
		return
	}
	switch ev.Key() {
	case tcell.KeyEnter:
		if ev.Modifiers()&tcell.ModShift != 0 {
			v.stepMatch(-1)
		} else {
			v.stepMatch(1)
		}
	case tcell.KeyUp:
		v.stepMatch(-1)
	case tcell.KeyDown:
		v.stepMatch(1)
	case tcell.KeyEsc:
		v.searchOpen = false
		v.focus = focusEditor
	case tcell.KeyTab:
		v.focus = focusEditor
	default:
		if v.searchInput.handleKey(ev) {
			v.runSearch()
		}
	}
}

// filterKey handles keys while the filter bar has focus.
func (v *Viewer) filterKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEnter:
		v.applyFilter()
	case tcell.KeyEsc, tcell.KeyTab:
		v.filterErr = ""
		v.focus = focusEditor
	default:
		v.filterInput.handleKey(ev)
	}
}

// --- Editing ---

// enterEdit switches to edit mode at the current line. col < 0 means
// auto-position at the first non-whitespace character.
func (v *Viewer) enterEdit(col int) {
	if v.filtered {
		v.setToast("clear the filter before editing")
		return
	}
	// Folding is incompatible with editing: collapsed ranges hide real
	// text and the fold placeholder is not part of the document. Unfold
	// everything so edit lines map 1:1 onto the visible buffer. Fold
	// indicators are also omitted while editing (see drawLine).
	if len(v.folded) > 0 {
		v.folded = make(map[int]bool)
		v.rebuildVisible()
	}
	v.editing = true
	v.clearSelection()
	v.editLine = v.visible[v.cursor]
	n := len([]rune(v.textLines[v.editLine]))
	if col < 0 {
		col = 0
		for i, r := range v.textLines[v.editLine] {
			if r != ' ' && r != '\t' {
				col = i
				break
			}
		}
	}
	if col > n {
		col = n
	}
	v.editCol = col
	v.goalCol = col
}

// exitEdit returns to normal mode.
func (v *Viewer) exitEdit() {
	v.editing = false
	v.clearSelection()
	v.cursor = v.visiblePosOf(v.editLine)
	v.ensureCursorVisible()
}

// syncEditCursor keeps the normal cursor on the edit line.
func (v *Viewer) syncEditCursor() {
	v.cursor = v.visiblePosOf(v.editLine)
	v.ensureCursorVisible()
}

// editKey handles keys in edit mode.
func (v *Viewer) editKey(ev *tcell.EventKey) {
	sel := ev.Modifiers()&tcell.ModShift != 0
	ctrl := ev.Modifiers()&tcell.ModCtrl != 0
	switch ev.Key() {
	case tcell.KeyEsc:
		v.exitEdit()
	case tcell.KeyTab:
		v.exitEdit()
		v.focus = focusFilter
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		if v.hasSelection() {
			v.pushUndo("newline")
			v.deleteSelection()
		}
		v.editNewline()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.hasSelection() {
			v.pushUndo("delete")
			v.deleteSelection()
		} else {
			v.editBackspace()
		}
	case tcell.KeyDelete:
		if v.hasSelection() {
			v.pushUndo("delete")
			v.deleteSelection()
		} else {
			v.editDeleteForward()
		}
	case tcell.KeyLeft:
		if ctrl {
			v.editMoveWord(-1, sel)
		} else {
			v.editMoveHoriz(-1, sel)
		}
	case tcell.KeyRight:
		if ctrl {
			v.editMoveWord(1, sel)
		} else {
			v.editMoveHoriz(1, sel)
		}
	case tcell.KeyUp:
		v.editMoveVert(-1, sel)
	case tcell.KeyDown:
		v.editMoveVert(1, sel)
	case tcell.KeyHome:
		v.editHomeEnd(false, sel)
	case tcell.KeyEnd:
		v.editHomeEnd(true, sel)
	case tcell.KeyCtrlA:
		v.selectAll()
	case tcell.KeyCtrlC:
		v.copySelection()
	case tcell.KeyCtrlX:
		v.cutSelection()
	case tcell.KeyCtrlV:
		v.pasteClipboard()
	case tcell.KeyCtrlZ:
		v.undoEdit()
	case tcell.KeyCtrlY:
		v.redoEdit()
	case tcell.KeyRune:
		r := ev.Rune()
		// Guard against terminal/IME quirks that fire duplicate events.
		if r == v.lastRune && time.Since(v.lastRuneAt) < 5*time.Millisecond {
			return
		}
		v.lastRune = r
		v.lastRuneAt = time.Now()
		if v.hasSelection() {
			v.pushUndo("replace")
			v.deleteSelection()
		} else {
			v.pushUndo("insert")
		}
		v.editInsert(r)
	}
}

// pushUndo records an undo checkpoint unless edits can be coalesced.
func (v *Viewer) pushUndo(kind string) {
	coalesce := kind == "insert" && v.lastKind == "insert" && time.Since(v.lastEdit) < 1500*time.Millisecond
	v.lastKind = kind
	v.lastEdit = time.Now()
	if coalesce {
		return
	}
	cp := append([]string(nil), v.textLines...)
	v.undo = append(v.undo, docSnapshot{lines: cp, line: v.editLine, col: v.editCol})
	if len(v.undo) > 200 {
		v.undo = v.undo[1:]
	}
	v.redo = nil
}

// afterEdit re-parses and re-lexes the document after a mutation.
func (v *Viewer) afterEdit() {
	v.reparse()
	v.rebuildFromText()
	v.syncEditCursor()
}

// editInsert inserts one rune at the text cursor.
func (v *Viewer) editInsert(r rune) {
	rs := []rune(v.textLines[v.editLine])
	if v.editCol > len(rs) {
		v.editCol = len(rs)
	}
	rs = append(rs, 0)
	copy(rs[v.editCol+1:], rs[v.editCol:])
	rs[v.editCol] = r
	v.textLines[v.editLine] = string(rs)
	v.editCol++
	v.goalCol = v.editCol
	v.afterEdit()
}

// editBackspace deletes before the cursor, joining lines at column 0.
func (v *Viewer) editBackspace() {
	if v.editCol > 0 {
		v.pushUndo("delete")
		rs := []rune(v.textLines[v.editLine])
		rs = append(rs[:v.editCol-1], rs[v.editCol:]...)
		v.textLines[v.editLine] = string(rs)
		v.editCol--
		v.goalCol = v.editCol
		v.afterEdit()
		return
	}
	if v.editLine == 0 {
		return
	}
	v.pushUndo("delete")
	prev := []rune(v.textLines[v.editLine-1])
	v.editCol = len(prev)
	v.textLines[v.editLine-1] += v.textLines[v.editLine]
	v.textLines = append(v.textLines[:v.editLine], v.textLines[v.editLine+1:]...)
	v.editLine--
	v.goalCol = v.editCol
	v.afterEdit()
}

// editDeleteForward deletes at the cursor, joining lines at end of line.
func (v *Viewer) editDeleteForward() {
	rs := []rune(v.textLines[v.editLine])
	if v.editCol < len(rs) {
		v.pushUndo("delete")
		rs = append(rs[:v.editCol], rs[v.editCol+1:]...)
		v.textLines[v.editLine] = string(rs)
		v.afterEdit()
		return
	}
	if v.editLine >= len(v.textLines)-1 {
		return
	}
	v.pushUndo("delete")
	v.textLines[v.editLine] += v.textLines[v.editLine+1]
	v.textLines = append(v.textLines[:v.editLine+1], v.textLines[v.editLine+2:]...)
	v.afterEdit()
}

// editNewline splits the current line at the cursor, carrying the
// current line's indentation to the new line.
func (v *Viewer) editNewline() {
	v.pushUndo("newline")
	rs := []rune(v.textLines[v.editLine])
	if v.editCol > len(rs) {
		v.editCol = len(rs)
	}
	before := string(rs[:v.editCol])
	after := string(rs[v.editCol:])
	indent := ""
	for _, r := range rs {
		if r == ' ' || r == '\t' {
			indent += string(r)
		} else {
			break
		}
	}
	v.textLines[v.editLine] = before
	v.textLines = append(v.textLines, "") // grow
	copy(v.textLines[v.editLine+2:], v.textLines[v.editLine+1:])
	v.textLines[v.editLine+1] = indent + after
	v.editLine++
	v.editCol = len([]rune(indent))
	v.goalCol = v.editCol
	v.afterEdit()
}

// editMoveHoriz moves the text cursor left/right, crossing line edges.
// The cursor may sit at end-of-line (editCol == len) to allow appending
// and deleting at EOL; only pressing right again past EOL crosses to
// the next line.
func (v *Viewer) editMoveHoriz(d int, extendSel bool) {
	v.maybeStartSelection(extendSel)
	v.editCol += d
	if v.editCol < 0 {
		pos := v.visiblePosOf(v.editLine)
		if pos > 0 {
			v.editLine = v.visible[pos-1]
			v.editCol = len([]rune(v.textLines[v.editLine]))
		} else {
			v.editCol = 0
		}
		v.goalCol = v.editCol
		v.syncEditCursor()
		return
	}
	n := len([]rune(v.textLines[v.editLine]))
	if v.editCol > n {
		pos := v.visiblePosOf(v.editLine)
		if pos < len(v.visible)-1 {
			v.editLine = v.visible[pos+1]
			v.editCol = 0
		} else {
			v.editCol = n
		}
		v.goalCol = v.editCol
		v.syncEditCursor()
		return
	}
	v.goalCol = v.editCol
	v.syncEditCursor()
}

// editMoveVert moves the text cursor between visible lines.
func (v *Viewer) editMoveVert(d int, extendSel bool) {
	v.maybeStartSelection(extendSel)
	pos := v.visiblePosOf(v.editLine) + d
	if pos < 0 || pos >= len(v.visible) {
		return
	}
	v.editLine = v.visible[pos]
	n := len([]rune(v.textLines[v.editLine]))
	if v.goalCol > n {
		v.editCol = n
	} else {
		v.editCol = v.goalCol
	}
	v.syncEditCursor()
}

// undoEdit restores the previous document snapshot.
func (v *Viewer) undoEdit() {
	if len(v.undo) == 0 {
		v.setToast("nothing to undo")
		return
	}
	v.redo = append(v.redo, docSnapshot{lines: append([]string(nil), v.textLines...), line: v.editLine, col: v.editCol})
	s := v.undo[len(v.undo)-1]
	v.undo = v.undo[:len(v.undo)-1]
	v.textLines = s.lines
	v.editLine = s.line
	v.editCol = s.col
	v.lastKind = "undo"
	v.clearSelection()
	v.clampEditCursor()
	v.reparse()
	v.rebuildFromText()
	v.syncEditCursor()
}

// redoEdit reapplies the last undone snapshot.
func (v *Viewer) redoEdit() {
	if len(v.redo) == 0 {
		v.setToast("nothing to redo")
		return
	}
	v.undo = append(v.undo, docSnapshot{lines: append([]string(nil), v.textLines...), line: v.editLine, col: v.editCol})
	s := v.redo[len(v.redo)-1]
	v.redo = v.redo[:len(v.redo)-1]
	v.textLines = s.lines
	v.editLine = s.line
	v.editCol = s.col
	v.lastKind = "redo"
	v.clearSelection()
	v.clampEditCursor()
	v.reparse()
	v.rebuildFromText()
	v.syncEditCursor()
}

// clampEditCursor keeps the text cursor inside the document.
func (v *Viewer) clampEditCursor() {
	if v.editLine >= len(v.textLines) {
		v.editLine = len(v.textLines) - 1
	}
	if v.editLine < 0 {
		v.editLine = 0
	}
	n := len([]rune(v.textLines[v.editLine]))
	if v.editCol > n {
		v.editCol = n
	}
	if v.editCol < 0 {
		v.editCol = 0
	}
}

// --- Selection ---

// hasSelection reports whether a non-empty selection exists.
func (v *Viewer) hasSelection() bool {
	return v.selActive && (v.selAnchorLine != v.editLine || v.selAnchorCol != v.editCol)
}

// clearSelection drops the current selection.
func (v *Viewer) clearSelection() { v.selActive = false }

// maybeStartSelection begins or extends a selection when extendSel is
// true, or clears it otherwise.
func (v *Viewer) maybeStartSelection(extendSel bool) {
	if extendSel {
		if !v.selActive {
			v.selActive = true
			v.selAnchorLine = v.editLine
			v.selAnchorCol = v.editCol
		}
	} else {
		v.clearSelection()
	}
}

// selBounds returns the selection endpoints in document order:
// (startLine, startCol, endLine, endCol).
func (v *Viewer) selBounds() (int, int, int, int) {
	if v.selAnchorLine < v.editLine ||
		(v.selAnchorLine == v.editLine && v.selAnchorCol < v.editCol) {
		return v.selAnchorLine, v.selAnchorCol, v.editLine, v.editCol
	}
	return v.editLine, v.editCol, v.selAnchorLine, v.selAnchorCol
}

// selectedText returns the text between the selection endpoints.
func (v *Viewer) selectedText() string {
	if !v.hasSelection() {
		return ""
	}
	sl, sc, el, ec := v.selBounds()
	if sl == el {
		rs := []rune(v.textLines[sl])
		if sc > len(rs) {
			sc = len(rs)
		}
		if ec > len(rs) {
			ec = len(rs)
		}
		return string(rs[sc:ec])
	}
	var b strings.Builder
	rs := []rune(v.textLines[sl])
	if sc > len(rs) {
		sc = len(rs)
	}
	b.WriteString(string(rs[sc:]))
	b.WriteByte('\n')
	for l := sl + 1; l < el; l++ {
		b.WriteString(v.textLines[l])
		b.WriteByte('\n')
	}
	rs = []rune(v.textLines[el])
	if ec > len(rs) {
		ec = len(rs)
	}
	b.WriteString(string(rs[:ec]))
	return b.String()
}

// deleteSelection removes the selected text and collapses the cursor
// to the selection start.
func (v *Viewer) deleteSelection() {
	if !v.hasSelection() {
		return
	}
	sl, sc, el, ec := v.selBounds()
	if sl == el {
		rs := []rune(v.textLines[sl])
		if sc > len(rs) {
			sc = len(rs)
		}
		if ec > len(rs) {
			ec = len(rs)
		}
		rs = append(rs[:sc], rs[ec:]...)
		v.textLines[sl] = string(rs)
	} else {
		first := []rune(v.textLines[sl])
		last := []rune(v.textLines[el])
		if sc > len(first) {
			sc = len(first)
		}
		if ec > len(last) {
			ec = len(last)
		}
		v.textLines[sl] = string(first[:sc]) + string(last[ec:])
		v.textLines = append(v.textLines[:sl+1], v.textLines[el+1:]...)
	}
	v.editLine = sl
	v.editCol = sc
	v.goalCol = sc
	v.clearSelection()
	v.afterEdit()
}

// selectAll selects the entire document.
func (v *Viewer) selectAll() {
	if len(v.textLines) == 0 {
		return
	}
	v.selActive = true
	v.selAnchorLine = 0
	v.selAnchorCol = 0
	v.editLine = len(v.textLines) - 1
	v.editCol = len([]rune(v.textLines[v.editLine]))
	v.goalCol = v.editCol
	v.syncEditCursor()
}

// isWordChar reports whether r is part of a word (alnum or underscore).
func isWordChar(r rune) bool {
	return r == '_' ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

// selectWord selects the word at the cursor, or the single character
// under the cursor if it is not a word character.
func (v *Viewer) selectWord() {
	rs := []rune(v.textLines[v.editLine])
	n := len(rs)
	if n == 0 || v.editCol >= n {
		return
	}
	v.selActive = true
	v.selAnchorLine = v.editLine
	if !isWordChar(rs[v.editCol]) {
		v.selAnchorCol = v.editCol
		v.editCol++
		v.goalCol = v.editCol
		return
	}
	start := v.editCol
	for start > 0 && isWordChar(rs[start-1]) {
		start--
	}
	end := v.editCol
	for end < n && isWordChar(rs[end]) {
		end++
	}
	v.selAnchorCol = start
	v.editCol = end
	v.goalCol = v.editCol
}

// editMoveWord moves the cursor by one word boundary.
func (v *Viewer) editMoveWord(d int, extendSel bool) {
	v.maybeStartSelection(extendSel)
	rs := []rune(v.textLines[v.editLine])
	n := len(rs)
	if d > 0 {
		for v.editCol < n && !isWordChar(rs[v.editCol]) {
			v.editCol++
		}
		for v.editCol < n && isWordChar(rs[v.editCol]) {
			v.editCol++
		}
	} else {
		for v.editCol > 0 && !isWordChar(rs[v.editCol-1]) {
			v.editCol--
		}
		for v.editCol > 0 && isWordChar(rs[v.editCol-1]) {
			v.editCol--
		}
	}
	v.goalCol = v.editCol
	v.syncEditCursor()
}

// editHomeEnd moves to the start (toEnd=false) or end of the line.
func (v *Viewer) editHomeEnd(toEnd bool, extendSel bool) {
	v.maybeStartSelection(extendSel)
	if toEnd {
		v.editCol = len([]rune(v.textLines[v.editLine]))
	} else {
		v.editCol = 0
	}
	v.goalCol = v.editCol
	v.syncEditCursor()
}

// --- Selection clipboard ---

// copySelection copies the selected text (or current line if no
// selection) to the clipboard.
func (v *Viewer) copySelection() {
	text := v.selectedText()
	if text == "" {
		text = v.textLines[v.editLine]
	}
	if err := clipboardWrite(text); err != nil {
		v.setToast("Copy failed: " + err.Error())
		return
	}
	v.setToast("Copied")
}

// cutSelection copies the selected text to the clipboard and deletes it.
// With no selection it cuts the current line.
func (v *Viewer) cutSelection() {
	if !v.hasSelection() {
		text := v.textLines[v.editLine]
		v.pushUndo("delete")
		v.textLines = append(v.textLines[:v.editLine], v.textLines[v.editLine+1:]...)
		if v.editLine >= len(v.textLines) {
			v.editLine = len(v.textLines) - 1
		}
		v.editCol = 0
		v.goalCol = 0
		v.afterEdit()
		clipboardWrite(text)
		v.setToast("Cut line")
		return
	}
	text := v.selectedText()
	v.pushUndo("delete")
	v.deleteSelection()
	clipboardWrite(text)
	v.setToast("Cut")
}

// pasteClipboard inserts the clipboard contents at the cursor,
// replacing any selection.
func (v *Viewer) pasteClipboard() {
	text, err := clipboardRead()
	if err != nil || text == "" {
		v.setToast("Clipboard empty")
		return
	}
	v.handlePaste(text)
}

// handlePaste inserts text according to the current focus: into the
// document (edit mode), the search input, or the filter input.
func (v *Viewer) handlePaste(text string) {
	switch v.focus {
	case focusSearch:
		for _, r := range text {
			v.searchInput.insert(r)
		}
		v.runSearch()
	case focusFilter:
		for _, r := range text {
			v.filterInput.insert(r)
		}
	default:
		if !v.editing {
			v.enterEdit(-1)
			if !v.editing {
				return // enterEdit failed (e.g. filter active)
			}
		}
		v.pushUndo("paste")
		if v.hasSelection() {
			v.deleteSelection()
		}
		v.editInsertText(text)
	}
}

// editInsertText inserts a (possibly multi-line) string at the cursor.
func (v *Viewer) editInsertText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	parts := strings.Split(text, "\n")

	rs := []rune(v.textLines[v.editLine])
	if v.editCol > len(rs) {
		v.editCol = len(rs)
	}
	before := string(rs[:v.editCol])
	after := string(rs[v.editCol:])

	if len(parts) == 1 {
		v.textLines[v.editLine] = before + parts[0] + after
		v.editCol += len([]rune(parts[0]))
	} else {
		newLines := make([]string, 0, len(v.textLines)+len(parts))
		newLines = append(newLines, v.textLines[:v.editLine]...)
		newLines = append(newLines, before+parts[0])
		for i := 1; i < len(parts)-1; i++ {
			newLines = append(newLines, parts[i])
		}
		newLines = append(newLines, parts[len(parts)-1]+after)
		newLines = append(newLines, v.textLines[v.editLine+1:]...)
		v.textLines = newLines
		v.editLine += len(parts) - 1
		v.editCol = len([]rune(parts[len(parts)-1]))
	}
	v.goalCol = v.editCol
	v.afterEdit()
}

// editExternal launches an external editor ($EDITOR, falling back to
// vim/vi/nano on Unix or notepad on Windows) with the current document.
// When the editor exits, the edited content is read back and the viewer
// state is refreshed. The TUI is suspended while the editor runs.
func (v *Viewer) editExternal() {
	if v.app == nil {
		return
	}
	tmp, err := os.CreateTemp("", "jv-*.json")
	if err != nil {
		v.setToast("Cannot create temp file")
		return
	}
	name := tmp.Name()
	defer os.Remove(name)

	text := strings.Join(v.textLines, "\n")
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		v.setToast("Cannot write temp file")
		return
	}
	tmp.Close()

	editor := resolveEditor()
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		v.setToast("No editor found (set $EDITOR)")
		return
	}
	cmd := exec.Command(parts[0], append(parts[1:], name)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	ok := true
	v.app.Suspend(func() {
		if err := cmd.Run(); err != nil {
			ok = false
		}
	})
	if !ok {
		v.setToast("Editor exited with error")
		return
	}

	data, err := os.ReadFile(name)
	if err != nil {
		v.setToast("Cannot read back temp file")
		return
	}
	newText := string(data)
	if newText == text {
		v.setToast("No changes")
		return
	}

	v.textLines = splitLines(newText)
	if len(v.textLines) == 0 {
		v.textLines = []string{""}
	}
	if !v.filtered {
		v.rootLines = append([]string(nil), v.textLines...)
	}
	// The document was replaced wholesale, so fold state keyed by old
	// line numbers is stale and must be dropped.
	v.folded = make(map[int]bool)
	v.reparse()
	v.rebuildFromText()
	if v.errLine > 0 {
		v.setToast(fmt.Sprintf("Edited (invalid JSON: line %d)", v.errLine))
	} else {
		v.setToast("Edited")
	}
}

// resolveEditor picks an editor command: $EDITOR, $VISUAL, or a
// platform-appropriate fallback.
func resolveEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	for _, name := range []string{"vim", "vi", "nano"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return "vi"
}

// reformat pretty-prints the document in place (normal mode).
func (v *Viewer) reformat() {
	if v.tree == nil {
		v.setToast("cannot format: invalid JSON")
		return
	}
	v.textLines = splitLines(FormatJSONEscape(v.tree, 2, v.escape))
	if !v.filtered {
		v.rootLines = v.textLines
	}
	v.rebuildFromText()
	v.setToast("Formatted")
}

// --- Mouse ---

// MouseHandler handles wheel scrolling, clicks and scrollbar dragging.
func (v *Viewer) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, ev *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y, w, h := v.GetInnerRect()
		mx, my := ev.Position()

		switch action {
		case tview.MouseScrollUp:
			v.wheelScroll(-1)
			return true, v
		case tview.MouseScrollDown:
			v.wheelScroll(1)
			return true, v
		case tview.MouseScrollLeft:
			v.hscroll -= v.wheelStep() * 4
			return true, v
		case tview.MouseScrollRight:
			v.hscroll += v.wheelStep() * 4
			return true, v
		case tview.MouseLeftDoubleClick:
			if v.searchOpen && v.panelRect.contains(mx, my) {
				return true, v // double-click on the search panel, not a node
			}
			if my >= y && my < y+v.editorH && mx >= x+v.gutterW && mx < x+w-1 {
				idx := v.scroll + (my - y)
				if idx < len(v.visible) {
					v.cursor = idx
					col := v.colFromX(v.visible[idx], mx-x-v.gutterW+v.hscroll)
					v.enterEdit(col)
					v.selectWord()
				}
			}
			return true, v
		case tview.MouseLeftDown:
			if v.helpOpen {
				v.helpOpen = false
				return true, v
			}
			switch {
			case v.searchOpen && v.panelRect.contains(mx, my):
				v.panelClick(mx, my)
			case my == y+h-1:
				v.barClick(mx, my)
			case mx == x+w-1 && my >= y && my < y+v.editorH:
				v.scrollbarClick(my - y)
			case my >= y && my < y+v.editorH:
				if v.editing && ev.Modifiers()&tcell.ModShift != 0 && mx-x >= v.gutterW {
					v.editorDrag(mx-x, my-y)
				} else {
					v.editorClick(mx-x, my-y)
					if v.editing && mx-x >= v.gutterW {
						v.mouseSelecting = true
					}
				}
			}
			return true, v
		case tview.MouseMove:
			if v.dragThumb {
				v.thumbDrag(my - y)
				return true, v
			}
			if v.mouseSelecting && v.editing && mx >= x+v.gutterW && mx < x+w-1 {
				v.editorDrag(mx-x, my-y)
				return true, v
			}
		case tview.MouseLeftUp:
			v.dragThumb = false
			v.mouseSelecting = false
		}
		return true, v
	}
}

// wheelStep returns the scroll step for a wheel event: 1 line when events
// arrive rapidly (touchpad), 3 lines for discrete mouse-wheel notches.
func (v *Viewer) wheelStep() int {
	step := 3
	if !v.lastWheel.IsZero() && time.Since(v.lastWheel) < 90*time.Millisecond {
		step = 1
	}
	v.lastWheel = time.Now()
	return step
}

// wheelScroll moves the viewport by dir*step lines. The cursor is left
// untouched, matching editor/trackpad behavior.
func (v *Viewer) wheelScroll(dir int) {
	v.scroll += dir * v.wheelStep()
	maxScroll := len(v.visible) - v.editorH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.scroll > maxScroll {
		v.scroll = maxScroll
	}
	if v.scroll < 0 {
		v.scroll = 0
	}
}

// editorClick selects the clicked line; gutter clicks toggle folding.
// In edit mode a click positions the text cursor instead.
func (v *Viewer) editorClick(col, row int) {
	idx := v.scroll + row
	if idx >= len(v.visible) {
		return
	}
	full := v.visible[idx]
	v.focus = focusEditor
	if v.editing {
		if col >= v.gutterW {
			v.editLine = full
			v.editCol = v.colFromX(full, col-v.gutterW+v.hscroll)
			v.goalCol = v.editCol
			v.selActive = true
			v.selAnchorLine = v.editLine
			v.selAnchorCol = v.editCol
		}
		v.cursor = idx
		return
	}
	if col < v.gutterW {
		if v.lines[full].kind == kindLeaf {
			v.cursor = idx
			return
		}
		v.toggleFoldLine(full)
		return
	}
	v.cursor = idx
}

// editorDrag extends the selection to the given screen position,
// auto-scrolling when the mouse leaves the viewport.
func (v *Viewer) editorDrag(col, row int) {
	if row < 0 {
		v.scroll += row
		v.clampState()
		row = 0
	}
	if row >= v.editorH {
		v.scroll += row - v.editorH + 1
		v.clampState()
		row = v.editorH - 1
	}
	idx := v.scroll + row
	if idx < 0 {
		idx = 0
	}
	if idx >= len(v.visible) {
		idx = len(v.visible) - 1
	}
	full := v.visible[idx]
	v.editLine = full
	if col >= v.gutterW {
		v.editCol = v.colFromX(full, col-v.gutterW+v.hscroll)
		v.goalCol = v.editCol
	}
	v.cursor = idx
	v.syncEditCursor()
}

// colFromX converts a screen cell offset within a line to a rune column.
func (v *Viewer) colFromX(full, cellX int) int {
	if cellX < 0 {
		return 0
	}
	rs := []rune(v.textLines[full])
	acc, c := 0, 0
	for c < len(rs) {
		rw := cellWidth(rs[c])
		if acc+rw > cellX {
			break
		}
		acc += rw
		c++
	}
	return c
}

// scrollbarClick pages or starts a thumb drag.
func (v *Viewer) scrollbarClick(row int) {
	if v.thumbTop < 0 {
		return
	}
	if row >= v.thumbTop && row < v.thumbTop+v.thumbH {
		v.dragThumb = true
		v.grabRow = row - v.thumbTop
		return
	}
	if row < v.thumbTop {
		v.scroll -= v.editorH
	} else {
		v.scroll += v.editorH
	}
	v.clampState()
}

// thumbDrag scrolls proportionally while dragging the thumb.
func (v *Viewer) thumbDrag(row int) {
	total := len(v.visible)
	if v.editorH-v.thumbH <= 0 {
		return
	}
	v.scroll = (row - v.grabRow) * (total - v.editorH) / (v.editorH - v.thumbH)
	v.clampState()
}

// barClick handles clicks in the bottom bar.
func (v *Viewer) barClick(mx, my int) {
	for _, ar := range v.actionRect {
		if ar.r.contains(mx, my) {
			ar.act.run()
			return
		}
	}
	if v.filterRect.contains(mx, my) {
		if v.editing {
			v.exitEdit()
		}
		v.focus = focusFilter
		return
	}
	v.focus = focusEditor
}

// panelClick handles clicks inside the search panel.
func (v *Viewer) panelClick(mx, my int) {
	for _, hr := range v.hitRects {
		if !hr.r.contains(mx, my) {
			continue
		}
		switch hr.id {
		case "case":
			v.optCase = !v.optCase
			v.runSearch()
		case "word":
			v.optWord = !v.optWord
			v.runSearch()
		case "regex":
			v.optRegex = !v.optRegex
			v.runSearch()
		case "prev":
			v.stepMatch(-1)
		case "next":
			v.stepMatch(1)
		case "close":
			v.searchOpen = false
			v.focus = focusEditor
		case "input":
			v.focus = focusSearch
		}
		return
	}
	v.focus = focusSearch
}

// --- Navigation & folding ---

// moveCursor moves the cursor by delta visible lines.
func (v *Viewer) moveCursor(delta int) {
	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.visible) {
		v.cursor = len(v.visible) - 1
	}
	v.ensureCursorVisible()
}

// toggleFoldAtCursor folds/unfolds the container at the cursor line.
func (v *Viewer) toggleFoldAtCursor() {
	if v.cursor >= len(v.visible) {
		return
	}
	v.toggleFoldLine(v.visible[v.cursor])
}

// toggleFoldLine folds/unfolds the container owning the given full line.
// Fold state is keyed by the container's opening line number.
func (v *Viewer) toggleFoldLine(full int) {
	ln := &v.lines[full]
	key := -1
	switch ln.kind {
	case kindOpen:
		key = full
	case kindClose:
		key = v.containers[ln.closeID].openLine
	default:
		return
	}
	v.folded[key] = !v.folded[key]
	v.rebuildVisible()
	v.cursor = v.visiblePosOf(key)
	v.ensureCursorVisible()
}

// foldOrParent folds an expanded container at the cursor, or jumps the
// cursor to the enclosing container's opening line.
func (v *Viewer) foldOrParent() {
	full := v.visible[v.cursor]
	ln := &v.lines[full]
	if ln.kind == kindOpen && !v.folded[full] {
		v.toggleFoldLine(full)
		return
	}
	if len(ln.parents) > 0 {
		parent := v.containers[ln.parents[len(ln.parents)-1]]
		v.cursor = v.visiblePosOf(parent.openLine)
		v.ensureCursorVisible()
	}
}

// expandAtCursor unfolds a folded container at the cursor.
func (v *Viewer) expandAtCursor() {
	full := v.visible[v.cursor]
	ln := &v.lines[full]
	if ln.kind == kindOpen && v.folded[full] {
		v.toggleFoldLine(full)
	}
}

// expandAll unfolds every container.
func (v *Viewer) expandAll() {
	v.folded = make(map[int]bool)
	full := v.visible[v.cursor]
	v.rebuildVisible()
	v.cursor = v.visiblePosOf(full)
	v.ensureCursorVisible()
}

// collapseAll folds every container.
func (v *Viewer) collapseAll() {
	for _, c := range v.containers {
		if c.closeLine > c.openLine {
			v.folded[c.openLine] = true
		}
	}
	full := v.visible[v.cursor]
	v.rebuildVisible()
	v.cursor = v.visiblePosOf(full)
	v.ensureCursorVisible()
}

// --- Search ---

// openSearch shows the search panel and focuses it.
func (v *Viewer) openSearch() {
	v.searchOpen = true
	v.focus = focusSearch
	v.runSearch()
}

// runSearch recomputes all matches for the current query and options.
func (v *Viewer) runSearch() {
	v.matches = nil
	v.byLine = make(map[int][]searchMatch)
	v.matchErr = ""
	v.matchActive = false
	v.matchIdx = 0

	query := v.searchInput.String()
	if query == "" {
		return
	}

	if v.optRegex {
		pat := query
		if v.optWord {
			pat = `\b(?:` + pat + `)\b`
		}
		if !v.optCase {
			pat = `(?i)` + pat
		}
		rx, err := regexp.Compile(pat)
		if err != nil {
			v.matchErr = "bad pattern"
			return
		}
		for i := range v.lines {
			for _, loc := range rx.FindAllStringIndex(v.lines[i].plain, -1) {
				v.addMatch(i, loc[0], loc[1])
			}
		}
		return
	}

	needle := query
	if !v.optCase {
		needle = strings.ToLower(query)
	}
	if needle == "" {
		return
	}
	for i := range v.lines {
		hay := v.lines[i].plain
		if !v.optCase {
			hay = v.lines[i].lower
		}
		off := 0
		for off <= len(hay)-len(needle) {
			idx := strings.Index(hay[off:], needle)
			if idx < 0 {
				break
			}
			start := off + idx
			end := start + len(needle)
			if !v.optWord || isWordMatch(hay, start, end) {
				v.addMatch(i, start, end)
			}
			off = start + 1
		}
	}
}

// addMatch records one match.
func (v *Viewer) addMatch(line, start, end int) {
	m := searchMatch{line: line, start: start, end: end}
	v.matches = append(v.matches, m)
	v.byLine[line] = append(v.byLine[line], m)
}

// isWordMatch reports whether the byte range in s is bounded by
// non-word characters.
func isWordMatch(s string, start, end int) bool {
	if start > 0 && isWordByte(s[start-1]) {
		return false
	}
	if end < len(s) && isWordByte(s[end]) {
		return false
	}
	return true
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// stepMatch jumps to the next/previous match relative to the cursor.
func (v *Viewer) stepMatch(dir int) {
	if len(v.matches) == 0 {
		return
	}
	curFull := v.visible[v.cursor]
	var idx int
	if dir > 0 {
		idx = sort.Search(len(v.matches), func(i int) bool {
			return v.matches[i].line > curFull ||
				(v.matches[i].line == curFull && v.matchActive && i > v.matchIdx)
		})
		if !v.matchActive {
			idx = sort.Search(len(v.matches), func(i int) bool { return v.matches[i].line >= curFull })
		}
		if idx >= len(v.matches) {
			idx = 0
		}
	} else {
		idx = sort.Search(len(v.matches), func(i int) bool { return v.matches[i].line >= curFull }) - 1
		if v.matchActive && v.matchIdx < len(v.matches) && v.matches[v.matchIdx].line == curFull {
			idx = v.matchIdx - 1
		}
		if idx < 0 {
			idx = len(v.matches) - 1
		}
	}
	v.jumpToMatch(idx)
}

// jumpToMatch unfolds and centers the given match.
func (v *Viewer) jumpToMatch(idx int) {
	if idx < 0 || idx >= len(v.matches) {
		return
	}
	v.matchIdx = idx
	v.matchActive = true
	m := v.matches[idx]

	// Unfold ancestors so the match line becomes visible.
	needRebuild := false
	for _, id := range v.lines[m.line].parents {
		key := v.containers[id].openLine
		if v.folded[key] {
			delete(v.folded, key)
			needRebuild = true
		}
	}
	if needRebuild {
		v.rebuildVisible()
	}
	v.cursor = v.visiblePosOf(m.line)
	v.scroll = v.cursor - v.editorH/2
	v.ensureCursorVisible()
	v.clampState()

	// Bring the match column into view.
	colStart := uniseg.StringWidth(v.lines[m.line].plain[:m.start])
	contentW := v.contentWidth()
	if colStart < v.hscroll || colStart > v.hscroll+contentW-6 {
		v.hscroll = colStart - contentW/3
		if v.hscroll < 0 {
			v.hscroll = 0
		}
	}
}

// --- Filter bar ---

// applyFilter evaluates the filter expression and swaps the document.
func (v *Viewer) applyFilter() {
	expr := strings.TrimSpace(v.filterInput.String())
	if expr == "" {
		v.filterErr = ""
		if v.filtered {
			v.textLines = v.rootLines
			v.filtered = false
			// Fold keys reference lines of the filtered document, which no
			// longer exist; drop them for the restored document.
			v.folded = make(map[int]bool)
			v.reparse()
			v.rebuildFromText()
			v.setToast("Filter cleared")
		}
		v.focus = focusEditor
		return
	}
	if v.rootTree == nil {
		v.filterErr = "document is not valid JSON"
		return
	}
	res, err := evalFilter(v.rootTree, expr)
	if err != nil {
		v.filterErr = err.Error()
		return
	}
	v.filterErr = ""
	if !v.filtered {
		v.rootLines = append([]string(nil), v.textLines...)
	}
	v.tree = res
	v.textLines = splitLines(FormatJSONEscape(res, 2, v.escape))
	v.filtered = true
	v.errLine, v.errMsg = 0, ""
	// Fold keys reference the unfiltered document; drop them so no stale
	// range is collapsed in the filtered view.
	v.folded = make(map[int]bool)
	v.rebuildFromText()
	summary := TypeString(res)
	if n := CountChildren(res); n > 0 {
		summary = fmt.Sprintf("%s · %d items", summary, n)
	}
	v.setToast("→ " + summary)
	v.focus = focusEditor
}

// --- Actions ---

// toggleEscape switches between UTF-8 and \uXXXX string display.
func (v *Viewer) toggleEscape() {
	if v.tree == nil {
		v.setToast("invalid JSON, nothing to escape")
		return
	}
	v.escape = !v.escape
	v.textLines = splitLines(FormatJSONEscape(v.tree, 2, v.escape))
	if !v.filtered {
		v.rootLines = v.textLines
	}
	v.rebuildFromText()
	if v.escape {
		v.setToast(`\uXXXX escape on`)
	} else {
		v.setToast(`\uXXXX escape off`)
	}
}

// copyFormatted copies the full document text as displayed.
func (v *Viewer) copyFormatted() {
	v.copyText(strings.Join(v.textLines, "\n"), "formatted JSON")
}

// copyMinified copies the document as a single minified line.
func (v *Viewer) copyMinified() {
	if v.tree == nil {
		v.setToast("invalid JSON, cannot minify")
		return
	}
	v.copyText(CompactJSON(v.tree, false), "minified JSON")
}

// copyMinifiedEscaped copies the document minified with all non-ASCII
// characters escaped as \uXXXX sequences.
func (v *Viewer) copyMinifiedEscaped() {
	if v.tree == nil {
		v.setToast("invalid JSON, cannot minify")
		return
	}
	v.copyText(CompactJSON(v.tree, true), `minified \uXXXX JSON`)
}

// copyValue copies the JSON value at the cursor line.
func (v *Viewer) copyValue() {
	ln := &v.lines[v.visible[v.cursor]]
	if v.tree == nil {
		v.copyText(strings.TrimSpace(ln.plain), "line")
		return
	}
	val, err := evalFilter(v.tree, ln.path)
	if err != nil {
		v.copyText(strings.TrimSpace(ln.plain), "line")
		return
	}
	if s, ok := val.(string); ok {
		v.copyText(s, "string value")
		return
	}
	v.copyText(CompactJSON(val, false), "value")
}

// copyPath copies the JSON path of the cursor line.
func (v *Viewer) copyPath() {
	path := v.lines[v.visible[v.cursor]].path
	if path == "" {
		path = "this"
	}
	v.copyText(path, "path")
}

// copyText writes to the clipboard and shows a toast.
func (v *Viewer) copyText(text, what string) {
	if err := clipboardWrite(text); err != nil {
		v.setToast("Copy failed: " + err.Error())
		return
	}
	size := len(text)
	sizeStr := fmt.Sprintf("%d B", size)
	if size >= 1024 {
		sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	v.setToast(fmt.Sprintf("Copied %s (%s)", what, sizeStr))
}

// setToast shows a transient message in the bottom bar.
func (v *Viewer) setToast(msg string) {
	v.toast = msg
	v.toastAt = time.Now()
	if v.app != nil {
		time.AfterFunc(2100*time.Millisecond, func() {
			v.app.QueueUpdateDraw(func() {})
		})
	}
}
