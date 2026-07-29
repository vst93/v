package plugin_enc

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// codecMode pairs a display label with the internal transform mode.
type codecMode struct {
	label string
	mode  string
}

// All encode/decode modes for the TUI selector.
var codecModes = []codecMode{
	{"Base64 Enc", "base64enc"},
	{"Base64 Dec", "base64dec"},
	{"Base32 Enc", "base32enc"},
	{"Base32 Dec", "base32dec"},
	{"URL Enc", "urlenc"},
	{"URL Dec", "urldec"},
	{"Hex Enc", "hexenc"},
	{"Hex Dec", "hexdec"},
	{"HTML Esc", "htmlesc"},
	{"HTML Unesc", "htmlunesc"},
	{"Unicode Esc", "unienc"},
	{"Unicode Unesc", "unidec"},
}

// focus areas
const (
	focusMode  = 0
	focusInput = 1
	focusOut   = 2
)

// encTUI holds the interactive encoder TUI state.
type encTUI struct {
	app        *tview.Application
	modeIdx    int
	focus      int
	inputArea  *tview.TextView
	outputArea *tview.TextView
	modeBar    *tview.TextView
	statusBar  *tview.TextView
	inputText  string
	outputText string
	cursorPos  int // cursor position in inputText (byte offset)
}

func runTUI() error {
	ui := &encTUI{
		modeIdx: 0,
		focus:   focusInput,
	}

	ui.app = tview.NewApplication()

	// --- Mode selector bar ---
	ui.modeBar = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetTextAlign(tview.AlignCenter)
	ui.modeBar.SetBorder(true).SetTitle(" Codec ").SetTitleAlign(tview.AlignCenter)

	// --- Input area ---
	ui.inputArea = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	ui.inputArea.SetBorder(true).SetTitle(" Input ").SetTitleAlign(tview.AlignLeft)

	// --- Output area ---
	ui.outputArea = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	ui.outputArea.SetBorder(true).SetTitle(" Output ").SetTitleAlign(tview.AlignLeft)

	// --- Status bar ---
	ui.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// --- Layout ---
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.modeBar, 3, 0, false).
		AddItem(ui.inputArea, 0, 1, true).
		AddItem(ui.outputArea, 0, 1, false).
		AddItem(ui.statusBar, 1, 0, false)

	ui.refreshModeBar()
	ui.refreshInput()
	ui.refreshOutput()
	ui.refreshAreas()
	ui.refreshStatus()

	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Tab / Shift+Tab: cycle focus
		if event.Key() == tcell.KeyTab {
			ui.focus = (ui.focus + 1) % 3
			ui.refreshAreas()
			ui.refreshStatus()
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ui.focus = (ui.focus - 1 + 3) % 3
			ui.refreshAreas()
			ui.refreshStatus()
			return nil
		}

		switch ui.focus {
		case focusMode:
			return ui.handleModeInput(event)
		case focusInput:
			return ui.handleInputEdit(event)
		case focusOut:
			return ui.handleOutputView(event)
		}
		return event
	})

	if err := ui.app.SetRoot(layout, true).Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

// handleModeInput handles keys when the mode selector is focused.
func (ui *encTUI) handleModeInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyLeft:
		ui.modeIdx = (ui.modeIdx - 1 + len(codecModes)) % len(codecModes)
		ui.refreshModeBar()
		ui.doTransform()
		return nil
	case tcell.KeyRight:
		ui.modeIdx = (ui.modeIdx + 1) % len(codecModes)
		ui.refreshModeBar()
		ui.doTransform()
		return nil
	case tcell.KeyUp:
		ui.modeIdx = (ui.modeIdx - 1 + len(codecModes)) % len(codecModes)
		ui.refreshModeBar()
		ui.doTransform()
		return nil
	case tcell.KeyDown:
		ui.modeIdx = (ui.modeIdx + 1) % len(codecModes)
		ui.refreshModeBar()
		ui.doTransform()
		return nil
	case tcell.KeyEnter:
		ui.focus = focusInput
		ui.refreshAreas()
		ui.refreshStatus()
		return nil
	case tcell.KeyEscape:
		ui.app.Stop()
		return nil
	case tcell.KeyRune:
		ch := event.Rune()
		// Number keys 1-9 quick select
		if ch >= '1' && ch <= '9' {
			idx := int(ch - '1')
			if idx < len(codecModes) {
				ui.modeIdx = idx
				ui.refreshModeBar()
				ui.doTransform()
				return nil
			}
		}
	}
	return event
}

// handleInputEdit handles keys when the input area is focused (editable).
func (ui *encTUI) handleInputEdit(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		ui.app.Stop()
		return nil
	case tcell.KeyEnter:
		ui.insertText("\n")
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if ui.cursorPos > 0 {
			// Find start of previous rune
			pos := ui.cursorPos
			for pos > 0 && (ui.inputText[pos-1] & 0xC0) == 0x80 {
				pos--
			}
			if pos > 0 {
				pos--
			}
			ui.inputText = ui.inputText[:pos] + ui.inputText[ui.cursorPos:]
			ui.cursorPos = pos
			ui.refreshInput()
			ui.doTransform()
		}
		return nil
	case tcell.KeyDelete:
		if ui.cursorPos < len(ui.inputText) {
			end := ui.cursorPos + 1
			for end < len(ui.inputText) && (ui.inputText[end] & 0xC0) == 0x80 {
				end++
			}
			ui.inputText = ui.inputText[:ui.cursorPos] + ui.inputText[end:]
			ui.refreshInput()
			ui.doTransform()
		}
		return nil
	case tcell.KeyLeft:
		if ui.cursorPos > 0 {
			pos := ui.cursorPos
			for pos > 0 && (ui.inputText[pos-1] & 0xC0) == 0x80 {
				pos--
			}
			if pos > 0 {
				pos--
			}
			ui.cursorPos = pos
			ui.refreshInput()
		}
		return nil
	case tcell.KeyRight:
		if ui.cursorPos < len(ui.inputText) {
			end := ui.cursorPos + 1
			for end < len(ui.inputText) && (ui.inputText[end] & 0xC0) == 0x80 {
				end++
			}
			ui.cursorPos = end
			ui.refreshInput()
		}
		return nil
	case tcell.KeyCtrlA:
		ui.cursorPos = 0
		ui.refreshInput()
		return nil
	case tcell.KeyCtrlE:
		ui.cursorPos = len(ui.inputText)
		ui.refreshInput()
		return nil
	case tcell.KeyCtrlU:
		ui.inputText = ui.inputText[ui.cursorPos:]
		ui.cursorPos = 0
		ui.refreshInput()
		ui.doTransform()
		return nil
	case tcell.KeyCtrlK:
		ui.inputText = ui.inputText[:ui.cursorPos]
		ui.refreshInput()
		ui.doTransform()
		return nil
	case tcell.KeyCtrlL:
		ui.inputText = ""
		ui.outputText = ""
		ui.cursorPos = 0
		ui.refreshInput()
		ui.refreshOutput()
		return nil
	case tcell.KeyCtrlV:
		clip, err := clipboard.ReadAll()
		if err == nil && clip != "" {
			ui.insertText(clip)
		}
		return nil
	case tcell.KeyCtrlR:
		// Reverse: swap input/output, switch to reverse mode
		if ui.outputText != "" {
			ui.inputText = ui.outputText
			ui.outputText = ""
			ui.cursorPos = len(ui.inputText)
			ui.modeIdx = ui.reverseModeIdx()
			ui.refreshModeBar()
			ui.refreshInput()
			ui.doTransform()
		}
		return nil
	case tcell.KeyRune:
		ui.insertText(string(event.Rune()))
		return nil
	}
	return event
}

// handleOutputView handles keys when the output area is focused.
func (ui *encTUI) handleOutputView(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		ui.app.Stop()
		return nil
	case tcell.KeyEnter:
		// Copy to clipboard
		if ui.outputText != "" {
			_ = clipboard.WriteAll(ui.outputText)
			ui.statusBar.SetText("[green]✅ Copied to clipboard[-:-]")
		}
		return nil
	case tcell.KeyRune:
		ch := event.Rune()
		if ch == 'c' || ch == 'C' {
			if ui.outputText != "" {
				_ = clipboard.WriteAll(ui.outputText)
				ui.statusBar.SetText("[green]✅ Copied to clipboard[-:-]")
			}
			return nil
		}
	}
	// Let arrow keys scroll the output
	return event
}

// insertText inserts text at cursor position and transforms.
func (ui *encTUI) insertText(s string) {
	ui.inputText = ui.inputText[:ui.cursorPos] + s + ui.inputText[ui.cursorPos:]
	ui.cursorPos += len(s)
	ui.refreshInput()
	ui.doTransform()
}

// doTransform runs the current mode on the input and updates the output area.
func (ui *encTUI) doTransform() {
	if strings.TrimSpace(ui.inputText) == "" {
		ui.outputText = ""
		ui.refreshOutput()
		ui.refreshStatus()
		return
	}
	result, err := doTransform(codecModes[ui.modeIdx].mode, ui.inputText)
	if err != nil {
		ui.outputText = ""
		ui.refreshOutput()
		ui.statusBar.SetText(fmt.Sprintf("[red]❌ %s[-:-]", err.Error()))
		return
	}
	ui.outputText = result
	ui.refreshOutput()
	ui.refreshStatus()
}

// reverseModeIdx returns the index of the reverse encode/decode mode.
func (ui *encTUI) reverseModeIdx() int {
	if ui.modeIdx%2 == 0 {
		return ui.modeIdx + 1
	}
	return ui.modeIdx - 1
}

// refreshModeBar renders the mode selector with the active mode highlighted.
func (ui *encTUI) refreshModeBar() {
	var sb strings.Builder
	sb.WriteString(" ")
	for i, m := range codecModes {
		if i == ui.modeIdx {
			sb.WriteString(fmt.Sprintf("[black:green:b] %s [-:-:-]", m.label))
		} else {
			sb.WriteString(fmt.Sprintf("[::d] %s [-:-:-]", m.label))
		}
		if i < len(codecModes)-1 {
			sb.WriteString(" ")
		}
	}
	ui.modeBar.SetText(sb.String())
}

// refreshInput renders the input text with cursor.
func (ui *encTUI) refreshInput() {
	if ui.focus == focusInput {
		// Show cursor
		before := tview.Escape(ui.inputText[:ui.cursorPos])
		ch := " "
		after := ""
		if ui.cursorPos < len(ui.inputText) {
			ch = tview.Escape(string(ui.inputText[ui.cursorPos]))
		}
		if ui.cursorPos+1 < len(ui.inputText) {
			after = tview.Escape(ui.inputText[ui.cursorPos+1:])
		}
		ui.inputArea.SetText(fmt.Sprintf("%s[black:white:b]%s[-:-:-]%s", before, ch, after))
	} else {
		ui.inputArea.SetText(tview.Escape(ui.inputText))
	}
	ui.inputArea.SetTitle(fmt.Sprintf(" Input (%d chars) ", len(ui.inputText)))
}

// refreshOutput renders the output text.
func (ui *encTUI) refreshOutput() {
	ui.outputArea.SetText(tview.Escape(ui.outputText))
	ui.outputArea.SetTitle(fmt.Sprintf(" Output (%d chars) ", len(ui.outputText)))
}

// refreshAreas updates border colors to show focus.
func (ui *encTUI) refreshAreas() {
	accent := tcell.Color42
	ui.modeBar.SetBorderColor(tcell.ColorDefault)
	ui.inputArea.SetBorderColor(tcell.ColorDefault)
	ui.outputArea.SetBorderColor(tcell.ColorDefault)
	switch ui.focus {
	case focusMode:
		ui.modeBar.SetBorderColor(accent)
	case focusInput:
		ui.inputArea.SetBorderColor(accent)
	case focusOut:
		ui.outputArea.SetBorderColor(accent)
	}
}

// refreshStatus updates the status bar with current state and keybindings.
func (ui *encTUI) refreshStatus() {
	if ui.outputText != "" {
		ui.statusBar.SetText(fmt.Sprintf(
			"[yellow]Tab[-] Next  [yellow]←→[-] Switch Codec  [yellow]Enter[-] Copy/Edit  [yellow]Ctrl-R[-] Reverse  [yellow]Ctrl-L[-] Clear  [yellow]Esc[-] Quit  [::d]%d→%d[-:-]",
			len(ui.inputText), len(ui.outputText)))
	} else {
		ui.statusBar.SetText("[yellow]Tab[-] Next  [yellow]←→[-] Switch Codec  [yellow]Enter[-] Edit Input  [yellow]Ctrl-V[-] Paste  [yellow]Ctrl-L[-] Clear  [yellow]Esc[-] Quit")
	}
}
