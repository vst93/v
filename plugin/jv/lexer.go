package plugin_jv

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rivo/uniseg"
)

// segClass identifies the syntax class of a text segment so the
// renderer can map it to a color style.
type segClass uint8

const (
	clsPunct segClass = iota
	clsKey
	clsString
	clsNumber
	clsBool
	clsNull
)

// segment is a run of text sharing one syntax class.
type segment struct {
	text string
	cls  segClass
}

// lineKind classifies a display line.
type lineKind uint8

const (
	kindLeaf  lineKind = iota // no foldable container starts/ends here
	kindOpen                  // a multi-line container opens on this line
	kindClose                 // a multi-line container closes on this line
)

// displayLine is one annotated line of the document.
type displayLine struct {
	segs    []segment
	plain   string // the raw line text
	lower   string // lowercase plain for case-insensitive search
	width   int    // screen-cell width of plain
	kind    lineKind
	path    string // JSON path of the value starting on this line
	openID  int    // container id, valid when kind == kindOpen
	closeID int    // container id, valid when kind == kindClose
	parents []int  // enclosing container ids at line start, outermost first
}

// containerInfo records the line range of a foldable container.
type containerInfo struct {
	openLine  int
	closeLine int    // -1 while unclosed
	closeText string // closing bracket (+ optional comma) shown after the fold placeholder
}

// Object parsing modes for the lexer frame stack.
const (
	objExpectKey = iota
	objExpectColon
	objExpectValue
	objExpectComma
)

// lexFrame tracks one open container while scanning.
type lexFrame struct {
	id      int
	bracket rune
	path    string // JSON path of this container
	arrIdx  int    // array: next element index
	objMode int    // object: one of objExpect*
	key     string // object: pending key awaiting its value
	isArray bool
}

// lexer incrementally annotates document lines.
type lexer struct {
	lines      []displayLine
	containers []containerInfo
	stack      []lexFrame
}

// lexDocument scans raw text lines and produces display lines with
// syntax classes, fold ranges and JSON paths. It never modifies the
// text layout and tolerates invalid JSON (mid-edit states), producing
// best-effort highlighting.
func lexDocument(textLines []string) ([]displayLine, []containerInfo) {
	lx := &lexer{}
	inString := false
	for i, text := range textLines {
		lx.lexLine(i, text, &inString)
	}
	if len(lx.lines) == 0 {
		lx.lines = []displayLine{{kind: kindLeaf}}
	}
	// Assign fold kinds for containers spanning multiple lines.
	for id, c := range lx.containers {
		if c.closeLine > c.openLine {
			if ln := &lx.lines[c.openLine]; ln.kind == kindLeaf {
				ln.kind = kindOpen
				ln.openID = id
			}
			if cl := &lx.lines[c.closeLine]; cl.kind == kindLeaf {
				cl.kind = kindClose
				cl.closeID = id
			}
		}
	}
	return lx.lines, lx.containers
}

// top returns the innermost open container frame, or nil.
func (lx *lexer) top() *lexFrame {
	if len(lx.stack) == 0 {
		return nil
	}
	return &lx.stack[len(lx.stack)-1]
}

// resolveValuePath consumes a value slot and returns the path for a
// value starting now.
func (lx *lexer) resolveValuePath() string {
	top := lx.top()
	if top == nil {
		return ""
	}
	if top.isArray {
		p := fmt.Sprintf("%s[%d]", top.path, top.arrIdx)
		top.arrIdx++
		return p
	}
	if top.objMode == objExpectValue {
		p := childPathKey(top.path, top.key)
		top.objMode = objExpectComma
		return p
	}
	return top.path
}

// openContainer pushes a new container frame and returns its id.
func (lx *lexer) openContainer(bracket rune, path string, line int) int {
	id := len(lx.containers)
	lx.containers = append(lx.containers, containerInfo{openLine: line, closeLine: -1})
	frame := lexFrame{id: id, bracket: bracket, path: path, isArray: bracket == '['}
	if !frame.isArray {
		frame.objMode = objExpectKey
	}
	lx.stack = append(lx.stack, frame)
	return id
}

// closeContainer pops the innermost container and records its fold
// range. Returns the frame id and its JSON path.
func (lx *lexer) closeContainer(line int, closeText string) (int, string) {
	if len(lx.stack) == 0 {
		return -1, ""
	}
	frame := lx.stack[len(lx.stack)-1]
	lx.stack = lx.stack[:len(lx.stack)-1]
	c := &lx.containers[frame.id]
	c.closeLine = line
	c.closeText = closeText
	return frame.id, frame.path
}

// lexLine annotates one line. inString carries string continuation
// state across lines (a string may span lines while editing).
func (lx *lexer) lexLine(lineNo int, text string, inString *bool) {
	parents := make([]int, len(lx.stack))
	for i, f := range lx.stack {
		parents[i] = f.id
	}

	rs := []rune(text)
	segs := make([]segment, 0, 8)
	appendSeg := func(s string, cls segClass) {
		if s != "" {
			segs = append(segs, segment{text: s, cls: cls})
		}
	}

	linePath := ""
	pathSet := false
	setPath := func(p string) {
		if !pathSet {
			linePath = p
			pathSet = true
		}
	}

	j := 0
	for j < len(rs) {
		// Continuation of a multi-line string.
		if *inString {
			start := j
			for j < len(rs) {
				if rs[j] == '\\' {
					j += 2
					continue
				}
				if rs[j] == '"' {
					j++
					*inString = false
					break
				}
				j++
			}
			appendSeg(string(rs[start:j]), clsString)
			continue
		}

		r := rs[j]
		switch {
		case r == ' ' || r == '\t':
			start := j
			for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t') {
				j++
			}
			appendSeg(string(rs[start:j]), clsPunct)

		case r == '"':
			start := j
			j++
			closed := false
			for j < len(rs) {
				if rs[j] == '\\' {
					j += 2
					continue
				}
				if rs[j] == '"' {
					closed = true
					j++
					break
				}
				j++
			}
			if !closed {
				*inString = true
			}
			strText := string(rs[start:j])
			top := lx.top()
			isKey := top != nil && !top.isArray && top.objMode == objExpectKey
			if !isKey && closed && top != nil && !top.isArray {
				// Fallback: a string followed by ':' is a key.
				k := j
				for k < len(rs) && (rs[k] == ' ' || rs[k] == '\t') {
					k++
				}
				isKey = k < len(rs) && rs[k] == ':'
			}
			if isKey {
				appendSeg(strText, clsKey)
				key := strText
				if closed {
					if err := json.Unmarshal([]byte(strText), &key); err != nil {
						key = strText
					}
				}
				top.key = key
				top.objMode = objExpectColon
				setPath(childPathKey(top.path, key))
			} else {
				appendSeg(strText, clsString)
				setPath(lx.resolveValuePath())
			}

		case r == '{' || r == '[':
			appendSeg(string(r), clsPunct)
			path := lx.resolveValuePath()
			setPath(path)
			lx.openContainer(r, path, lineNo)
			j++

		case r == '}' || r == ']':
			closeText := string(r)
			k := j + 1
			for k < len(rs) && rs[k] == ' ' {
				k++
			}
			if k < len(rs) && rs[k] == ',' {
				closeText += ","
			}
			appendSeg(closeText, clsPunct)
			if _, cpath := lx.closeContainer(lineNo, closeText); cpath != "" {
				setPath(cpath)
			}
			j++

		case r == ',' || r == ':':
			appendSeg(string(r), clsPunct)
			if top := lx.top(); top != nil && !top.isArray {
				switch {
				case r == ':' && top.objMode == objExpectColon:
					top.objMode = objExpectValue
				case r == ',' && top.objMode == objExpectComma:
					top.objMode = objExpectKey
				}
			}
			j++

		case r == '-' || (r >= '0' && r <= '9'):
			start := j
			for j < len(rs) && strings.ContainsRune("0123456789+-.eE", rs[j]) {
				j++
			}
			appendSeg(string(rs[start:j]), clsNumber)
			setPath(lx.resolveValuePath())

		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			start := j
			for j < len(rs) && ((rs[j] >= 'a' && rs[j] <= 'z') || (rs[j] >= 'A' && rs[j] <= 'Z')) {
				j++
			}
			word := string(rs[start:j])
			cls := clsPunct
			switch {
			case strings.HasPrefix("true", word), strings.HasPrefix("false", word):
				cls = clsBool
			case strings.HasPrefix("null", word):
				cls = clsNull
			}
			appendSeg(word, cls)
			setPath(lx.resolveValuePath())

		default:
			appendSeg(string(r), clsPunct)
			j++
		}
	}

	// Lines without a value of their own inherit the enclosing path.
	if !pathSet {
		if top := lx.top(); top != nil {
			linePath = top.path
		}
	}

	lx.lines = append(lx.lines, displayLine{
		segs:    segs,
		plain:   text,
		lower:   strings.ToLower(text),
		width:   uniseg.StringWidth(text),
		kind:    kindLeaf, // fold kinds are assigned in lexDocument
		path:    linePath,
		parents: parents,
	})
}

// childPathKey joins an object key onto a dotted path, using bracket
// notation when the key is not a plain identifier.
func childPathKey(parent, key string) string {
	if isIdent(key) {
		if parent == "" {
			return key
		}
		return parent + "." + key
	}
	q := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "]", `\]`).Replace(key)
	return fmt.Sprintf(`%s["%s"]`, parent, q)
}

// isIdent reports whether s is usable as a dotted path segment.
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '$' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r >= 0x80) {
			return false
		}
	}
	return true
}
