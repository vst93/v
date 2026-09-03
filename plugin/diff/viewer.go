package plugin_diff

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"v/internal/theme"
)

// DiffViewer holds the state for the interactive side-by-side diff TUI.
type DiffViewer struct {
	lines     []DiffLine
	leftFile  string
	rightFile string

	leftView   *tview.TextView
	rightView  *tview.TextView
	leftTitle  *tview.TextView
	rightTitle *tview.TextView
	statusBar  *tview.TextView
	helpBar    *tview.TextView
	searchBar  *tview.InputField
	app        *tview.Application
	bottomBar  *tview.Flex

	currentIdx int // current highlighted line index
	searchTerm string
	searchHits []int // indices of matching lines
	searchPos  int   // current position in searchHits

	// cached formatted content
	leftContent  string
	rightContent string
	panelWidth   int // render width for full-line backgrounds (0 = not yet known)

	rootLayout *tview.Flex
	onEdit     func() // invoked on 'e' to return to edit mode (nil = disabled)
}

// NewDiffViewer creates a new DiffViewer with the given diff lines.
func NewDiffViewer(lines []DiffLine, leftFile, rightFile string) *DiffViewer {
	return &DiffViewer{
		lines:     lines,
		leftFile:  leftFile,
		rightFile: rightFile,
	}
}

// build constructs the viewer's widgets and layout. If app is non-nil it is
// reused (for embedding in an external application such as the paste-mode
// viewer); otherwise a new application is created. The input capture is
// installed on the app and the configured app is returned.
func (dv *DiffViewer) build(app *tview.Application) *tview.Application {
	if app == nil {
		app = tview.NewApplication()
	}
	dv.app = app

	dv.leftView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false).
		SetRegions(true)

	dv.rightView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false).
		SetRegions(true)

	dv.leftTitle = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	dv.rightTitle = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	dv.updateTitles()

	dv.statusBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)

	dv.helpBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	dv.helpBar.SetText(helpText([]helpItem{
		{"↑↓/jk", "nav"}, {"n/N", "diff"}, {"/", "search"},
		{"c", "changes"}, {"a", "all"}, {"e", "edit"}, {"q", "quit"},
	}))

	dv.searchBar = tview.NewInputField().
		SetLabel(fmt.Sprintf("[%s]Search: [-:-:-]", theme.Hex(theme.Current().Warn))).
		SetFieldBackgroundColor(theme.Current().FieldBg).
		SetFieldTextColor(theme.Current().FieldFg).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				term := dv.searchBar.GetText()
				if term != "" {
					dv.searchTerm = term
					dv.doSearch()
				}
				dv.exitSearch()
			} else if key == tcell.KeyEscape {
				dv.exitSearch()
			}
		})

	dv.renderContent()
	dv.renderToViews()

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(dv.leftTitle, 1, 0, false).
		AddItem(dv.leftView, 0, 1, true)

	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(dv.rightTitle, 1, 0, false).
		AddItem(dv.rightView, 0, 1, true)

	mainRow := tview.NewFlex().
		AddItem(leftPanel, 0, 1, false).
		AddItem(vSeparator(), 1, 0, false).
		AddItem(rightPanel, 0, 1, false)

	// Bottom area: status bar + (help bar OR search bar)
	dv.bottomBar = tview.NewFlex().SetDirection(tview.FlexRow)
	dv.bottomBar.AddItem(dv.statusBar, 1, 0, false)
	dv.bottomBar.AddItem(dv.helpBar, 1, 0, false)
	// searchBar is added/removed dynamically

	dv.rootLayout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainRow, 0, 1, true).
		AddItem(dv.bottomBar, 2, 0, false)

	dv.currentIdx = dv.firstDiffIndex()
	dv.updateStatusBar()

	// Input handler shared across all focusable widgets
	inputHandler := func(event *tcell.EventKey) *tcell.EventKey {
		if dv.app.GetFocus() == dv.searchBar {
			return dv.handleSearchInput(event)
		}

		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				dv.app.Stop()
				return nil
			case 'e':
				if dv.onEdit != nil {
					dv.onEdit()
					return nil
				}
			case 'j':
				dv.cursorDown()
				return nil
			case 'k':
				dv.cursorUp()
				return nil
			case 'n':
				if dv.searchTerm != "" {
					dv.nextSearchHit()
				} else {
					dv.nextDiff()
				}
				return nil
			case 'N':
				if dv.searchTerm != "" {
					dv.prevSearchHit()
				} else {
					dv.prevDiff()
				}
				return nil
			case 'a':
				dv.showAll()
				return nil
			case 'c':
				dv.showChangesOnly()
				return nil
			case '/':
				dv.startSearch()
				return nil
			}
		case tcell.KeyDown:
			dv.cursorDown()
			return nil
		case tcell.KeyUp:
			dv.cursorUp()
			return nil
		}
		return event
	}

	// Set input capture on the app level so it works regardless of focus
	dv.app.SetInputCapture(inputHandler)

	// Re-render with full-width backgrounds whenever the terminal resizes
	// (and on the first draw, once the real screen width is known).
	dv.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, _ := screen.Size()
		pw := (w - 1) / 2
		if pw < 1 {
			pw = 1
		}
		if dv.panelWidth != pw {
			dv.panelWidth = pw
			dv.renderContent()
			dv.renderToViews()
		}
		return false
	})

	return app
}

// Run launches the interactive TUI as a standalone application.
func (dv *DiffViewer) Run() error {
	theme.Init(true) // paste mode reuses this app via build(nil), already themed
	theme.ApplyTView()
	app := dv.build(nil)
	return app.SetRoot(dv.rootLayout, true).EnableMouse(true).Run()
}

// Root returns the root layout primitive, for embedding in an external app.
func (dv *DiffViewer) Root() tview.Primitive {
	return dv.rootLayout
}

// SetOnEdit registers a callback invoked when the user presses 'e' to return
// to the input/edit mode. When nil (the default), 'e' is a no-op.
func (dv *DiffViewer) SetOnEdit(fn func()) {
	dv.onEdit = fn
}

// --- Rendering ---

// Diff color palette (dark terminal theme, VS Code-style): soft tinted
// backgrounds fill the whole row for changed lines, a stronger background
// highlights the specific changed words, and blank regions show a hatch fill.
const (
	colorEqualText = "#cccccc"
	colorLineNum   = "#555555"
	colorLineNumHi = "#888888" // line number on a tinted background

	colorDelBg     = "#4a2222" // soft dark red: deletions / left side of a change
	colorDelText   = "#e0b0b0"
	colorDelWordBg = "#7a3030" // stronger red: deleted words
	colorAddBg     = "#224a22" // soft dark green: additions / right side of a change
	colorAddText   = "#b0e0b0"
	colorAddWordBg = "#307a30" // stronger green: added words

	colorHatch = "#3a3a3a" // neutral dim hatch for blank regions
	hatchChar  = "░"
)

// Bottom-bar styling (jv-inspired): key "pills" + dim descriptions, with
// status segments joined by " · ".
const (
	colorPillBg  = "#008080" // teal pill background
	colorPillFg  = "#000000" // black bold key on the pill
	colorBarDim  = "#888888" // dim descriptions / secondary status text
	colorBarMain = "#aaaaaa" // primary status text
	colorBarAcc  = "#ffaa00" // accent (search)
)

// helpItem pairs a key (shown as a pill) with a short description.
type helpItem struct{ key, desc string }

// pill renders a key as a filled "pill": black bold text on a teal background,
// space-padded so it reads as a button (jv-style action chip).
func pill(key string) string {
	return fmt.Sprintf("[%s:%s:b] %s [-:-:-]", colorPillFg, colorPillBg, tview.Escape(key))
}

// helpText builds a help bar from key/description pairs: each key as a pill
// followed by a dim description, joined with a double space.
func helpText(items []helpItem) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = pill(it.key) + " [" + colorBarDim + "]" + it.desc + "[-:-:-]"
	}
	return strings.Join(parts, "  ")
}

// vSeparator returns a 1-column vertical divider widget: a dim '│' column
// that visually separates the left and right panels.
func vSeparator() *tview.TextView {
	s := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	s.SetText("[" + theme.Hex(theme.Current().TextDim) + "]" + strings.Repeat("│\n", 256) + "[-]")
	return s
}

func (dv *DiffViewer) renderContent() {
	dv.leftContent, dv.rightContent = dv.renderLines(dv.lines)
}

// renderLines builds the side-by-side text for both panels. Changed lines get
// a soft full-width background tint; blank regions get a hatch fill; modified
// lines also get inline word-level highlights with a stronger background. Each
// line is padded to dv.panelWidth so the background/hatch fills the panel.
func (dv *DiffViewer) renderLines(lines []DiffLine) (string, string) {
	var leftSB, rightSB strings.Builder
	lineNumWidth := dv.lineNumWidth()
	pw := dv.panelWidth

	for i, line := range lines {
		leftRegion := fmt.Sprintf("[\"L%d\"]", i)
		rightRegion := fmt.Sprintf("[\"R%d\"]", i)
		end := `[""]`

		switch line.Op {
		case OpEqual:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[%s]%*d[-] [%s]%s[-]", colorLineNum, lineNumWidth, line.LeftNum, colorEqualText, tview.Escape(line.Left)))
			leftSB.WriteString(end + "\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[%s]%*d[-] [%s]%s[-]", colorLineNum, lineNumWidth, line.RightNum, colorEqualText, tview.Escape(line.Right)))
			rightSB.WriteString(end + "\n")

		case opChange:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[%s:%s]%*d [-:-:-]", colorLineNumHi, colorDelBg, lineNumWidth, line.LeftNum))
			leftSB.WriteString(renderInlineDiff(line.Left, line.Right, colorDelText, colorDelBg, colorDelWordBg, true))
			leftSB.WriteString(bgPad(lineNumWidth+1+visibleWidth(line.Left), pw, colorDelBg))
			leftSB.WriteString(end + "\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[%s:%s]%*d [-:-:-]", colorLineNumHi, colorAddBg, lineNumWidth, line.RightNum))
			rightSB.WriteString(renderInlineDiff(line.Left, line.Right, colorAddText, colorAddBg, colorAddWordBg, false))
			rightSB.WriteString(bgPad(lineNumWidth+1+visibleWidth(line.Right), pw, colorAddBg))
			rightSB.WriteString(end + "\n")

		case OpDel:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[%s:%s]%*d [%s:%s]%s[-:-:-]",
				colorLineNumHi, colorDelBg, lineNumWidth, line.LeftNum,
				colorDelText, colorDelBg, tview.Escape(line.Left)))
			leftSB.WriteString(bgPad(lineNumWidth+1+visibleWidth(line.Left), pw, colorDelBg))
			leftSB.WriteString(end + "\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(hatchFill(pw))
			rightSB.WriteString(end + "\n")

		case OpAdd:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(hatchFill(pw))
			leftSB.WriteString(end + "\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[%s:%s]%*d [%s:%s]%s[-:-:-]",
				colorLineNumHi, colorAddBg, lineNumWidth, line.RightNum,
				colorAddText, colorAddBg, tview.Escape(line.Right)))
			rightSB.WriteString(bgPad(lineNumWidth+1+visibleWidth(line.Right), pw, colorAddBg))
			rightSB.WriteString(end + "\n")
		}
	}

	return leftSB.String(), rightSB.String()
}

// renderInlineDiff produces inline word-level highlights for a changed line.
// baseText/baseBg apply to unchanged words; wordBg applies to the differing
// words (deleted words on the left, added words on the right). Each segment
// fully specifies its colours so the row background is preserved between
// highlighted words.
func renderInlineDiff(left, right, baseText, baseBg, wordBg string, isLeft bool) string {
	parts := WordDiff(left, right)
	var sb strings.Builder
	for _, part := range parts {
		escaped := tview.Escape(part.Text)
		switch part.DiffType {
		case OpEqual:
			sb.WriteString(fmt.Sprintf("[%s:%s]%s[-:-:-]", baseText, baseBg, escaped))
		case OpDel:
			if isLeft {
				sb.WriteString(fmt.Sprintf("[%s:%s]%s[-:-:-]", baseText, wordBg, escaped))
			}
		case OpAdd:
			if !isLeft {
				sb.WriteString(fmt.Sprintf("[%s:%s]%s[-:-:-]", baseText, wordBg, escaped))
			}
		}
	}
	return sb.String()
}

// visibleWidth returns the display-cell count of s (1 cell per rune; CJK wide
// chars are undercounted, consistent with the rest of the viewer).
func visibleWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// bgPad returns a run of spaces coloured with bg that fills from contentW to
// panelWidth, so the row background reaches the panel edge. Returns "" when
// there is nothing to pad.
func bgPad(contentW, panelW int, bg string) string {
	if bg == "" || panelW <= 0 {
		return ""
	}
	pad := panelW - contentW
	if pad <= 0 {
		return ""
	}
	return fmt.Sprintf("[-:%s]%s[-:-:-]", bg, strings.Repeat(" ", pad))
}

// hatchFill returns a hatch pattern filling panelWidth cells, used for the
// blank side of a pure insertion/deletion row.
func hatchFill(panelW int) string {
	if panelW <= 0 {
		return ""
	}
	return fmt.Sprintf("[%s]%s[-:-:-]", colorHatch, strings.Repeat(hatchChar, panelW))
}

func (dv *DiffViewer) renderToViews() {
	leftRow, _ := dv.leftView.GetScrollOffset()
	rightRow, _ := dv.rightView.GetScrollOffset()

	dv.leftView.SetText(dv.leftContent)
	dv.rightView.SetText(dv.rightContent)

	dv.leftView.ScrollTo(leftRow, 0)
	dv.rightView.ScrollTo(rightRow, 0)
}

func (dv *DiffViewer) lineNumWidth() int {
	maxNum := 0
	for _, line := range dv.lines {
		if line.LeftNum > maxNum {
			maxNum = line.LeftNum
		}
		if line.RightNum > maxNum {
			maxNum = line.RightNum
		}
	}
	w := len(fmt.Sprintf("%d", maxNum))
	if w < 3 {
		w = 3
	}
	return w
}

// --- Navigation ---

func (dv *DiffViewer) cursorDown() {
	if dv.currentIdx < len(dv.lines)-1 {
		dv.currentIdx++
		dv.refreshUI()
	}
}

func (dv *DiffViewer) cursorUp() {
	if dv.currentIdx > 0 {
		dv.currentIdx--
		dv.refreshUI()
	}
}

func (dv *DiffViewer) nextDiff() {
	for i := dv.currentIdx + 1; i < len(dv.lines); i++ {
		if dv.lines[i].Op != OpEqual {
			dv.currentIdx = i
			dv.refreshUI()
			return
		}
	}
}

func (dv *DiffViewer) prevDiff() {
	for i := dv.currentIdx - 1; i >= 0; i-- {
		if dv.lines[i].Op != OpEqual {
			dv.currentIdx = i
			dv.refreshUI()
			return
		}
	}
}

func (dv *DiffViewer) firstDiffIndex() int {
	for i, line := range dv.lines {
		if line.Op != OpEqual {
			return i
		}
	}
	return 0
}

// refreshUI updates the highlight, scroll position, and status bar,
// then triggers a redraw. Call this after any state change.
func (dv *DiffViewer) refreshUI() {
	dv.leftView.Highlight(fmt.Sprintf("L%d", dv.currentIdx))
	dv.rightView.Highlight(fmt.Sprintf("R%d", dv.currentIdx))

	_, _, _, height := dv.leftView.GetInnerRect()
	scrollRow, _ := dv.leftView.GetScrollOffset()

	lineY := dv.currentIdx
	if lineY < scrollRow {
		dv.leftView.ScrollTo(lineY, 0)
		dv.rightView.ScrollTo(lineY, 0)
	} else if height > 0 && lineY >= scrollRow+height-1 {
		newScroll := lineY - height + 2
		if newScroll < 0 {
			newScroll = 0
		}
		dv.leftView.ScrollTo(newScroll, 0)
		dv.rightView.ScrollTo(newScroll, 0)
	}
	dv.updateStatusBar()
}

// --- Search ---

func (dv *DiffViewer) startSearch() {
	dv.searchBar.SetText("")
	// Replace help bar with search bar in the bottom layout
	dv.bottomBar.RemoveItem(dv.helpBar)
	dv.bottomBar.AddItem(dv.searchBar, 1, 0, true)
	dv.app.SetFocus(dv.searchBar)
}

func (dv *DiffViewer) handleSearchInput(event *tcell.EventKey) *tcell.EventKey {
	// Let Enter and Escape pass through to the InputField's DoneFunc
	if event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEscape {
		return event
	}
	return event
}

func (dv *DiffViewer) exitSearch() {
	// Replace search bar with help bar
	dv.bottomBar.RemoveItem(dv.searchBar)
	dv.bottomBar.AddItem(dv.helpBar, 1, 0, false)
	dv.app.SetFocus(dv.leftView)
}

func (dv *DiffViewer) doSearch() {
	dv.searchHits = nil
	term := strings.ToLower(dv.searchTerm)
	for i, line := range dv.lines {
		if strings.Contains(strings.ToLower(line.Left), term) ||
			strings.Contains(strings.ToLower(line.Right), term) {
			dv.searchHits = append(dv.searchHits, i)
		}
	}
	dv.searchPos = 0
	if len(dv.searchHits) > 0 {
		dv.currentIdx = dv.searchHits[0]
		dv.refreshUI()
	}
	dv.updateStatusBar()
}

func (dv *DiffViewer) nextSearchHit() {
	if len(dv.searchHits) == 0 {
		return
	}
	dv.searchPos = (dv.searchPos + 1) % len(dv.searchHits)
	dv.currentIdx = dv.searchHits[dv.searchPos]
	dv.refreshUI()
}

func (dv *DiffViewer) prevSearchHit() {
	if len(dv.searchHits) == 0 {
		return
	}
	dv.searchPos = (dv.searchPos - 1 + len(dv.searchHits)) % len(dv.searchHits)
	dv.currentIdx = dv.searchHits[dv.searchPos]
	dv.refreshUI()
}

// --- Filter modes ---

func (dv *DiffViewer) showAll() {
	dv.renderContent()
	dv.renderToViews()
	dv.currentIdx = dv.firstDiffIndex()
	dv.refreshUI()
}

func (dv *DiffViewer) showChangesOnly() {
	var changed []DiffLine
	for _, line := range dv.lines {
		if line.Op != OpEqual {
			changed = append(changed, line)
		}
	}
	dv.leftContent, dv.rightContent = dv.renderLines(changed)
	dv.renderToViews()
	dv.currentIdx = 0
	dv.refreshUI()
}

// --- Status bar ---

func (dv *DiffViewer) updateTitles() {
	leftName := dv.leftFile
	rightName := dv.rightFile
	if leftName == "" {
		leftName = "<left>"
	}
	if rightName == "" {
		rightName = "<right>"
	}

	adds, dels, changes := 0, 0, 0
	for _, line := range dv.lines {
		switch line.Op {
		case OpAdd:
			adds++
		case OpDel:
			dels++
		case opChange:
			changes++
		}
	}

	p := theme.Current()
	dv.leftTitle.SetText(fmt.Sprintf("[%s::b]◀ %s[-:-:-]  [%s]- %d del, ~ %d chg[-]",
		theme.Hex(p.Error), leftName, theme.Hex(p.TextDim), dels, changes))
	dv.rightTitle.SetText(fmt.Sprintf("[%s::b]%s ▶[-:-:-]  [%s]+ %d add, ~ %d chg[-]",
		theme.Hex(p.Success), rightName, theme.Hex(p.TextDim), adds, changes))
}

func (dv *DiffViewer) updateStatusBar() {
	total := len(dv.lines)
	// When in changes-only mode, count from the rendered content
	renderedTotal := 0
	for _, line := range dv.lines {
		if line.Op != OpEqual {
			renderedTotal++
		}
	}
	// If currently showing only changes, use that count
	displayTotal := total
	if len(dv.searchHits) == 0 && renderedTotal > 0 {
		// Check if we're in changes-only mode by seeing if leftContent
		// has fewer lines than total
		leftLines := strings.Count(dv.leftContent, "\n")
		if leftLines < total {
			displayTotal = leftLines
		}
	}

	diffCount := 0
	for _, line := range dv.lines {
		if line.Op != OpEqual {
			diffCount++
		}
	}

	opWord, opColor := "equal", colorBarMain
	if dv.currentIdx < len(dv.lines) {
		switch dv.lines[dv.currentIdx].Op {
		case OpAdd:
			opWord, opColor = "added", "#44ff44"
		case OpDel:
			opWord, opColor = "deleted", "#ff4444"
		case opChange:
			opWord, opColor = "changed", "#ffaa44"
		}
	}

	searchInfo := ""
	if len(dv.searchHits) > 0 {
		searchInfo = fmt.Sprintf("  [%s]· search '%s' [%d/%d][-]", colorBarAcc, dv.searchTerm, dv.searchPos+1, len(dv.searchHits))
	}

	dv.statusBar.SetText(fmt.Sprintf(" [%s]Ln %d/%d · [%s]%s[%s] · %d diffs[-]%s",
		colorBarMain, dv.currentIdx+1, displayTotal, opColor, opWord, colorBarMain, diffCount, searchInfo))
}
