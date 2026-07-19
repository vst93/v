package plugin_jv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// OrderedMap is a map that preserves key insertion order.
// JSON objects are decoded into OrderedMap so that formatted output
// keeps the original key order instead of alphabetical sort.
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		keys:   make([]string, 0),
		values: make(map[string]interface{}),
	}
}

// Set stores a key-value pair. If the key is new it is appended to the
// ordered key list; if it already exists only the value is replaced.
func (m *OrderedMap) Set(key string, value interface{}) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get returns the value for key, or nil if not found.
func (m *OrderedMap) Get(key string) interface{} {
	if m == nil {
		return nil
	}
	return m.values[key]
}

// Has reports whether the key exists.
func (m *OrderedMap) Has(key string) bool {
	if m == nil {
		return false
	}
	_, ok := m.values[key]
	return ok
}

// Keys returns the ordered list of keys.
func (m *OrderedMap) Keys() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

// Len returns the number of keys.
func (m *OrderedMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// MarshalJSON implements json.Marshaler so OrderedMap serializes as a
// JSON object with keys in insertion order.
func (m OrderedMap) MarshalJSON() ([]byte, error) {
	if m.Len() == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		// marshal key as JSON string
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON implements json.Unmarshaler. It decodes a JSON object
// into an OrderedMap, preserving key order. Nested objects are also
// decoded as OrderedMap so order is preserved recursively.
func (m *OrderedMap) UnmarshalJSON(data []byte) error {
	if m.values == nil {
		m.values = make(map[string]interface{})
	}
	if m.keys == nil {
		m.keys = make([]string, 0)
	}

	// Use json.Decoder with UseNumber to preserve number precision.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	// Read opening delimiter.
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("expected '{', got %v", tok)
	}

	for dec.More() {
		// Read key.
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("expected string key, got %v", tok)
		}

		// Read value via decodeValue to handle nested structures.
		val, err := decodeValue(dec)
		if err != nil {
			return err
		}
		m.Set(key, val)
	}

	// Read closing delimiter.
	if _, err := dec.Token(); err != nil {
		return err
	}
	return nil
}

// decodeValue reads a single JSON value from the decoder, preserving
// object key order (OrderedMap) and number precision (json.Number).
func decodeValue(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	if delim, ok := tok.(json.Delim); ok {
		switch delim {
		case '{':
			om := NewOrderedMap()
			for dec.More() {
				ktok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key := ktok.(string)
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				om.Set(key, val)
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return om, nil
		case '[':
			arr := make([]interface{}, 0)
			for dec.More() {
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		}
	}

	return tok, nil
}

// SortKeys recursively sorts keys of the OrderedMap and any nested
// OrderedMaps. This is used for the --sort flag.
func (m *OrderedMap) SortKeys() {
	if m == nil {
		return
	}
	sort.Strings(m.keys)
	for _, k := range m.keys {
		switch v := m.values[k].(type) {
		case *OrderedMap:
			v.SortKeys()
		case []interface{}:
			for i := range v {
				sortKeysInValue(v[i])
			}
		}
	}
}

func sortKeysInValue(v interface{}) {
	switch v := v.(type) {
	case *OrderedMap:
		v.SortKeys()
	case []interface{}:
		for i := range v {
			sortKeysInValue(v[i])
		}
	}
}

// String returns a simple (non-colored) string representation.
func (m *OrderedMap) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}

// KeyPath returns a dotted path string for display purposes.
func keyPathString(path []string) string {
	return strings.Join(path, ".")
}
