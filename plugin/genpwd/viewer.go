package plugin_genpwd

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Preset password lengths for the slider.
var presetLengths = []int{8, 12, 16, 20, 24, 32, 48, 64}

// PasswordUI is the interactive password generator TUI.
type PasswordUI struct {
	app       *tview.Application
	config    PasswordConfig
	pwd       string
	focus     int // which form item is focused
	lengthIdx int // index into presetLengths, -1 = custom
	items     []formItem
	preview   *tview.TextView
	status    *tview.TextView
	helpBar   *tview.TextView
}

type formItem struct {
	label    string
	itemType string // "slider", "checkbox"
}

func NewPasswordUI(config PasswordConfig) *PasswordUI {
	ui := &PasswordUI{
		config: config,
		items: []formItem{
			{label: "Length", itemType: "slider"},
			{label: "Lowercase (a-z)", itemType: "checkbox"},
			{label: "Uppercase (A-Z)", itemType: "checkbox"},
			{label: "Digits (0-9)", itemType: "checkbox"},
			{label: "Special (!@#$...)", itemType: "checkbox"},
		},
		focus: 0,
	}

	// Find closest preset or mark as custom
	ui.lengthIdx = -1
	for i, l := range presetLengths {
		if l == config.Length {
			ui.lengthIdx = i
			break
		}
	}

	return ui
}

func (ui *PasswordUI) Run() error {
	ui.app = tview.NewApplication()

	// Generate initial password
	ui.regenerate()

	// --- Preview area ---
	ui.preview = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(false)
	ui.preview.SetBorder(true).SetTitle(" 🔑 Password ").SetTitleAlign(tview.AlignCenter)
	ui.refreshPreview()

	// --- Form area ---
	form := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	form.SetBorder(true).SetTitle(" ⚙ Configuration ").SetTitleAlign(tview.AlignCenter)

	// --- Status bar ---
	ui.status = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// --- Help bar ---
	ui.helpBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	ui.refreshHelp()

	// --- Length input field (for custom length) ---
	lengthField := tview.NewInputField().
		SetLabel("Custom length (1-128): ").
		SetFieldWidth(10).
		SetAcceptanceFunc(tview.InputFieldInteger)
	lengthField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			v := strings.TrimSpace(lengthField.GetText())
			if v != "" {
				if n, err := fmtAtoi(v); err == nil && n >= 1 && n <= 128 {
					ui.config.Length = n
					// Check if it matches a preset
					ui.lengthIdx = -1
					for i, l := range presetLengths {
						if l == n {
							ui.lengthIdx = i
							break
						}
					}
				}
			}
			ui.app.SetRoot(ui.buildLayout(form), true)
			ui.refreshForm(form)
			ui.app.SetFocus(form)
			ui.regenerate()
			ui.refreshPreview()
			ui.refreshStatus()
		} else if key == tcell.KeyEscape {
			ui.app.SetRoot(ui.buildLayout(form), true)
			ui.refreshForm(form)
			ui.app.SetFocus(form)
		}
	})

	// --- Layout ---
	layout := ui.buildLayout(form)

	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If editing length field, let it handle its own keys
		if ui.app.GetFocus() == lengthField {
			return event
		}

		switch event.Key() {
		case tcell.KeyTab:
			ui.focus = (ui.focus + 1) % len(ui.items)
			ui.refreshForm(form)
			ui.refreshHelp()
			return nil
		case tcell.KeyBacktab:
			ui.focus = (ui.focus - 1 + len(ui.items)) % len(ui.items)
			ui.refreshForm(form)
			ui.refreshHelp()
			return nil
		case tcell.KeyUp:
			ui.focus = (ui.focus - 1 + len(ui.items)) % len(ui.items)
			ui.refreshForm(form)
			ui.refreshHelp()
			return nil
		case tcell.KeyDown:
			ui.focus = (ui.focus + 1) % len(ui.items)
			ui.refreshForm(form)
			ui.refreshHelp()
			return nil
		case tcell.KeyLeft:
			if ui.focus == 0 {
				ui.lengthPrev()
				ui.refreshForm(form)
				ui.regenerate()
				ui.refreshPreview()
				return nil
			}
		case tcell.KeyRight:
			if ui.focus == 0 {
				ui.lengthNext()
				ui.refreshForm(form)
				ui.regenerate()
				ui.refreshPreview()
				return nil
			}
		case tcell.KeyEnter:
			if ui.focus == 0 {
				// Edit custom length
				lengthField.SetText("")
				ui.app.SetRoot(ui.buildLengthLayout(lengthField), true)
				ui.app.SetFocus(lengthField)
				return nil
			}
			// Toggle checkbox
			ui.toggleCurrent()
			ui.refreshForm(form)
			ui.regenerate()
			ui.refreshPreview()
			ui.refreshStatus()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case ' ':
				if ui.focus > 0 {
					ui.toggleCurrent()
					ui.refreshForm(form)
					ui.regenerate()
					ui.refreshPreview()
					ui.refreshStatus()
				} else {
					// Space on slider = next preset
					ui.lengthNext()
					ui.refreshForm(form)
					ui.regenerate()
					ui.refreshPreview()
				}
				return nil
			case 'r':
				ui.regenerate()
				ui.refreshPreview()
				ui.refreshStatus()
				return nil
			case 'y':
				if err := clipboard.WriteAll(ui.pwd); err != nil {
					ui.status.SetText(fmt.Sprintf("[#ff5555]✗ Clipboard error: %s[-]", err))
				} else {
					ui.status.SetText("[#50fa7b]✅ Copied to clipboard[-]")
				}
				return nil
			case 'q':
				ui.app.Stop()
				return nil
			}
		}
		return event
	})

	ui.refreshForm(form)
	ui.refreshStatus()

	if err := ui.app.SetRoot(layout, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	return nil
}

func (ui *PasswordUI) buildLayout(form *tview.TextView) *tview.Flex {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.preview, 5, 0, false).
		AddItem(form, 0, 1, true).
		AddItem(ui.status, 1, 0, false).
		AddItem(ui.helpBar, 1, 0, false)
}

func (ui *PasswordUI) buildLengthLayout(field *tview.InputField) *tview.Flex {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.preview, 5, 0, false).
		AddItem(field, 3, 0, true).
		AddItem(ui.status, 1, 0, false).
		AddItem(ui.helpBar, 1, 0, false)
}

// lengthNext advances to the next preset (or increases by 4 if custom).
func (ui *PasswordUI) lengthNext() {
	if ui.lengthIdx >= 0 && ui.lengthIdx < len(presetLengths)-1 {
		ui.lengthIdx++
		ui.config.Length = presetLengths[ui.lengthIdx]
	} else if ui.lengthIdx < 0 {
		// Custom: increase by 4, clamp at 128
		ui.config.Length += 4
		if ui.config.Length > 128 {
			ui.config.Length = 128
		}
		// Check if we hit a preset
		for i, l := range presetLengths {
			if l == ui.config.Length {
				ui.lengthIdx = i
				return
			}
		}
	}
}

// lengthPrev goes to the previous preset (or decreases by 4 if custom).
func (ui *PasswordUI) lengthPrev() {
	if ui.lengthIdx > 0 {
		ui.lengthIdx--
		ui.config.Length = presetLengths[ui.lengthIdx]
	} else if ui.lengthIdx < 0 {
		// Custom: decrease by 4, clamp at 1
		ui.config.Length -= 4
		if ui.config.Length < 1 {
			ui.config.Length = 1
		}
		for i, l := range presetLengths {
			if l == ui.config.Length {
				ui.lengthIdx = i
				return
			}
		}
	} else if ui.lengthIdx == 0 {
		// At first preset, switch to custom going down
		ui.lengthIdx = -1
		ui.config.Length = presetLengths[0] - 4
		if ui.config.Length < 1 {
			ui.config.Length = 1
		}
	}
}

func (ui *PasswordUI) toggleCurrent() {
	switch ui.focus {
	case 1:
		ui.config.Lowercase = !ui.config.Lowercase
	case 2:
		ui.config.Uppercase = !ui.config.Uppercase
	case 3:
		ui.config.Digits = !ui.config.Digits
	case 4:
		ui.config.Special = !ui.config.Special
	}
	// Ensure at least one is on
	if !ui.config.Lowercase && !ui.config.Uppercase && !ui.config.Digits && !ui.config.Special {
		ui.config.Lowercase = true
	}
}

func (ui *PasswordUI) regenerate() {
	pwd, err := GeneratePassword(ui.config)
	if err == nil {
		ui.pwd = pwd
	}
}

func (ui *PasswordUI) refreshPreview() {
	ui.preview.Clear()
	var sb strings.Builder
	sb.WriteString("\n")
	for _, ch := range ui.pwd {
		switch {
		case ch >= 'a' && ch <= 'z':
			sb.WriteString(fmt.Sprintf("[#f8f8f2]%c[-]", ch))
		case ch >= 'A' && ch <= 'Z':
			sb.WriteString(fmt.Sprintf("[#8be9fd::b]%c[-:-:-]", ch))
		case ch >= '0' && ch <= '9':
			sb.WriteString(fmt.Sprintf("[#50fa7b]%c[-]", ch))
		default:
			sb.WriteString(fmt.Sprintf("[#f1fa8c]%c[-]", ch))
		}
	}
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("[#6272a4]%d chars | %.1f bits entropy[-]", ui.config.Length, ui.config.Entropy()))
	fmt.Fprint(ui.preview, sb.String())
}

func (ui *PasswordUI) refreshForm(form *tview.TextView) {
	form.Clear()
	var sb strings.Builder
	sb.WriteString("\n")

	for i := range ui.items {
		selected := i == ui.focus
		cursor := "  "
		if selected {
			cursor = "[#50fa7b::b]▶ [-:-:-]"
		}

		switch i {
		case 0:
			ui.writeLengthSlider(&sb, cursor, selected)
		case 1:
			ui.writeCheckbox(&sb, cursor, selected, "Lowercase   ", ui.config.Lowercase, "a-z")
		case 2:
			ui.writeCheckbox(&sb, cursor, selected, "Uppercase   ", ui.config.Uppercase, "A-Z")
		case 3:
			ui.writeCheckbox(&sb, cursor, selected, "Digits      ", ui.config.Digits, "0-9")
		case 4:
			ui.writeCheckbox(&sb, cursor, selected, "Special     ", ui.config.Special, "!@#$%^&*")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("[#6272a4]Charset: %d characters | Entropy: %.1f bits[-]\n",
		ui.config.CharsetSize(), ui.config.Entropy()))

	strengthLabel, strengthColor := passwordStrength(ui.config.Entropy())
	sb.WriteString(fmt.Sprintf("[#6272a4]Strength: [%s]%s[-][-]", strengthColor, strengthLabel))

	fmt.Fprint(form, sb.String())
}

// writeLengthSlider renders the length selection as a horizontal slider with preset stops.
func (ui *PasswordUI) writeLengthSlider(sb *strings.Builder, cursor string, selected bool) {
	labelColor := "[#f8f8f2]"
	endTag := "[-]"
	if selected {
		labelColor = "[#f8f8f2::b]"
		endTag = "[-:-:-]"
	}

	// Build the slider bar
	var bar strings.Builder
	for i, l := range presetLengths {
		if i == ui.lengthIdx {
			bar.WriteString(fmt.Sprintf("[#50fa7b::b][%d][-:-:-]", l))
		} else {
			bar.WriteString(fmt.Sprintf("[#6272a4] %d [-]", l))
		}
		if i < len(presetLengths)-1 {
			bar.WriteString("[#6272a4]─[-]")
		}
	}

	// If custom length, show it separately
	customNote := ""
	if ui.lengthIdx < 0 {
		customNote = fmt.Sprintf("  [#ffb86c](custom: %d)[-]", ui.config.Length)
	}

	sb.WriteString(fmt.Sprintf("%s%sLength:%s  %s%s\n", cursor, labelColor, endTag, bar.String(), customNote))
	sb.WriteString(fmt.Sprintf("  [#6272a4]← → preset | Enter for custom[-]\n"))
}

func (ui *PasswordUI) writeCheckbox(sb *strings.Builder, cursor string, selected bool, label string, checked bool, example string) {
	check := "[#ff5555]☐[-]"
	if checked {
		check = "[#50fa7b]☑[-]"
	}
	labelColor := "[#f8f8f2]"
	endTag := "[-]"
	if selected {
		labelColor = "[#f8f8f2::b]"
		endTag = "[-:-:-]"
	}
	sb.WriteString(fmt.Sprintf("%s%s %s%s%s [#6272a4]%s[-]\n", cursor, check, labelColor, label, endTag, example))
}

func (ui *PasswordUI) refreshStatus() {
	ui.status.SetText("[#6272a4]Press r to regenerate | y to copy[-]")
}

func (ui *PasswordUI) refreshHelp() {
	if ui.focus == 0 {
		ui.helpBar.SetText("[#888888]← → Adjust length   Enter Custom length   Space Next   Tab/↑↓ Navigate   r Regenerate   y Copy   q Quit[-]")
	} else {
		ui.helpBar.SetText("[#888888]Space Toggle   Tab/↑↓ Navigate   r Regenerate   y Copy   q Quit[-]")
	}
}

func passwordStrength(entropy float64) (string, string) {
	switch {
	case entropy < 28:
		return "Very Weak", "#ff5555"
	case entropy < 36:
		return "Weak", "#ff6655"
	case entropy < 60:
		return "Fair", "#ffb86c"
	case entropy < 128:
		return "Strong", "#50fa7b"
	default:
		return "Very Strong", "#50fa7b::b"
	}
}

func fmtAtoi(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
