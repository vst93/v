package plugin_jv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gookit/color"
)

// Color styles for JSON pretty-print output.
var (
	colorKey     = color.New(color.FgCyan)
	colorString  = color.New(color.FgGreen)
	colorNumber  = color.New(color.FgYellow)
	colorBool    = color.New(color.FgMagenta)
	colorNull    = color.New(color.FgRed, color.OpBold)
	colorPunct   = color.New(color.FgDarkGray)
	colorIndex   = color.New(color.FgBlue, color.OpBold)
	colorBracket = color.New(color.FgBlue)
)

// FormatJSON renders a parsed JSON value (from DecodeJSON) into a
// human-readable, colored, indented string with the given indent size.
// When colorize is false the output is plain (no ANSI codes).
func FormatJSON(v interface{}, indent int, colorize bool) string {
	var sb strings.Builder
	renderJSON(&sb, v, 0, indent, colorize, false)
	return sb.String()
}

// FormatJSONEscape renders like FormatJSON without colors, optionally
// escaping all non-ASCII characters as \uXXXX sequences.
func FormatJSONEscape(v interface{}, indent int, escape bool) string {
	var sb strings.Builder
	renderJSON(&sb, v, 0, indent, false, escape)
	return sb.String()
}

// CompactJSON renders a parsed JSON value into a single line with no
// extra whitespace. When escape is true, non-ASCII characters are
// escaped as \uXXXX sequences in string values (matching encoding/json
// default behaviour).
func CompactJSON(v interface{}, escape bool) string {
	var sb strings.Builder
	renderCompact(&sb, v, escape)
	return sb.String()
}

// renderJSON is the recursive worker for FormatJSON.
func renderJSON(sb *strings.Builder, v interface{}, depth, indent int, colorize, escape bool) {
	switch val := v.(type) {
	case *OrderedMap:
		if val == nil || val.Len() == 0 {
			writePunct(sb, "{}", colorize)
			return
		}
		writePunct(sb, "{", colorize)
		sb.WriteByte('\n')
		keys := val.Keys()
		for i, k := range keys {
			writeIndent(sb, depth+1, indent)
			writeKey(sb, k, colorize, escape)
			writePunct(sb, ": ", colorize)
			renderJSON(sb, val.Get(k), depth+1, indent, colorize, escape)
			if i < len(keys)-1 {
				writePunct(sb, ",", colorize)
			}
			sb.WriteByte('\n')
		}
		writeIndent(sb, depth, indent)
		writePunct(sb, "}", colorize)

	case []interface{}:
		if len(val) == 0 {
			writePunct(sb, "[]", colorize)
			return
		}
		writePunct(sb, "[", colorize)
		sb.WriteByte('\n')
		for i, item := range val {
			writeIndent(sb, depth+1, indent)
			renderJSON(sb, item, depth+1, indent, colorize, escape)
			if i < len(val)-1 {
				writePunct(sb, ",", colorize)
			}
			sb.WriteByte('\n')
		}
		writeIndent(sb, depth, indent)
		writePunct(sb, "]", colorize)

	case string:
		writeString(sb, val, colorize, escape)

	case json.Number:
		writeNumber(sb, val.String(), colorize)

	case float64, float32, int, int64:
		writeNumber(sb, fmt.Sprintf("%v", val), colorize)

	case bool:
		writeBool(sb, val, colorize)

	case nil:
		writeNull(sb, colorize)

	default:
		// Fallback for any other type.
		b, _ := json.Marshal(val)
		sb.Write(b)
	}
}

// renderCompact produces a minified single-line representation.
func renderCompact(sb *strings.Builder, v interface{}, escape bool) {
	switch val := v.(type) {
	case *OrderedMap:
		if val == nil || val.Len() == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteByte('{')
		keys := val.Keys()
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			b, _ := json.Marshal(k)
			sb.Write(b)
			sb.WriteByte(':')
			renderCompact(sb, val.Get(k), escape)
		}
		sb.WriteByte('}')

	case []interface{}:
		if len(val) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				sb.WriteByte(',')
			}
			renderCompact(sb, item, escape)
		}
		sb.WriteByte(']')

	case string:
		if escape {
			// Escape non-ASCII to \uXXXX, keep JSON control escapes
			sb.WriteString(escapeJSONString(val))
		} else {
			writeStringRaw(sb, val)
		}

	case json.Number:
		sb.WriteString(val.String())

	case bool:
		if val {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}

	case nil:
		sb.WriteString("null")

	default:
		b, _ := json.Marshal(val)
		sb.Write(b)
	}
}

// --- Coloured writers ---

func writeKey(sb *strings.Builder, key string, colorize, escape bool) {
	q := quoteString(key)
	if escape {
		q = escapeJSONString(key)
	}
	if colorize {
		sb.WriteString(colorKey.Render(q))
	} else {
		sb.WriteString(q)
	}
}

func writeString(sb *strings.Builder, s string, colorize bool, escape bool) {
	quoted := quoteString(s)
	if escape {
		// Escape non-ASCII as \uXXXX while keeping <>& as-is.
		quoted = escapeJSONString(s)
	}
	if colorize {
		sb.WriteString(colorString.Render(quoted))
	} else {
		sb.WriteString(quoted)
	}
}

// writeStringRaw writes a JSON-safe quoted string without escaping
// non-ASCII characters. Used by compact mode with escape=false.
func writeStringRaw(sb *strings.Builder, s string) {
	sb.WriteString(quoteString(s))
}

func writeNumber(sb *strings.Builder, n string, colorize bool) {
	if colorize {
		sb.WriteString(colorNumber.Render(n))
	} else {
		sb.WriteString(n)
	}
}

func writeBool(sb *strings.Builder, b bool, colorize bool) {
	s := "false"
	if b {
		s = "true"
	}
	if colorize {
		sb.WriteString(colorBool.Render(s))
	} else {
		sb.WriteString(s)
	}
}

func writeNull(sb *strings.Builder, colorize bool) {
	if colorize {
		sb.WriteString(colorNull.Render("null"))
	} else {
		sb.WriteString("null")
	}
}

func writePunct(sb *strings.Builder, s string, colorize bool) {
	if colorize {
		sb.WriteString(colorPunct.Render(s))
	} else {
		sb.WriteString(s)
	}
}

func writeIndent(sb *strings.Builder, depth, indent int) {
	for i := 0; i < depth*indent; i++ {
		sb.WriteByte(' ')
	}
}

// quoteString produces a JSON-quoted string, escaping control
// characters but preserving non-ASCII UTF-8 (e.g. Chinese) as-is.
func quoteString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '	':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// escapeJSONString produces a JSON-quoted string where all non-ASCII
// runes are escaped as \uXXXX (or surrogate pairs), suitable for
// ASCII-only JSON output. JSON control characters are also escaped.
func escapeJSONString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '	':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else if r < 128 {
				sb.WriteRune(r)
			} else if r <= 0xFFFF {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				// Surrogate pair for code points > 0xFFFF
				r -= 0x10000
				high := 0xD800 + (r >> 10)
				low := 0xDC00 + (r & 0x3FF)
				sb.WriteString(fmt.Sprintf(`\u%04x\u%04x`, high, low))
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// --- Escape / Unescape utilities ---

// EscapeUnicode replaces all non-ASCII runes with \uXXXX escapes,
// producing output compatible with standard JSON tools that expect
// ASCII-only output (like some older JSON validators).
func EscapeUnicode(input string) string {
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

// UnescapeUnicode converts \uXXXX sequences back to UTF-8 characters.
func UnescapeUnicode(input string) string {
	// Use json to unmarshal a quoted string, which handles \uXXXX,
	// \n, \t, \\, \" etc. all at once.
	if !strings.Contains(input, "\\u") {
		return input
	}
	// Try to decode as a JSON string literal (with quotes).
	s := `"` + strings.ReplaceAll(input, `"`, `\"`) + `"`
	var result string
	if err := json.Unmarshal([]byte(s), &result); err == nil {
		return result
	}

	// Fallback: manual \uXXXX decode.
	return manualUnescape(input)
}

func manualUnescape(input string) string {
	var sb strings.Builder
	i := 0
	for i < len(input) {
		if i+5 < len(input) && input[i] == '\\' && (input[i+1] == 'u' || input[i+1] == 'U') {
			hex := input[i+2 : i+6]
			if n, err := strconv.ParseInt(hex, 16, 32); err == nil {
				sb.WriteRune(rune(n))
				i += 6
				continue
			}
		}
		sb.WriteByte(input[i])
		i++
	}
	return sb.String()
}

// EscapeString escapes special characters in a JSON string value for
// safe embedding into JSON output (adds quotes).
func EscapeString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// IsValidJSON checks whether the input is parseable JSON.
func IsValidJSON(input string) bool {
	var v interface{}
	return json.Unmarshal([]byte(input), &v) == nil
}

// DecodeJSON parses JSON input preserving key order and number precision.
// Returns a value that may be *OrderedMap, []interface{}, or a primitive.
func DecodeJSON(input string) (interface{}, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	// Use a decoder with UseNumber to preserve number precision.
	dec := json.NewDecoder(strings.NewReader(input))
	dec.UseNumber()

	// Peek first byte to decide.
	if input[0] == '{' {
		om := NewOrderedMap()
		if err := dec.Decode(om); err != nil {
			return nil, err
		}
		return om, nil
	}

	if input[0] == '[' {
		// Decode into []interface{} with nested OrderedMaps.
		var arr []interface{}
		// We'll decode token by token to preserve nested object order.
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		delim, ok := tok.(json.Delim)
		if !ok || delim != '[' {
			return nil, fmt.Errorf("expected '['")
		}
		arr = make([]interface{}, 0)
		for dec.More() {
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		return arr, nil
	}

	// Primitive value (string, number, bool, null).
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// CountChildren returns the number of direct children of a JSON value.
// For objects it returns the key count, for arrays the element count,
// and 0 for primitives.
func CountChildren(v interface{}) int {
	switch val := v.(type) {
	case *OrderedMap:
		return val.Len()
	case []interface{}:
		return len(val)
	default:
		return 0
	}
}

// TypeString returns a human-readable type name for a JSON value.
func TypeString(v interface{}) string {
	switch v.(type) {
	case *OrderedMap:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// PreviewValue returns a short preview string of a value for display
// in collapsed tree nodes.
func PreviewValue(v interface{}) string {
	switch val := v.(type) {
	case *OrderedMap:
		return fmt.Sprintf("{%d}", val.Len())
	case []interface{}:
		return fmt.Sprintf("[%d]", len(val))
	case string:
		if utf8.RuneCountInString(val) > 30 {
			runes := []rune(val)
			return quoteString(string(runes[:30])) + "…"
		}
		return quoteString(val)
	case json.Number:
		return val.String()
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}
