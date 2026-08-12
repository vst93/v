package plugin_codec

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
	"github.com/gookit/color"
)

type Codec struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (c *Codec) Init() error {
	c.name = "codec"
	c.version = "1.0.0"
	c.description = "Encode/Decode text: base64, base32, url, hex, html, unicode"
	c.command = "codec"
	c.args = map[string]string{
		"-b64":   "Base64 encode",
		"-b64d":  "Base64 decode",
		"-b32":   "Base32 encode",
		"-b32d":  "Base32 decode",
		"-url":   "URL encode (percent-encoding)",
		"-urld":  "URL decode",
		"-hex":   "Hex encode",
		"-hexd":  "Hex decode",
		"-html":  "HTML escape",
		"-htmld": "HTML unescape",
		"-uni":   "Unicode escape (non-ASCII to \\uXXXX)",
		"-unid":  "Unicode unescape (\\uXXXX to UTF-8)",
		"-file":  "Read from file path instead of clipboard/arg",
		"-clip":  "Read from clipboard (the default when nothing else is given)",
		"-pipe":  "Read from pipe/stdin (auto-detected)",
		"-out":   "Write the result to a file instead of stdout",
		"-copy":  "Copy result to clipboard",
		"-tui":   "Interactive TUI (default when no mode given)",
		"-h":     "Show help",
	}
	c.author = "vst"
	return nil
}

func (c *Codec) GetName() string            { return c.name }
func (c *Codec) GetVersion() string         { return c.version }
func (c *Codec) GetDescription() string     { return c.description }
func (c *Codec) GetCommand() string         { return c.command }
func (c *Codec) GetArgs() map[string]string { return c.args }
func (c *Codec) GetAuthor() string          { return c.author }
func (c *Codec) GetAliases() []string       { return []string{"cc"} }
func (c *Codec) Stop() error                { return nil }

func (c *Codec) Run(args []string) error {
	var (
		mode     string
		filePath string
		outPath  string
		pipeData string
		hasPipe  bool
		useClip  bool
		copyCB   bool
		forceTUI bool
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
		case "-out":
			if i+1 < len(args) {
				outPath = args[i+1]
				i++
			}
		case "-clip":
			useClip = true
		case "-copy":
			copyCB = true
		case "-tui":
			forceTUI = true
		case "-pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
			c.printHelp()
			return nil
		default:
			if !strings.HasPrefix(arg, "-") && !hasInput {
				input = arg
				hasInput = true
			}
		}
	}

	// No mode at all -> interactive TUI, seeded with whatever input was given.
	if forceTUI || (mode == "" && !hasPipe && filePath == "" && !hasInput && !useClip) {
		seed := ""
		switch {
		case hasPipe:
			seed = pipeData
		case hasInput:
			seed = input
		}
		return runTUI(seed)
	}

	if mode == "" {
		c.printHelp()
		return nil
	}

	// Get input. Priority: pipe > file > argument > clipboard.
	var inputData string
	switch {
	case hasPipe:
		inputData = pipeData
	case filePath != "":
		data, err := os.ReadFile(expandHome(filePath))
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		inputData = string(data)
	case hasInput:
		inputData = input
	default:
		clip, err := clipboard.ReadAll()
		if err != nil || clip == "" {
			return fmt.Errorf("no input provided: use -file, -pipe, pass text as argument, or copy to clipboard")
		}
		inputData = clip
	}

	result, err := doTransform(mode, inputData)
	if err != nil {
		return err
	}

	if outPath != "" {
		if err := os.WriteFile(expandHome(outPath), []byte(result+"\n"), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
		fmt.Printf("✅ Written to %s\n", outPath)
	} else {
		fmt.Println(result)
	}

	if copyCB {
		if err := clipboard.WriteAll(result); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✅ Copied to clipboard")
	}

	return nil
}

// doTransform applies a single codec mode. Shared by the CLI and the TUI.
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

func (c *Codec) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>codec - Encode/Decode Utility v%s</>\n\n", c.version)
	color.Println("Usage: v codec [mode] [input] [options]")
	color.Println("<gray>Short command: cc</>")
	color.Println()
	color.Println("<fg=magenta;op=bold>Modes:</>")
	color.Println("  <green>-b64</>    Base64 encode      <green>-b64d</>   Base64 decode")
	color.Println("  <green>-b32</>    Base32 encode      <green>-b32d</>   Base32 decode")
	color.Println("  <green>-url</>    URL encode         <green>-urld</>   URL decode")
	color.Println("  <green>-hex</>    Hex encode         <green>-hexd</>   Hex decode")
	color.Println("  <green>-html</>   HTML escape        <green>-htmld</>  HTML unescape")
	color.Println("  <green>-uni</>    Unicode escape     <green>-unid</>   Unicode unescape")
	color.Println()
	color.Println("<gray>I/O: -pipe (auto) · -file <path> · -clip · -out <path> · -copy · -tui · -h</>")
	color.Println("<gray>     No mode launches the interactive TUI.</>")
	color.Println()
	color.Println("<fg=magenta;op=bold>Examples:</>")
	color.Println(`  v codec <green>-b64</> "Hello World"`)
	color.Println(`  echo "Hello" | v codec <green>-b64</> <green>-copy</>`)
	color.Println(`  v codec <green>-b64d</> "SGVsbG8gV29ybGQ="`)
	color.Println("<gray>--------------------------------------------------</>")
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}

// escapeUnicode replaces all non-ASCII runes with \uXXXX escapes,
// using surrogate pairs for code points above 0xFFFF.
func escapeUnicode(input string) string {
	var sb strings.Builder
	for _, r := range input {
		if r < 128 {
			sb.WriteRune(r)
		} else if r <= 0xFFFF {
			fmt.Fprintf(&sb, `\u%04x`, r)
		} else {
			// surrogate pair
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			fmt.Fprintf(&sb, `\u%04x\u%04x`, high, low)
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
