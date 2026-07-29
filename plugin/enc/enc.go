package plugin_enc

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
)

type Enc struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (e *Enc) Init() error {
	e.name = "enc"
	e.version = "0.0.1"
	e.description = "Encode/Decode text: base64, url, hex, html, base32"
	e.command = "enc"
	e.args = map[string]string{
		"-b64":   "Base64 encode",
		"-b64d":  "Base64 decode",
		"-b32":   "Base32 encode",
		"-b32d":  "Base32 decode",
		"-url":   "URL encode",
		"-urld":  "URL decode",
		"-hex":   "Hex encode",
		"-hexd":  "Hex decode",
		"-html":  "HTML escape",
		"-htmld": "HTML unescape",
		"-uni":   "Unicode escape (non-ASCII to \\uXXXX)",
		"-unid":  "Unicode unescape (\\uXXXX to UTF-8)",
		"-file":  "Read from file path instead of clipboard/arg",
		"-c":     "Copy result to clipboard",
		"-pipe":  "Read from pipe/stdin (auto-detected)",
		"-h":     "Show help",
	}
	e.author = "vst"
	return nil
}

func (e *Enc) GetName() string            { return e.name }
func (e *Enc) GetVersion() string         { return e.version }
func (e *Enc) GetDescription() string     { return e.description }
func (e *Enc) GetCommand() string         { return e.command }
func (e *Enc) GetArgs() map[string]string { return e.args }
func (e *Enc) GetAuthor() string          { return e.author }
func (e *Enc) Stop() error                { return nil }

func (e *Enc) Run(args []string) error {
	var (
		mode     string
		filePath string
		pipeData string
		hasPipe  bool
		copyCB   bool
		input    string
		hasInput bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-b64":
			mode = "base64enc"
		case "-b64d":
			mode = "base64dec"
		case "-b32":
			mode = "base32enc"
		case "-b32d":
			mode = "base32dec"
		case "-url":
			mode = "urlenc"
		case "-urld":
			mode = "urldec"
		case "-hex":
			mode = "hexenc"
		case "-hexd":
			mode = "hexdec"
		case "-html":
			mode = "htmlesc"
		case "-htmld":
			mode = "htmlunesc"
		case "-uni":
			mode = "unienc"
		case "-unid":
			mode = "unidec"
		case "-file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		case "-c":
			copyCB = true
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "--help":
			e.printHelp()
			return nil
		default:
			if !strings.HasPrefix(arg, "-") && !hasInput {
				input = arg
				hasInput = true
			}
		}
	}

	if mode == "" && !hasPipe && filePath == "" && !hasInput {
		// No args at all -> launch TUI
		return runTUI()
	}

	if mode == "" {
		e.printHelp()
		return nil
	}

	// Get input
	var inputData string
	switch {
	case hasPipe:
		inputData = pipeData
	case filePath != "":
		filePath = expandHome(filePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		inputData = string(data)
	case hasInput:
		inputData = input
	default:
		// Try clipboard
		clip, err := clipboard.ReadAll()
		if err != nil || clip == "" {
			return fmt.Errorf("no input provided: use -file, -pipe, pass text as argument, or copy to clipboard")
		}
		inputData = clip
	}

	result, err := e.transform(mode, inputData)
	if err != nil {
		return err
	}

	fmt.Println(result)

	if copyCB {
		if err := clipboard.WriteAll(result); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✅ Copied to clipboard")
	}

	return nil
}

func (e *Enc) transform(mode, input string) (string, error) {
	return doTransform(mode, input)
}

// doTransform is the standalone transform function (also used by the TUI).
func doTransform(mode, input string) (string, error) {
	switch mode {
	case "base64enc":
		return base64.StdEncoding.EncodeToString([]byte(input)), nil
	case "base64dec":
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input))
		if err != nil {
			return "", fmt.Errorf("base64 decode failed: %w", err)
		}
		return string(data), nil
	case "base32enc":
		return base32.StdEncoding.EncodeToString([]byte(input)), nil
	case "base32dec":
		data, err := base32.StdEncoding.DecodeString(strings.TrimSpace(input))
		if err != nil {
			return "", fmt.Errorf("base32 decode failed: %w", err)
		}
		return string(data), nil
	case "urlenc":
		return url.QueryEscape(input), nil
	case "urldec":
		decoded, err := url.QueryUnescape(input)
		if err != nil {
			return "", fmt.Errorf("URL decode failed: %w", err)
		}
		return decoded, nil
	case "hexenc":
		return hex.EncodeToString([]byte(input)), nil
	case "hexdec":
		data, err := hex.DecodeString(strings.TrimSpace(input))
		if err != nil {
			return "", fmt.Errorf("hex decode failed: %w", err)
		}
		return string(data), nil
	case "htmlesc":
		return html.EscapeString(input), nil
	case "htmlunesc":
		return html.UnescapeString(input), nil
	case "unienc":
		return escapeUnicode(input), nil
	case "unidec":
		return unescapeUnicode(input), nil
	default:
		return "", fmt.Errorf("unknown mode: %s", mode)
	}
}

func (e *Enc) printHelp() {
	fmt.Println(`📦 enc - Encode/Decode Utility

Usage: v enc <mode> [input] [options]

Modes:
  -b64      Base64 encode
  -b64d     Base64 decode
  -b32      Base32 encode
  -b32d     Base32 decode
  -url      URL encode (percent-encoding)
  -urld     URL decode
  -hex      Hex encode
  -hexd     Hex decode
  -html     HTML escape
  -htmld    HTML unescape
  -uni      Unicode escape (non-ASCII to \uXXXX)
  -unid     Unicode unescape (\uXXXX to UTF-8)

Input sources (priority: pipe > file > argument > clipboard):
  (text)     Pass text directly as argument
  -file      Read from file
  -pipe      Read from stdin/pipe (auto-detected)
  (none)     Read from clipboard

Options:
  -c         Copy result to clipboard
  -h         Show this help

Examples:
  v enc -b64 "Hello World"
  echo "Hello" | v enc -b64 -c
  v enc -b64d "SGVsbG8gV29ybGQ="
  v enc -url "hello world&foo=bar"
  v enc -hexd "48656c6c6f"
  v enc -uni "你好"
  v enc -unid "\u4f60\u597d"`)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
}

// escapeUnicode replaces all non-ASCII runes with \uXXXX escapes,
// using surrogate pairs for code points above 0xFFFF.
func escapeUnicode(input string) string {
	var sb strings.Builder
	for _, r := range input {
		if r < 128 {
			sb.WriteRune(r)
		} else if r <= 0xFFFF {
			sb.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else {
			// surrogate pair
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			sb.WriteString(fmt.Sprintf(`\u%04x\u%04x`, high, low))
		}
	}
	return sb.String()
}

// unescapeUnicode converts \uXXXX sequences (including surrogate pairs)
// back to UTF-8 characters.
func unescapeUnicode(input string) string {
	if !strings.Contains(input, `\u`) {
		return input
	}
	var sb strings.Builder
	i := 0
	runes := []rune(input)
	for i < len(runes) {
		if runes[i] == '\\' && i+1 < len(runes) && runes[i+1] == 'u' {
			if i+5 < len(runes) {
				hex := string(runes[i+2 : i+6])
				if n, err := strconv.ParseInt(hex, 16, 32); err == nil {
					r := rune(n)
					// Check for high surrogate
					if r >= 0xD800 && r <= 0xDBFF && i+11 < len(runes) &&
						runes[i+6] == '\\' && runes[i+7] == 'u' {
						lowHex := string(runes[i+8 : i+12])
						if low, err := strconv.ParseInt(lowHex, 16, 32); err == nil {
							if low >= 0xDC00 && low <= 0xDFFF {
								r = 0x10000 + (r-0xD800)<<10 + (rune(low) - 0xDC00)
								sb.WriteRune(r)
								i += 12
								continue
							}
						}
					}
					sb.WriteRune(r)
					i += 6
					continue
				}
			}
		}
		sb.WriteRune(runes[i])
		i++
	}
	return sb.String()
}
