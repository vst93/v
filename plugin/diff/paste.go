package plugin_diff

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PasteViewer is the input-mode TUI: the user pastes left/right text into two
// editable panels and presses Ctrl+D to compute the diff. The clipboard is
// never read programmatically — the user pastes manually via the terminal
// (Cmd/Ctrl+V, middle-click, bracketed paste).
type PasteViewer struct {
	app        *tview.Application
	leftArea   *tview.TextArea
	rightArea  *tview.TextArea
	leftTitle  *tview.TextView
	rightTitle *tview.TextView
	statusBar  *tview.TextView
	helpBar    *tview.TextView
	inputRoot  *tview.Flex
}

// NewPasteViewer creates a PasteViewer with empty input panels.
func NewPasteViewer() *PasteViewer {
	return &PasteViewer{}
}

// Run launches the paste-mode TUI as a standalone application.
func (pv *PasteViewer) Run() error {
	pv.app = tview.NewApplication()
	pv.buildInput()
	pv.app.SetInputCapture(pv.inputCapture)
	pv.app.SetFocus(pv.leftArea)
	return pv.app.SetRoot(pv.inputRoot, true).EnableMouse(true).Run()
}

// buildInput constructs the two TextAreas, title/status/help bars, and root
// layout. The frame deliberately mirrors the diff viewer (title row above
// borderless content, then a status row and a help row) so the paste<->diff
// transition reads as the same screen transforming rather than a jump to a
// different layout.
func (pv *PasteViewer) buildInput() {
	pv.leftArea = tview.NewTextArea().
		SetPlaceholder("Paste left-side text here (Cmd/Ctrl+V)...")
	pv.rightArea = tview.NewTextArea().
		SetPlaceholder("Paste right-side text here (Cmd/Ctrl+V)...")

	pv.leftTitle = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	pv.rightTitle = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)

	pv.statusBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	pv.helpBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	pv.helpBar.SetText(helpText([]helpItem{
		{"Tab", "switch"}, {"Ctrl+D", "diff"}, {"Ctrl+C", "quit"},
	}))

	// Keep titles and the status bar in sync as the user types or pastes.
	refresh := func() { pv.refreshInputStatus() }
	pv.leftArea.SetChangedFunc(refresh)
	pv.rightArea.SetChangedFunc(refresh)

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pv.leftTitle, 1, 0, false).
		AddItem(pv.leftArea, 0, 1, true)
	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pv.rightTitle, 1, 0, false).
		AddItem(pv.rightArea, 0, 1, true)

	mainRow := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(vSeparator(), 1, 0, false).
		AddItem(rightPanel, 0, 1, true)

	pv.inputRoot = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainRow, 0, 1, true).
		AddItem(pv.statusBar, 1, 0, false).
		AddItem(pv.helpBar, 1, 0, false)

	pv.refreshInputStatus()
}

// refreshInputStatus updates the title bars (per-side line counts) and the
// status bar (jv-style " · "-joined segments). Called on every keystroke/paste.
func (pv *PasteViewer) refreshInputStatus() {
	lLines := lineCount(pv.leftArea.GetText())
	rLines := lineCount(pv.rightArea.GetText())

	pv.leftTitle.SetText(fmt.Sprintf("[#ff4444::b]◀ Left (paste)[-:-:-]  [#555555]L: %d lines[-]", lLines))
	pv.rightTitle.SetText(fmt.Sprintf("[#44ff44::b]Right (paste) ▶[-:-:-]  [#555555]R: %d lines[-]", rLines))

	pv.statusBar.SetText(fmt.Sprintf(" [%s]Paste · L %d ln · R %d ln[-]", colorBarMain, lLines, rLines))
}

// lineCount returns the number of lines in s (0 for empty, newline+1 otherwise).
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// inputCapture handles Tab (switch panel), Ctrl+D (compute diff), and Ctrl+C
// (quit). All other keys pass through to the focused TextArea so the user can
// type and paste freely. 'q' cannot be used to quit here because it would be
// inserted into the text area.
func (pv *PasteViewer) inputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		if pv.app.GetFocus() == pv.leftArea {
			pv.app.SetFocus(pv.rightArea)
		} else {
			pv.app.SetFocus(pv.leftArea)
		}
		return nil
	case tcell.KeyCtrlD:
		pv.computeDiff()
		return nil
	case tcell.KeyCtrlC:
		pv.app.Stop()
		return nil
	}
	return event
}

// computeDiff reads the pasted text from both panels, computes the diff, and
// swaps the root layout to the diff viewer. Pressing 'e' in the diff viewer
// returns to this input mode, preserving the pasted text for further editing.
func (pv *PasteViewer) computeDiff() {
	leftText := strings.TrimSpace(pv.leftArea.GetText())
	rightText := strings.TrimSpace(pv.rightArea.GetText())

	if leftText == "" && rightText == "" {
		pv.statusBar.SetText("[#ff4444]Both sides are empty — paste some text first.[-:-:-]")
		return
	}

	lines := DiffLines(leftText, rightText)
	dv := NewDiffViewer(lines, "Left (pasted)", "Right (pasted)")
	dv.build(pv.app) // reuse the same application instance
	dv.SetOnEdit(func() {
		// Drop the diff viewer's width-aware redraw hook so it does not fire
		// while the paste panels are showing.
		pv.app.SetBeforeDrawFunc(nil)
		pv.refreshInputStatus()
		pv.app.SetInputCapture(pv.inputCapture)
		pv.app.SetRoot(pv.inputRoot, true).EnableMouse(true)
		pv.app.SetFocus(pv.leftArea)
	})
	pv.app.SetRoot(dv.Root(), true).EnableMouse(true)
}
