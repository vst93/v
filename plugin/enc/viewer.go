package plugin_enc

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// codecType is a top-level encoding type (left column).
type codecType struct {
	label string
	key   rune // quick-select key (lowercase)
}

// All encoding types for the left column.
var codecTypes = []codecType{
	{"Base64", 'b'},
	{"Base32", 'a'},
	{"URL", 'u'},
	{"Hex", 'h'},
	{"HTML", 'm'},
	{"Unicode", 'n'},
}

// direction: 0 = encode, 1 = decode (default decode)
const (
	dirEncode = 0
	dirDecode = 1
)

// modeFromType builds the internal mode string from a type index + direction.
func modeFromType(typeIdx, dir int) string {
	switch typeIdx {
	case 0: // Base64
		if dir == dirDecode {
			return "base64dec"
		}
		return "base64enc"
	case 1: // Base32
		if dir == dirDecode {
			return "base32dec"
		}
		return "base32enc"
	case 2: // URL
		if dir == dirDecode {
			return "urldec"
		}
		return "urlenc"
	case 3: // Hex
		if dir == dirDecode {
			return "hexdec"
		}
		return "hexenc"
	case 4: // HTML
		if dir == dirDecode {
			return "htmlunesc"
		}
		return "htmlesc"
	case 5: // Unicode
		if dir == dirDecode {
			return "unidec"
		}
		return "unienc"
	}
	return "base64enc"
}

// encTUI holds the interactive encoder TUI state.
type encTUI struct {
	app        *tview.Application
	typeIdx    int   // selected codec type (left column)
	dir        int   // 0=encode, 1=decode (right column, default decode)
	focus      int   // which panel is focused
	inputArea  *tview.TextView
	outputArea *tview.TextView
	typeList   *tview.TextView
	dirList    *tview.TextView
	statusBar  *tview.TextView
	inputText  string
	outputText string
	cursorPos  int // cursor position in inputText (byte offset)
}

// focus areas
const (
	focusType  = 0
	focusDir   = 1
	focusInput = 2
	focusOut   = 3
)

func runTUI() error {
	ui := &encTUI{
		typeIdx: 0,
		dir:     dirDecode, // default decode
		focus:   focusInput,
	}

	ui.app = tview.NewApplication()

	// --- Left column: codec type selector ---
	ui.typeList = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	ui.typeList.SetBorder(true).SetTitle(" Codec ").SetTitleAlign(tview.AlignCenter)

	// --- Right column: encode/decode selector ---
	ui.dirList = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	ui.dirList.SetBorder(true).SetTitle(" Mode ").SetTitleAlign(tview.AlignCenter)

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

	// --- Selector row: two columns side by side ---
	selectorRow := tview.NewFlex().
		AddItem(ui.typeList, 14, 0, false).
		AddItem(ui.dirList, 12, 0, false)

	// --- Main layout ---
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(selectorRow, 8, 0, false).
		AddItem(ui.inputArea, 0, 1, true).
		AddItem(ui.outputArea, 0, 1, false).
		AddItem(ui.statusBar, 1, 0, false)

	ui.refreshTypeList()
	ui.refreshDirList()
	ui.refreshInput()
	ui.refreshOutput()
	ui.refreshAreas()
	ui.refreshStatus()

	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Tab / Shift+Tab: cycle focus (4 areas)
		if event.Key() == tcell.KeyTab {
			ui.focus = (ui.focus + 1) % 4
			ui.refreshAreas()
			ui.refreshStatus()
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ui.focus = (ui.focus - 1 + 4) % 4
			ui.refreshAreas()
			ui.refreshStatus()
			return nil
		}

		switch ui.focus {
		case focusType:
			return ui.handleTypeInput(event)
		case focusDir:
			return ui.handleDirInput(event)
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

// handleTypeInput handles keys when the codec type list is focused.
func (ui *encTUI) handleTypeInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyUp:
		ui.typeIdx = (ui.typeIdx - 1 + len(codecTypes)) % len(codecTypes)
		ui.refreshTypeList()
		ui.doTransform()
		return nil
	case tcell.KeyDown:
		ui.typeIdx = (ui.typeIdx + 1) % len(codecTypes)
		ui.refreshTypeList()
		ui.doTransform()
		return nil
	case tcell.KeyLeft, tcell.KeyRight:
		ui.focus = focusDir
		ui.refreshAreas()
		ui.refreshStatus()
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
		lower := ch
		if ch >= 'A' && ch <= 'Z' {
			lower = ch + 32
		}
		// Quick-select by letter key
		for i, t := range codecTypes {
			if t.key == lower {
				ui.typeIdx = i
				ui.refreshTypeList()
				ui.doTransform()
				return nil
			}
		}
	}
	return event
}

// handleDirInput handles keys when the encode/decode list is focused.
func (ui *encTUI) handleDirInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyUp:
		ui.dir = (ui.dir - 1 + 2) % 2
		ui.refreshDirList()
		ui.doTransform()
		return nil
	case tcell.KeyDown:
		ui.dir = (ui.dir + 1) % 2
		ui.refreshDirList()
		ui.doTransform()
		return nil
	case tcell.KeyLeft, tcell.KeyRight:
		ui.focus = focusType
		ui.refreshAreas()
		ui.refreshStatus()
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
		lower := ch
		if ch >= 'A' && ch <= 'Z' {
			lower = ch + 32
		}
		// e = encode, d = decode
		if lower == 'e' {
			ui.dir = dirEncode
			ui.refreshDirList()
			ui.doTransform()
			return nil
		}
		if lower == 'd' {
			ui.dir = dirDecode
			ui.refreshDirList()
			ui.doTransform()
			return nil
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
		// Reverse: swap input/output, switch direction
		if ui.outputText != "" {
			ui.inputText = ui.outputText
			ui.outputText = ""
			ui.cursorPos = len(ui.inputText)
			// Toggle direction
			ui.dir = 1 - ui.dir
			ui.refreshDirList()
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
	mode := modeFromType(ui.typeIdx, ui.dir)
	result, err := doTransform(mode, ui.inputText)
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

// refreshTypeList renders the codec type list (left column).
func (ui *encTUI) refreshTypeList() {
	var sb strings.Builder
	for i, t := range codecTypes {
		if i == ui.typeIdx {
			sb.WriteString(fmt.Sprintf("[black:green:b] %s (%s) [-:-:-]", t.label, string(t.key)))
		} else {
			sb.WriteString(fmt.Sprintf("[::d] %s (%s) [-:-:-]", t.label, string(t.key)))
		}
		if i < len(codecTypes)-1 {
			sb.WriteString("\n")
		}
	}
	ui.typeList.SetText(sb.String())
}

// refreshDirList renders the encode/decode list (right column).
func (ui *encTUI) refreshDirList() {
	var sb strings.Builder
	encodeLabel := " Encode (e)"
	decodeLabel := " Decode (d)"
	if ui.dir == dirEncode {
		sb.WriteString(fmt.Sprintf("[black:green:b]%s[-:-:-]", encodeLabel))
	} else {
		sb.WriteString(fmt.Sprintf("[::d]%s[-:-:-]", encodeLabel))
	}
	sb.WriteString("\n")
	if ui.dir == dirDecode {
		sb.WriteString(fmt.Sprintf("[black:green:b]%s[-:-:-]", decodeLabel))
	} else {
		sb.WriteString(fmt.Sprintf("[::d]%s[-:-:-]", decodeLabel))
	}
	ui.dirList.SetText(sb.String())
}

// refreshInput renders the input text with cursor.
func (ui *encTUI) refreshInput() {
	if ui.focus == focusInput {
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
	ui.typeList.SetBorderColor(tcell.ColorDefault)
	ui.dirList.SetBorderColor(tcell.ColorDefault)
	ui.inputArea.SetBorderColor(tcell.ColorDefault)
	ui.outputArea.SetBorderColor(tcell.ColorDefault)
	switch ui.focus {
	case focusType:
		ui.typeList.SetBorderColor(accent)
	case focusDir:
		ui.dirList.SetBorderColor(accent)
	case focusInput:
		ui.inputArea.SetBorderColor(accent)
	case focusOut:
		ui.outputArea.SetBorderColor(accent)
	}
}

// refreshStatus updates the status bar with current state and keybindings.
func (ui *encTUI) refreshStatus() {
	modeLabel := "Encode"
	if ui.dir == dirDecode {
		modeLabel = "Decode"
	}
	typeLabel := codecTypes[ui.typeIdx].label
	if ui.outputText != "" {
		ui.statusBar.SetText(fmt.Sprintf(
			"[yellow]Tab[-] Next  [yellow]↑↓[-] Select  [yellow]Enter[-] Copy/Edit  [yellow]Ctrl-R[-] Reverse  [yellow]Ctrl-L[-] Clear  [yellow]Esc[-] Quit  [::d]%s %s %d→%d[-:-]",
			typeLabel, modeLabel, len(ui.inputText), len(ui.outputText)))
	} else {
		ui.statusBar.SetText(fmt.Sprintf(
			"[yellow]Tab[-] Next  [yellow]↑↓[-] Select  [yellow]Enter[-] Edit Input  [yellow]Ctrl-V[-] Paste  [yellow]Ctrl-L[-] Clear  [yellow]Esc[-] Quit  [::d]%s %s[-:-]",
			typeLabel, modeLabel))
	}
}
