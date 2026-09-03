package plugin_codec

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"v/internal/theme"
)

// codecSpec is one entry in the Codec list: a display label plus the two
// transform modes it maps to.
type codecSpec struct {
	label    string
	shortcut rune
	enc      string
	dec      string
}

var codecs = []codecSpec{
	{"Base64", '1', "base64enc", "base64dec"},
	{"Base32", '2', "base32enc", "base32dec"},
	{"URL", '3', "urlenc", "urldec"},
	{"Hex", '4', "hexenc", "hexdec"},
	{"HTML", '5', "htmlesc", "htmlunesc"},
	{"Unicode", '6', "unienc", "unidec"},
}

// direction: 0 = encode, 1 = decode (decode is the default)
const (
	dirEncode = 0
	dirDecode = 1
)

// sidebarWidth is the fixed width of the Codec/Mode column. It fits the
// widest item ("Unicode") plus the 4 columns tview.List reserves for the
// "(n)" shortcut, plus the box border.
const sidebarWidth = 18

// narrowCols is the terminal width below which the sidebar moves from a left
// column to a row above the editor, so nothing gets clipped.
const narrowCols = 64

type codecTUI struct {
	app *tview.Application

	// theme-derived colors, resolved once at startup.
	accent       tcell.Color
	accentFg     tcell.Color
	dim          tcell.Color
	errColor     tcell.Color
	warnColor    tcell.Color
	successColor tcell.Color

	codecList *tview.List
	modeList  *tview.List
	input     *tview.TextArea
	output    *tview.TextView
	copyBtn   *tview.Button
	swapBtn   *tview.Button
	status    *tview.TextView

	root    *tview.Flex // outer column: body + action bar
	body    *tview.Flex // sidebar + editor, direction depends on width
	sidebar *tview.Flex
	editor  *tview.Flex

	focusRing []tview.Primitive
	focusIdx  int

	narrow     bool
	narrowInit bool

	outputText string
}

// runTUI launches the interactive encoder. seed pre-fills the input area.
func runTUI(seed string) error {
	theme.Init(true)
	theme.ApplyTView() // stock widgets must not assume a black terminal
	p := theme.Current()
	ui := &codecTUI{
		app:          tview.NewApplication(),
		accent:       p.Accent,
		accentFg:     p.AccentFg,
		dim:          p.TextDim,
		errColor:     p.Error,
		warnColor:    p.Warn,
		successColor: p.Success,
	}

	ui.buildWidgets()
	ui.buildLayout(false)
	ui.narrow, ui.narrowInit = false, true

	ui.focusRing = []tview.Primitive{ui.input, ui.codecList, ui.modeList, ui.output, ui.copyBtn, ui.swapBtn}

	if seed != "" {
		ui.input.SetText(seed, true)
	}
	ui.transform()
	ui.setFocus(0)

	// Reflow when the terminal gets too narrow for a side column.
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, _ := screen.Size()
		narrow := w < narrowCols
		if narrow != ui.narrow || !ui.narrowInit {
			ui.narrow, ui.narrowInit = narrow, true
			ui.buildLayout(narrow)
		}
		return false
	})

	ui.app.SetInputCapture(ui.globalKeys)

	if err := ui.app.SetRoot(ui.root, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

func (ui *codecTUI) buildWidgets() {
	// --- Codec selector: a real List, so clicks and taps select natively ---
	ui.codecList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(ui.accent).Foreground(ui.accentFg).Bold(true)).
		SetShortcutColor(ui.dim)
	for _, c := range codecs {
		ui.codecList.AddItem(c.label, "", c.shortcut, nil)
	}
	ui.codecList.SetChangedFunc(func(int, string, string, rune) { ui.transform() })
	ui.codecList.SetSelectedFunc(func(int, string, string, rune) { ui.focusOn(ui.input) })
	ui.codecList.SetBorder(true).SetTitle(" Codec ").SetTitleAlign(tview.AlignLeft)

	// --- Encode/Decode selector ---
	ui.modeList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(ui.accent).Foreground(ui.accentFg).Bold(true)).
		SetShortcutColor(ui.dim)
	ui.modeList.AddItem("Encode", "", 'e', nil)
	ui.modeList.AddItem("Decode", "", 'd', nil)
	ui.modeList.SetCurrentItem(dirDecode)
	ui.modeList.SetChangedFunc(func(int, string, string, rune) { ui.transform() })
	ui.modeList.SetSelectedFunc(func(int, string, string, rune) { ui.focusOn(ui.input) })
	ui.modeList.SetBorder(true).SetTitle(" Mode ").SetTitleAlign(tview.AlignLeft)

	// --- Input: TextArea brings cursor placement, drag-selection, wheel
	// scrolling and undo/redo for free. Wire its clipboard to the real one. ---
	ui.input = tview.NewTextArea().
		SetWrap(true).
		SetPlaceholder("Type or paste here, or press Ctrl-V to paste the clipboard...")
	ui.input.SetClipboard(
		func(s string) { _ = clipboard.WriteAll(s) },
		func() string { s, _ := clipboard.ReadAll(); return s },
	)
	ui.input.SetChangedFunc(ui.transform)
	ui.input.SetBorder(true).SetTitle(" Input ").SetTitleAlign(tview.AlignLeft)

	// --- Output ---
	ui.output = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	ui.output.SetBorder(true).SetTitle(" Output ").SetTitleAlign(tview.AlignLeft)

	// --- Action bar: buttons are clickable and tappable ---
	ui.copyBtn = tview.NewButton("⧉ Copy").SetSelectedFunc(ui.copyOutput)
	ui.swapBtn = tview.NewButton("⇄ Swap").SetSelectedFunc(ui.swap)

	ui.status = tview.NewTextView().SetDynamicColors(true)
}

// buildLayout (re)assembles the widget tree. Called once at startup and again
// whenever the terminal crosses the narrow threshold.
func (ui *codecTUI) buildLayout(narrow bool) {
	if ui.root == nil {
		ui.sidebar = tview.NewFlex()
		ui.editor = tview.NewFlex().SetDirection(tview.FlexRow)
		ui.body = tview.NewFlex()

		actionBar := tview.NewFlex().
			AddItem(ui.copyBtn, 10, 0, false).
			AddItem(nil, 1, 0, false).
			AddItem(ui.swapBtn, 10, 0, false).
			AddItem(nil, 2, 0, false).
			AddItem(ui.status, 0, 1, false)

		ui.root = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(ui.body, 0, 1, true).
			AddItem(actionBar, 1, 0, false)
	}

	ui.editor.Clear().
		AddItem(ui.input, 0, 1, true).
		AddItem(ui.output, 0, 1, false)

	ui.sidebar.Clear()
	ui.body.Clear()

	if narrow {
		// Selectors side by side above the editor.
		ui.sidebar.SetDirection(tview.FlexColumn).
			AddItem(ui.codecList, 0, 1, false).
			AddItem(ui.modeList, 0, 1, false)
		ui.body.SetDirection(tview.FlexRow).
			AddItem(ui.sidebar, 8, 0, false).
			AddItem(ui.editor, 0, 1, true)
		return
	}

	// Wide: fixed left column. The Codec box grows to absorb spare height so
	// the sidebar never leaves a dead gap.
	ui.sidebar.SetDirection(tview.FlexRow).
		AddItem(ui.codecList, 0, 1, false).
		AddItem(ui.modeList, 4, 0, false)
	ui.body.SetDirection(tview.FlexColumn).
		AddItem(ui.sidebar, sidebarWidth, 0, false).
		AddItem(ui.editor, 0, 1, true)
}

// globalKeys handles the shortcuts that must work regardless of focus. Tab is
// intercepted here so it always cycles focus instead of being swallowed by the
// text area as an indent.
func (ui *codecTUI) globalKeys(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		ui.setFocus(ui.focusIdx + 1)
		return nil
	case tcell.KeyBacktab:
		ui.setFocus(ui.focusIdx - 1)
		return nil
	case tcell.KeyEscape:
		ui.app.Stop()
		return nil
	case tcell.KeyCtrlR:
		ui.swap()
		return nil
	case tcell.KeyCtrlL:
		ui.input.SetText("", false)
		ui.transform()
		return nil
	case tcell.KeyCtrlY:
		ui.copyOutput()
		return nil
	}

	// 'y' copies the output, but only when the text area does not have focus —
	// otherwise it would eat a perfectly ordinary letter.
	if event.Key() == tcell.KeyRune && (event.Rune() == 'y' || event.Rune() == 'Y') && !ui.input.HasFocus() {
		ui.copyOutput()
		return nil
	}
	return event
}

// setFocus moves focus around the ring, wrapping at both ends.
func (ui *codecTUI) setFocus(idx int) {
	n := len(ui.focusRing)
	ui.focusIdx = ((idx % n) + n) % n
	ui.app.SetFocus(ui.focusRing[ui.focusIdx])
	ui.refreshBorders()
	ui.refreshStatus("")
}

// focusOn jumps directly to a primitive, keeping the ring index in sync.
func (ui *codecTUI) focusOn(p tview.Primitive) {
	for i, item := range ui.focusRing {
		if item == p {
			ui.setFocus(i)
			return
		}
	}
}

func (ui *codecTUI) refreshBorders() {
	for _, p := range []*tview.Box{
		ui.codecList.Box, ui.modeList.Box, ui.input.Box, ui.output.Box,
	} {
		p.SetBorderColor(ui.dim)
	}
	switch ui.focusRing[ui.focusIdx] {
	case ui.codecList:
		ui.codecList.SetBorderColor(ui.accent)
	case ui.modeList:
		ui.modeList.SetBorderColor(ui.accent)
	case ui.input:
		ui.input.SetBorderColor(ui.accent)
	case ui.output:
		ui.output.SetBorderColor(ui.accent)
	}
}

// mode returns the transform mode for the current codec + direction.
func (ui *codecTUI) mode() string {
	c := codecs[ui.codecList.GetCurrentItem()]
	if ui.modeList.GetCurrentItem() == dirDecode {
		return c.dec
	}
	return c.enc
}

// transform re-runs the current codec over the input and repaints the output.
func (ui *codecTUI) transform() {
	in := ui.input.GetText()
	ui.input.SetTitle(fmt.Sprintf(" Input (%d chars) ", len(in)))

	if strings.TrimSpace(in) == "" {
		ui.outputText = ""
		ui.output.SetText("")
		ui.output.SetTitle(" Output ")
		ui.refreshStatus("")
		return
	}

	result, err := doTransform(ui.mode(), in)
	if err != nil {
		ui.outputText = ""
		// Show the reason in place rather than flashing the status bar, so it
		// is obvious why the output is empty.
		ui.output.SetText(fmt.Sprintf("[%s]%s[-]", theme.Hex(ui.errColor), tview.Escape(err.Error())))
		ui.output.SetTitle(" Output ")
		ui.refreshStatus("")
		return
	}

	ui.outputText = result
	ui.output.SetText(tview.Escape(result))
	ui.output.SetTitle(fmt.Sprintf(" Output (%d chars) ", len(result)))
	ui.refreshStatus("")
}

func (ui *codecTUI) copyOutput() {
	if ui.outputText == "" {
		ui.refreshStatus(fmt.Sprintf("[%s]nothing to copy[-]", theme.Hex(ui.warnColor)))
		return
	}
	if err := clipboard.WriteAll(ui.outputText); err != nil {
		ui.refreshStatus(fmt.Sprintf("[%s]clipboard write failed[-]", theme.Hex(ui.errColor)))
		return
	}
	ui.refreshStatus(fmt.Sprintf("[%s]✅ copied to clipboard[-]", theme.Hex(ui.successColor)))
}

// swap feeds the output back in as the new input and flips encode/decode, so
// a round-trip is a single keystroke.
func (ui *codecTUI) swap() {
	if ui.outputText == "" {
		return
	}
	ui.input.SetText(ui.outputText, true)
	if ui.modeList.GetCurrentItem() == dirDecode {
		ui.modeList.SetCurrentItem(dirEncode)
	} else {
		ui.modeList.SetCurrentItem(dirDecode)
	}
	ui.transform() // SetCurrentItem already fires it, but keep it explicit
}

// refreshStatus repaints the hint line. A non-empty toast replaces the hints.
func (ui *codecTUI) refreshStatus(toast string) {
	if toast != "" {
		ui.status.SetText(toast)
		return
	}
	dir := "Encode"
	if ui.modeList.GetCurrentItem() == dirDecode {
		dir = "Decode"
	}
	ui.status.SetText(fmt.Sprintf(
		"[%s]Tab[-] focus  [%s]↑↓[-] select  [%s]y[-] copy  [%s]^R[-] swap  [%s]^L[-] clear  [%s]Esc[-] quit   [::d]%s %s  %d→%d[-:-:-]",
		theme.Hex(ui.accent), theme.Hex(ui.accent), theme.Hex(ui.accent), theme.Hex(ui.accent), theme.Hex(ui.accent), theme.Hex(ui.accent),
		codecs[ui.codecList.GetCurrentItem()].label, dir,
		len(ui.input.GetText()), len(ui.outputText),
	))
}
