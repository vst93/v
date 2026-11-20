package plugin_qr

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"v/internal/helputil"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/term"
)

type QR struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (q *QR) Init() error {
	q.name = "qr"
	q.version = "0.0.1"
	q.description = "Generate QR codes in the terminal or as PNG images"
	q.command = "qr"
	q.author = "vst"
	q.args = map[string]string{
		"-terminal":        "Display a QR image inside a supported terminal",
		"-protocol <name>": "Terminal image protocol: auto, iterm2, kitty, text (implies -terminal)",
		"-image":           "Open a PNG in the default image viewer",
		"-save":            "Save a PNG to the system Downloads directory",
		"-out <path>":      "Save a PNG to a custom path (overrides -save)",
		"-size <px>":       "PNG width and height in pixels (default: 256, max: 4096)",
		"-file <path>":     "Read text from a file",
		"-clip":            "Read text from the clipboard (default without input)",
		"-pipe":            "Read text from pipe/stdin (auto-detected)",
		"-raw":             "Print terminal QR without ANSI colors",
		"-h":               "Show help",
	}
	return nil
}

func (q *QR) GetName() string            { return q.name }
func (q *QR) GetVersion() string         { return q.version }
func (q *QR) GetDescription() string     { return q.description }
func (q *QR) GetCommand() string         { return q.command }
func (q *QR) GetArgs() map[string]string { return q.args }
func (q *QR) GetAuthor() string          { return q.author }
func (q *QR) GetAliases() []string       { return []string{"qrcode"} }
func (q *QR) Stop() error                { return nil }

type qrOptions struct {
	input      string
	hasPipe    bool
	file       string
	positional []string
	image      bool
	terminal   bool
	protocol   string
	save       bool
	out        string
	size       int
	raw        bool
}

func parseQRArgs(args []string) (qrOptions, bool, error) {
	opts := qrOptions{size: 256, protocol: "auto"}
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "-image":
			opts.image = true
		case "-terminal":
			opts.terminal = true
		case "-save":
			opts.save = true
		case "-raw":
			opts.raw = true
		case "-clip":
			// Clipboard is the fallback after pipe, file and positional input.
		case "-file", "-out", "-size", "-pipe", "-protocol":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("%s requires a value", arg)
			}
			i++
			value := args[i]
			if arg != "-pipe" && (value == "" || strings.HasPrefix(value, "-")) {
				return opts, false, fmt.Errorf("invalid value %q for %s", value, arg)
			}
			switch arg {
			case "-protocol":
				switch value {
				case "auto", "iterm2", "kitty", "text":
					opts.protocol, opts.terminal = value, true
				default:
					return opts, false, fmt.Errorf("-protocol must be auto, iterm2, kitty, or text")
				}
			case "-file":
				opts.file = value
			case "-out":
				opts.out = value
			case "-pipe":
				opts.input, opts.hasPipe = value, true
			case "-size":
				size, err := strconv.Atoi(value)
				if err != nil || size < 1 || size > 4096 {
					return opts, false, fmt.Errorf("-size must be an integer between 1 and 4096")
				}
				opts.size = size
			}
		case "-h", "-help", "--help":
			return opts, true, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, false, fmt.Errorf("unknown option %q; run `v qr -h` for help", arg)
			}
			opts.positional = append(opts.positional, arg)
		}
	}
	return opts, false, nil
}

func resolveQRInput(opts qrOptions) (string, error) {
	if opts.hasPipe {
		return opts.input, nil
	}
	if opts.file != "" {
		data, err := os.ReadFile(expandHome(opts.file))
		if err != nil {
			return "", fmt.Errorf("read input file: %w", err)
		}
		return string(data), nil
	}
	if len(opts.positional) > 0 {
		return strings.Join(opts.positional, " "), nil
	}
	text, err := clipboard.ReadAll()
	if err != nil {
		return "", fmt.Errorf("read clipboard (or pass text as an argument, with -file, or through a pipe): %w", err)
	}
	return text, nil
}

func (q *QR) Run(args []string) error {
	opts, help, err := parseQRArgs(args)
	if err != nil {
		return err
	}
	if help {
		q.printHelp()
		return nil
	}
	input, err := resolveQRInput(opts)
	if err != nil {
		return err
	}
	if input == "" {
		return fmt.Errorf("no text to encode; pass text, use -file, pipe it in, or copy text to the clipboard")
	}
	code, err := qrcode.New(input, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("generate QR code: %w", err)
	}
	protocol := "text"
	if opts.terminal {
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		protocol = terminalImageProtocol(opts.protocol, opts.raw, isTTY)
		if protocol == "text" && isTTY && !opts.raw && opts.protocol == "auto" {
			fmt.Fprintln(os.Stderr, "Terminal image support not detected; showing a text QR. Use -protocol iterm2 or -protocol kitty on a compatible terminal.")
		}
	}
	if !opts.image && !opts.save && opts.out == "" && protocol == "text" {
		_, err := fmt.Print(terminalQR(code, opts.raw))
		return err
	}
	if minSize := len(code.Bitmap()); opts.size < minSize {
		return fmt.Errorf("-size must be at least %d pixels for this content", minSize)
	}
	data, err := code.PNG(opts.size)
	if err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}
	if opts.image || opts.save || opts.out != "" {
		path, err := saveQRImage(data, opts)
		if err != nil {
			return err
		}
		if opts.save || opts.out != "" {
			fmt.Printf("✅ Saved QR code to %s\n", path)
		}
		if opts.image {
			if err := openImage(path); err != nil {
				return fmt.Errorf("PNG generated at %s, but could not open the image viewer: %w", path, err)
			}
			fmt.Printf("📁 Opened QR image: %s\n", path)
		}
	}
	if opts.terminal {
		output := terminalQR(code, opts.raw)
		if protocol != "text" {
			output, err = terminalImageSequence(data, opts.size, protocol, os.Getenv("TMUX") != "")
			if err != nil {
				return err
			}
		}
		if _, err := fmt.Print(output); err != nil {
			return fmt.Errorf("display terminal image: %w", err)
		}
	}
	return nil
}

func terminalQR(code *qrcode.QRCode, raw bool) string {
	if raw || !color.Enable || !color.SupportColor() {
		// The light blocks form the quiet zone on a typical dark terminal.
		return code.ToSmallString(false)
	}
	// Fix both colors so the code stays scannable on light and dark terminals.
	style := color.Style{color.FgBlack, color.BgWhite}
	var out strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(code.ToSmallString(true), "\n"), "\n") {
		out.WriteString(style.Sprint(line))
		out.WriteByte('\n')
	}
	return out.String()
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, "~\\") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

func (q *QR) printHelp() {
	helputil.New("qr - QR Code Generator", q.version, "Encode text or a URL as a terminal QR code or a PNG image.").
		WithShortCommand("qrcode").
		AddUsage("v qr [options] [text]", "printf '%s' 'hello' | v qr [options]").
		AddMode("(default)", "Print a compact QR code in the terminal").
		AddMode("-terminal", "Display an image inside the terminal (text fallback if unsupported)").
		AddMode("-image", "Open a PNG in the default image viewer (temporary file)").
		AddMode("-save", "Save a PNG to the system Downloads directory").
		AddOption("-out <path>", "Save a PNG to a custom path; overrides the Downloads destination").
		AddOption("-size <px>", "PNG width and height (default: 256, max: 4096; must fit the QR code)").
		AddOption("-protocol <name>", "auto (default), iterm2, kitty, text; implies -terminal").
		AddOption("-file <path>", "Read text from a file").
		AddOption("-clip", "Read clipboard text (default without input)").
		AddOption("-pipe", "Read pipe/stdin text (auto-detected)").
		AddOption("-raw", "Terminal QR without ANSI colors").
		AddOption("-h, -help, --help", "Show help").
		AddExample(`v qr "https://example.com"`, "Print a QR code in the terminal").
		AddExample(`v qr "你好" -terminal`, "Display a QR image inside the terminal").
		AddExample(`v qr "hello" -terminal -save`, "Display in the terminal and save to Downloads").
		AddExample(`v qr "你好" -image`, "Preview a PNG").
		AddExample(`v qr "hello" -save -size 512`, "Save a 512×512 PNG to Downloads").
		AddExample(`v qr "hello" -save -image`, "Save and preview the same PNG").
		AddExample(`v qr -file text.txt -out ./qr.png`, "Encode file contents and save to a custom path").
		WithFooter("Input priority: pipe > -file > argument > clipboard. Text is encoded verbatim, including newlines.\nTerminal images: iTerm2 / WezTerm / VS Code; Kitty / Ghostty. In VS Code, enable terminal.integrated.enableImages.\n-raw and redirected stdout use text. In tmux, choose -protocol explicitly and enable allow-passthrough.\nPNG output keeps a white quiet zone. System image previews remain in the system temp directory.").
		Print()
}
