package plugin_diff

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"v/internal/theme"
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
	theme.Init(true)
	theme.ApplyTView()
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
		{"Tab", "switch"}, {"^A", "all"}, {"^C/X/V", "copy/cut/paste"}, {"^D", "diff"}, {"Esc", "quit"},
	}))

	// Keep titles and the status bar in sync as the user types or pastes.
	refresh := func() { pv.refreshInputStatus() }
	pv.leftArea.SetChangedFunc(refresh)
	pv.rightArea.SetChangedFunc(refresh)

	// Convert KeyCtrlJ (tcell's mapping for \n) to KeyEnter so that
	// pasted newlines are handled correctly by TextArea.
	ctrlJFix := func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyCtrlJ {
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		}
		return ev
	}
	pv.leftArea.SetInputCapture(ctrlJFix)
	pv.rightArea.SetInputCapture(ctrlJFix)

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

	p := theme.Current()
	pv.leftTitle.SetText(fmt.Sprintf("[%s::b]◀ Left (paste)[-:-:-]  [%s]L: %d lines[-]",
		theme.Hex(p.Error), theme.Hex(p.TextDim), lLines))
	pv.rightTitle.SetText(fmt.Sprintf("[%s::b]Right (paste) ▶[-:-:-]  [%s]R: %d lines[-]",
		theme.Hex(p.Success), theme.Hex(p.TextDim), rLines))

	pv.statusBar.SetText(fmt.Sprintf(" [%s]Paste · L %d ln · R %d ln[-]", colorBarMain, lLines, rLines))
}

// lineCount returns the number of lines in s (0 for empty, newline+1 otherwise).
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// inputCapture handles Tab (switch panel), Ctrl+D (compute diff), Escape
// (quit), and remaps standard editing shortcuts to TextArea's internal
// bindings: Ctrl-A -> Ctrl-L (select all), Ctrl-C -> Ctrl-Q (copy).
// Ctrl-X (cut) and Ctrl-V (paste) are handled by TextArea natively.
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
	case tcell.KeyEscape:
		pv.app.Stop()
		return nil
	case tcell.KeyCtrlA:
		// Select all: TextArea uses Ctrl-L for this.
		return tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone)
	case tcell.KeyCtrlC:
		// Copy: TextArea uses Ctrl-Q. Returning a different event also
		// prevents tview's built-in Ctrl-C quit.
		return tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
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
		pv.statusBar.SetText(fmt.Sprintf("[%s]Both sides are empty — paste some text first.[-:-:-]", theme.Hex(theme.Current().Error)))
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
