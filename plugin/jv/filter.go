package plugin_jv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// evalFilter evaluates a simple path expression against decoded JSON.
//
// Supported syntax (chained in any order):
//
//	.key        object key (bare keys; use ["key"] for special characters)
//	["key"]     object key, quoted form
//	[0]         array index (negative counts from the end, e.g. [-1])
//	.length     length of an array, object or string
//	.map(expr)  apply a sub-expression to every element of an array
//
// A leading "this" or "$" is accepted and ignored. An empty expression
// returns the root value unchanged.
func evalFilter(root interface{}, expr string) (interface{}, error) {
	expr = strings.TrimSpace(expr)
	expr = strings.TrimPrefix(expr, "this")
	expr = strings.TrimPrefix(expr, "$")
	if expr == "" {
		return root, nil
	}

	cur := root
	i := 0
	for i < len(expr) {
		switch expr[i] {
		case '.':
			i++
			start := i
			for i < len(expr) && !strings.ContainsRune(".[()", rune(expr[i])) {
				i++
			}
			name := expr[start:i]
			if name == "" {
				return nil, fmt.Errorf("expected a key after '.' at %d", start)
			}
			if name == "length" && (i >= len(expr) || expr[i] != '(') {
				var err error
				cur, err = lengthOf(cur)
				if err != nil {
					return nil, err
				}
				continue
			}
			if name == "map" && i < len(expr) && expr[i] == '(' {
				sub, next, err := readParen(expr, i)
				if err != nil {
					return nil, err
				}
				cur, err = mapOver(cur, sub)
				if err != nil {
					return nil, err
				}
				i = next
				continue
			}
			om, ok := cur.(*OrderedMap)
			if !ok {
				return nil, fmt.Errorf("cannot read key %q from %s", name, TypeString(cur))
			}
			if !om.Has(name) {
				return nil, fmt.Errorf("key %q does not exist", name)
			}
			cur = om.Get(name)

		case '[':
			i++
			if i < len(expr) && expr[i] == '"' {
				key, next, err := readQuotedIndex(expr, i)
				if err != nil {
					return nil, err
				}
				om, ok := cur.(*OrderedMap)
				if !ok {
					return nil, fmt.Errorf("cannot read key %s from %s", key, TypeString(cur))
				}
				if !om.Has(key) {
					return nil, fmt.Errorf("key %s does not exist", key)
				}
				cur = om.Get(key)
				i = next
				continue
			}
			idx, next, err := readIntIndex(expr, i)
			if err != nil {
				return nil, err
			}
			arr, ok := cur.([]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot index %s", TypeString(cur))
			}
			if idx < 0 {
				idx += len(arr)
			}
			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("index %d out of range (length %d)", idx, len(arr))
			}
			cur = arr[idx]
			i = next

		default:
			return nil, fmt.Errorf("unexpected %q at %d (expressions start with . or [)", string(expr[i]), i)
		}
	}
	return cur, nil
}

// readParen extracts the parenthesised sub-expression starting at the '('
// at position i, returning the inner text and the position after ')'.
func readParen(expr string, i int) (string, int, error) {
	depth := 1
	j := i + 1
	for j < len(expr) && depth > 0 {
		switch expr[j] {
		case '(':
			depth++
		case ')':
			depth--
		}
		j++
	}
	if depth != 0 {
		return "", 0, fmt.Errorf("unclosed '(' in map(...)")
	}
	return expr[i+1 : j-1], j, nil
}

// readQuotedIndex parses ["key"] starting just after '['.
func readQuotedIndex(expr string, i int) (string, int, error) {
	j := i + 1
	for j < len(expr) {
		if expr[j] == '\\' {
			j += 2
			continue
		}
		if expr[j] == '"' {
			break
		}
		j++
	}
	if j >= len(expr) {
		return "", 0, fmt.Errorf("unclosed quote in index")
	}
	var key string
	if err := json.Unmarshal([]byte(expr[i:j+1]), &key); err != nil {
		return "", 0, fmt.Errorf("bad quoted key: %v", err)
	}
	if j+1 >= len(expr) || expr[j+1] != ']' {
		return "", 0, fmt.Errorf("expected ']' after quoted key")
	}
	return key, j + 2, nil
}

// readIntIndex parses [123] or [-1] starting just after '['.
func readIntIndex(expr string, i int) (int, int, error) {
	j := i
	for j < len(expr) && expr[j] != ']' {
		j++
	}
	if j >= len(expr) {
		return 0, 0, fmt.Errorf("unclosed '['")
	}
	n, err := strconv.Atoi(strings.TrimSpace(expr[i:j]))
	if err != nil {
		return 0, 0, fmt.Errorf("bad array index %q", expr[i:j])
	}
	return n, j + 1, nil
}

// lengthOf returns the length of an array, object or string as a
// json.Number so it renders like a regular JSON number.
func lengthOf(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		return json.Number(strconv.Itoa(len(val))), nil
	case *OrderedMap:
		return json.Number(strconv.Itoa(val.Len())), nil
	case string:
		return json.Number(strconv.Itoa(utf8.RuneCountInString(val))), nil
	default:
		return nil, fmt.Errorf("%s has no length", TypeString(v))
	}
}

// mapOver applies a sub-expression to every element of an array.
func mapOver(v interface{}, sub string) (interface{}, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("map() requires an array, got %s", TypeString(v))
	}
	out := make([]interface{}, 0, len(arr))
	for i, item := range arr {
		res, err := evalFilter(item, sub)
		if err != nil {
			return nil, fmt.Errorf("map element [%d]: %w", i, err)
		}
		out = append(out, res)
	}
	return out, nil
}
