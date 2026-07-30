package plugin_diff

import (
	"strings"
)

// Op represents a diff operation type.
type Op int

const (
	OpEqual Op = iota // line is present in both sides
	OpAdd             // line exists only in the right (added)
	OpDel             // line exists only in the left (deleted)
)

// DiffLine represents a single line in the diff result.
type DiffLine struct {
	Op       Op
	Left     string // text from left side (empty for Add)
	Right    string // text from right side (empty for Del)
	LeftNum  int    // left line number (1-based, 0 if not present)
	RightNum int    // right line number (1-based, 0 if not present)
}

// DiffLines computes a line-level diff between two texts using the
// Myers algorithm. Returns a slice of DiffLine aligned for side-by-side
// display.
func DiffLines(left, right string) []DiffLine {
	leftLines := splitLines(left)
	rightLines := splitLines(right)

	// Compute Myers shortest edit script
	ses := myersDiff(leftLines, rightLines)

	// Group consecutive insertions/deletions and align them
	// into DiffLine entries.
	var result []DiffLine

	i, j := 0, 0 // indices into leftLines, rightLines
	for _, op := range ses {
		switch op.typ {
		case opEqual:
			result = append(result, DiffLine{
				Op:       OpEqual,
				Left:     leftLines[i],
				Right:    rightLines[j],
				LeftNum:  i + 1,
				RightNum: j + 1,
			})
			i++
			j++
		case opAdd:
			result = append(result, DiffLine{
				Op:       OpAdd,
				Right:    rightLines[j],
				RightNum: j + 1,
			})
			j++
		case opDel:
			result = append(result, DiffLine{
				Op:      OpDel,
				Left:    leftLines[i],
				LeftNum: i + 1,
			})
			i++
		}
	}

	// Post-process: pair up adjacent Del+Add blocks into aligned rows
	// so the side-by-side view shows changed lines on the same row.
	result = alignChangedLines(result)

	return result
}

// alignChangedLines merges adjacent Del and Add lines into "change" rows
// (where both Left and Right are non-empty, Op stays as the original).
// Non-paired deletions or additions keep their original form.
func alignChangedLines(lines []DiffLine) []DiffLine {
	var result []DiffLine

	i := 0
	for i < len(lines) {
		if lines[i].Op == OpDel {
			// Collect consecutive deletions
			dels := []DiffLine{}
			for i < len(lines) && lines[i].Op == OpDel {
				dels = append(dels, lines[i])
				i++
			}
			// Collect consecutive additions immediately following
			adds := []DiffLine{}
			for i < len(lines) && lines[i].Op == OpAdd {
				adds = append(adds, lines[i])
				i++
			}
			// Pair them up
			maxLen := len(dels)
			if len(adds) > maxLen {
				maxLen = len(adds)
			}
			for k := 0; k < maxLen; k++ {
				dl := DiffLine{Op: OpEqual}
				if k < len(dels) {
					dl.Op = OpDel
					dl.Left = dels[k].Left
					dl.LeftNum = dels[k].LeftNum
				}
				if k < len(adds) {
					if dl.Op == OpDel {
						dl.Op = OpEqual // mark as "changed" - both sides present but different
						// We'll use a special marker: Op will be set to indicate "change"
					} else {
						dl.Op = OpAdd
					}
					dl.Right = adds[k].Right
					dl.RightNum = adds[k].RightNum
				}
				// If both sides present and different, mark as "changed"
				if dl.Left != "" && dl.Right != "" && dl.Left != dl.Right {
					dl.Op = opChange
				}
				result = append(result, dl)
			}
		} else {
			result = append(result, lines[i])
			i++
		}
	}

	return result
}

// opChange is an internal operation type for lines that exist on both
// sides but have different content (a modification rather than pure add/del).
const opChange Op = 99

// splitLines splits text into lines, preserving the content without
// trailing newlines. An empty string produces an empty slice.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return []string{""}
	}
	// Remove trailing newline to avoid an extra empty line
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n")
}

// --- Myers diff algorithm ---

type opType int

const (
	opEqual opType = iota
	opAdd
	opDel
)

type sesEntry struct {
	typ opType
}

// myersDiff computes the shortest edit script between two sequences
// of lines using Eugene Myers' algorithm.
func myersDiff(a, b []string) []sesEntry {
	n := len(a)
	m := len(b)
	max := n + m

	if max == 0 {
		return nil
	}

	// V is the array of furthest-reaching points on diagonals.
	// V[k] = furthest x on diagonal k = x - y
	// We use a map for simplicity with negative indices.
	v := make(map[int]int)
	v[1] = 0

	var trace []map[int]int

	for d := 0; d <= max; d++ {
		// Save a copy of V for backtracking
		vCopy := make(map[int]int, len(v))
		for k, val := range v {
			vCopy[k] = val
		}
		trace = append(trace, vCopy)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[k-1] < v[k+1]) {
				x = v[k+1] // down (insert)
			} else {
				x = v[k-1] + 1 // right (delete)
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[k] = x
			if x >= n && y >= m {
				// Found the shortest edit script
				return backtrack(trace, a, b)
			}
		}
	}

	// Fallback: should not reach here
	return nil
}

// backtrack reconstructs the edit script from the trace.
func backtrack(trace []map[int]int, a, b []string) []sesEntry {
	n := len(a)
	m := len(b)
	x, y := n, m

	var script []sesEntry

	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y

		var prevK int
		if k == -d || (k != d && v[k-1] < v[k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			script = append(script, sesEntry{typ: opEqual})
			x--
			y--
		}

		if d > 0 {
			if x == prevX {
				script = append(script, sesEntry{typ: opAdd})
			} else {
				script = append(script, sesEntry{typ: opDel})
			}
		}

		x = prevX
		y = prevY
	}

	// Handle the initial diagonal (if any)
	for x > 0 && y > 0 && a[x-1] == b[y-1] {
		script = append(script, sesEntry{typ: opEqual})
		x--
		y--
	}

	// Reverse the script (it was built backwards)
	for i, j := 0, len(script)-1; i < j; i, j = i+1, j-1 {
		script[i], script[j] = script[j], script[i]
	}

	return script
}

// WordDiff computes inline word-level differences between two strings.
// Returns pairs of (text, isDifferent) for rendering inline highlights.
type WordDiffPart struct {
	Text     string
	DiffType Op // OpEqual, OpAdd, OpDel
}

// WordDiff compares two strings word-by-word and returns inline diff parts.
func WordDiff(left, right string) []WordDiffPart {
	leftWords := splitWords(left)
	rightWords := splitWords(right)

	ses := myersDiff(leftWords, rightWords)

	var parts []WordDiffPart
	i, j := 0, 0
	for _, op := range ses {
		switch op.typ {
		case opEqual:
			parts = append(parts, WordDiffPart{Text: leftWords[i], DiffType: OpEqual})
			i++
			j++
		case opAdd:
			parts = append(parts, WordDiffPart{Text: rightWords[j], DiffType: OpAdd})
			j++
		case opDel:
			parts = append(parts, WordDiffPart{Text: leftWords[i], DiffType: OpDel})
			i++
		}
	}
	return parts
}

// splitWords splits a string into words and whitespace tokens,
// preserving spaces so the reconstructed text matches the original.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder
	inWord := false

	for _, r := range s {
		if r == ' ' || r == '\t' {
			if inWord {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r)
			inWord = false
		} else {
			if !inWord && current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r)
			inWord = true
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}
