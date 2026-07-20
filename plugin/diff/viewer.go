package plugin_diff

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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

	currentIdx int    // current highlighted line index
	searchTerm string
	searchHits []int  // indices of matching lines
	searchPos  int    // current position in searchHits

	// cached formatted content
	leftContent  string
	rightContent string

	rootLayout *tview.Flex
}

// NewDiffViewer creates a new DiffViewer with the given diff lines.
func NewDiffViewer(lines []DiffLine, leftFile, rightFile string) *DiffViewer {
	return &DiffViewer{
		lines:     lines,
		leftFile:  leftFile,
		rightFile: rightFile,
	}
}

// Run launches the interactive TUI.
func (dv *DiffViewer) Run() error {
	dv.app = tview.NewApplication()

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
	dv.helpBar.SetText("[#888888]↑↓/jk: navigate  n/N: next/prev diff  /: search  c: changes only  a: all  q: quit[-:-:-]")

	dv.searchBar = tview.NewInputField().
		SetLabel("[#ffaa00]Search: [-:-:-]").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
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

	return dv.app.SetRoot(dv.rootLayout, true).EnableMouse(true).Run()
}

// --- Rendering ---

func (dv *DiffViewer) renderContent() {
	dv.leftContent, dv.rightContent = dv.renderLines(dv.lines)
}

func (dv *DiffViewer) renderLines(lines []DiffLine) (string, string) {
	var leftSB, rightSB strings.Builder
	lineNumWidth := dv.lineNumWidth()

	for i, line := range lines {
		leftRegion := fmt.Sprintf("[\"L%d\"]", i)
		rightRegion := fmt.Sprintf("[\"R%d\"]", i)
		leftEnd := `[""]`
		rightEnd := `[""]`

		switch line.Op {
		case OpEqual:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[#555555]%*d[-] [#cccccc]%s[-]", lineNumWidth, line.LeftNum, tview.Escape(line.Left)))
			leftSB.WriteString(leftEnd)
			leftSB.WriteString("\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[#555555]%*d[-] [#cccccc]%s[-]", lineNumWidth, line.RightNum, tview.Escape(line.Right)))
			rightSB.WriteString(rightEnd)
			rightSB.WriteString("\n")

		case opChange:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[#ffaa44]%*d[-] ", lineNumWidth, line.LeftNum))
			leftSB.WriteString(renderInlineDiff(line.Left, line.Right, true))
			leftSB.WriteString(leftEnd)
			leftSB.WriteString("\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[#ffaa44]%*d[-] ", lineNumWidth, line.RightNum))
			rightSB.WriteString(renderInlineDiff(line.Left, line.Right, false))
			rightSB.WriteString(rightEnd)
			rightSB.WriteString("\n")

		case OpDel:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[#ff4444]%*d[-] [#ff8888::d]%s[-:-:-]", lineNumWidth, line.LeftNum, tview.Escape(line.Left)))
			leftSB.WriteString(leftEnd)
			leftSB.WriteString("\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[#333333]%*s [-]", lineNumWidth, ""))
			rightSB.WriteString(rightEnd)
			rightSB.WriteString("\n")

		case OpAdd:
			leftSB.WriteString(leftRegion)
			leftSB.WriteString(fmt.Sprintf("[#333333]%*s [-]", lineNumWidth, ""))
			leftSB.WriteString(leftEnd)
			leftSB.WriteString("\n")

			rightSB.WriteString(rightRegion)
			rightSB.WriteString(fmt.Sprintf("[#44ff44]%*d[-] [#88ff88]%s[-]", lineNumWidth, line.RightNum, tview.Escape(line.Right)))
			rightSB.WriteString(rightEnd)
			rightSB.WriteString("\n")
		}
	}

	return leftSB.String(), rightSB.String()
}

func renderInlineDiff(left, right string, isLeft bool) string {
	parts := WordDiff(left, right)
	var sb strings.Builder
	for _, part := range parts {
		escaped := tview.Escape(part.Text)
		switch part.DiffType {
		case OpEqual:
			sb.WriteString(fmt.Sprintf("[#ffcc88]%s[-]", escaped))
		case OpDel:
			if isLeft {
				sb.WriteString(fmt.Sprintf("[#ff4444::d]%s[-:-:-]", escaped))
			}
		case OpAdd:
			if !isLeft {
				sb.WriteString(fmt.Sprintf("[#44ff44::b]%s[-:-:-]", escaped))
			}
		}
	}
	return sb.String()
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

	dv.leftTitle.SetText(fmt.Sprintf("[#ff4444::b]◀ %s[-:-:-]  [#555555]- %d del, ~ %d chg[-]", leftName, dels, changes))
	dv.rightTitle.SetText(fmt.Sprintf("[#44ff44::b]%s ▶[-:-:-]  [#555555]+ %d add, ~ %d chg[-]", rightName, adds, changes))
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

	searchInfo := ""
	if len(dv.searchHits) > 0 {
		searchInfo = fmt.Sprintf("  |  [#ffaa00]Search '%s'[-] [#555555][%d/%d hits][-]", dv.searchTerm, dv.searchPos+1, len(dv.searchHits))
	}

	curOp := "equal"
	if dv.currentIdx < len(dv.lines) {
		switch dv.lines[dv.currentIdx].Op {
		case OpAdd:
			curOp = "[#44ff44]added[-]"
		case OpDel:
			curOp = "[#ff4444]deleted[-]"
		case opChange:
			curOp = "[#ffaa44]changed[-]"
		}
	}

	dv.statusBar.SetText(fmt.Sprintf(" [#aaaaaa]Line %d/%d[-]  [#555555]Type: %s[-]  [#555555]Diffs: %d[-]%s",
		dv.currentIdx+1, displayTotal, curOp, diffCount, searchInfo))
}
