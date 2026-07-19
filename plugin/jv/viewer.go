package plugin_jv

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TreeNode represents a node in the JSON tree.
type JSONNode struct {
	Key       string      // key in parent object, or "" for root/array elements
	Value     interface{} // the parsed JSON value
	Type      string      // "object", "array", "string", "number", "bool", "null"
	Depth     int
	Parent    *JSONNode
	Children  []*JSONNode
	Collapsed bool
	Path      string // dotted path for display
}

// BuildTree constructs a JSONNode tree from a parsed JSON value.
func BuildTree(value interface{}, key, path string, depth int, parent *JSONNode) *JSONNode {
	node := &JSONNode{
		Key:    key,
		Value:  value,
		Type:   TypeString(value),
		Depth:  depth,
		Parent: parent,
		Path:   path,
	}

	switch v := value.(type) {
	case *OrderedMap:
		for _, k := range v.Keys() {
			childPath := path
			if childPath == "" {
				childPath = k
			} else {
				childPath = path + "." + k
			}
			child := BuildTree(v.Get(k), k, childPath, depth+1, node)
			node.Children = append(node.Children, child)
		}
	case []interface{}:
		for i, item := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			child := BuildTree(item, fmt.Sprintf("[%d]", i), childPath, depth+1, node)
			node.Children = append(node.Children, child)
		}
	}

	// Collapse deep nodes by default (depth >= 2)
	if depth >= 2 {
		node.Collapsed = true
	}

	return node
}

// nodeLabel returns the display label for a tree node.
func nodeLabel(node *JSONNode) string {
	var sb strings.Builder

	if node.Key != "" {
		// Object key or array index
		if strings.HasPrefix(node.Key, "[") {
			// Array index
			sb.WriteString(tview.Escape(fmt.Sprintf("%s ", node.Key)))
		} else {
			sb.WriteString(fmt.Sprintf("[#00ffff]%s[-:-:-] ", tview.Escape(node.Key)))
		}
	}

	switch node.Type {
	case "object":
		count := len(node.Children)
		if node.Collapsed {
			sb.WriteString(fmt.Sprintf("[#555555]{…}[-:-:-] [#888888]{%d}[-:-:-]", count))
		} else {
			sb.WriteString(fmt.Sprintf("[#555555]{[-:-:-]"))
		}
	case "array":
		count := len(node.Children)
		if node.Collapsed {
			sb.WriteString(fmt.Sprintf("[#555555][…] [-:-:-][#888888][%d][-:-:-]", count))
		} else {
			sb.WriteString(fmt.Sprintf("[#555555][[-:-:-]"))
		}
	case "string":
		val := node.Value.(string)
		if len([]rune(val)) > 50 {
			val = string([]rune(val)[:50]) + "…"
		}
		sb.WriteString(fmt.Sprintf("[#00ff00]\"%s\"[-:-:-]", tview.Escape(val)))
	case "number":
		sb.WriteString(fmt.Sprintf("[#ffff00]%s[-:-:-]", node.Value))
	case "bool":
		sb.WriteString(fmt.Sprintf("[#ff00ff]%v[-:-:-]", node.Value))
	case "null":
		sb.WriteString("[#ff0000::b]null[-:-:-]")
	}

	return sb.String()
}

// RunInteractive launches the interactive TUI tree viewer.
func RunInteractive(data interface{}, source string) error {
	app := tview.NewApplication()

	rootNode := BuildTree(data, "", "", 0, nil)

	// Tree view
	tree := tview.NewTreeView().
		SetGraphicsColor(tcell.ColorDarkGray)

	// Build tview tree nodes from our JSONNode tree
	var buildTViewNodes func(jn *JSONNode) *tview.TreeNode
	buildTViewNodes = func(jn *JSONNode) *tview.TreeNode {
		node := tview.NewTreeNode(nodeLabel(jn)).
			SetExpanded(!jn.Collapsed).
			SetReference(jn)

		for _, child := range jn.Children {
			node.AddChild(buildTViewNodes(child))
		}
		return node
	}

	root := buildTViewNodes(rootNode)
	tree.SetRoot(root).SetCurrentNode(root)

	// refreshNode updates the label and expansion state of a tview node
	// to match its underlying JSONNode.
	refreshNode := func(tn *tview.TreeNode) {
		ref := tn.GetReference()
		if ref == nil {
			return
		}
		jn := ref.(*JSONNode)
		tn.SetText(nodeLabel(jn)).
			SetExpanded(!jn.Collapsed)
	}

	// refreshAll walks the entire tview tree and refreshes each node.
	var refreshAll func(tn *tview.TreeNode)
	refreshAll = func(tn *tview.TreeNode) {
		refreshNode(tn)
		for _, child := range tn.GetChildren() {
			refreshAll(child)
		}
	}

	// Info bar at bottom
	infoBar := tview.NewTextView()
	infoBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	updateInfoBar := func() {
		current := tree.GetCurrentNode()
		ref := current.GetReference()
		if ref == nil {
			return
		}
		jn := ref.(*JSONNode)
		info := fmt.Sprintf(" [#00ffff]Path: [-:-:-]%s  [#00ffff]Type: [-:-:-]%s",
			jn.Path, jn.Type)
		if jn.Type == "object" || jn.Type == "array" {
			info += fmt.Sprintf("  [#00ffff]Items: [-:-:-]%d", len(jn.Children))
		}
		infoBar.SetText(info)
	}
	updateInfoBar()

	// Header
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetText(fmt.Sprintf("[#ffaa00::b]jv - JSON Viewer[-:-:-]  [#555555]%s[-:-:-]", source))

	// Help bar
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	helpBar.SetText("[#888888]Enter: collapse/expand  ↑↓: navigate  c: collapse all  e: expand all  y: copy value  p: copy path  q: quit[-:-:-]")

	// Layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(tree, 0, 1, true).
		AddItem(infoBar, 1, 0, false).
		AddItem(helpBar, 1, 0, false)

	// Key bindings
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		jn := ref.(*JSONNode)
		if jn.Type == "object" || jn.Type == "array" {
			jn.Collapsed = !jn.Collapsed
			refreshNode(node)
			updateInfoBar()
		}
	})

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				app.Stop()
				return nil
			case 'c':
				collapseAll(rootNode)
				refreshAll(root)
				updateInfoBar()
				return nil
			case 'e':
				expandAll(rootNode)
				refreshAll(root)
				updateInfoBar()
				return nil
			case 'y':
				current := tree.GetCurrentNode()
				if ref := current.GetReference(); ref != nil {
					jn := ref.(*JSONNode)
					copyNodeValue(jn)
				}
				return nil
			case 'p':
				current := tree.GetCurrentNode()
				if ref := current.GetReference(); ref != nil {
					jn := ref.(*JSONNode)
					copyToClipboard(jn.Path, "path")
				}
				return nil
			}
		}
		return event
	})

	tree.SetChangedFunc(func(node *tview.TreeNode) {
		updateInfoBar()
	})

	return app.SetRoot(layout, true).EnableMouse(true).Run()
}

// collapseAll recursively collapses all object/array nodes.
func collapseAll(node *JSONNode) {
	if node.Type == "object" || node.Type == "array" {
		node.Collapsed = true
	}
	for _, child := range node.Children {
		collapseAll(child)
	}
}

// expandAll recursively expands all nodes.
func expandAll(node *JSONNode) {
	node.Collapsed = false
	for _, child := range node.Children {
		expandAll(child)
	}
}

// copyNodeValue copies a node's JSON value to clipboard.
func copyNodeValue(node *JSONNode) {
	var val string
	switch node.Type {
	case "string":
		val = node.Value.(string)
	case "number", "bool":
		val = fmt.Sprintf("%v", node.Value)
	case "null":
		val = "null"
	case "object", "array":
		val = CompactJSON(node.Value, false)
	}
	copyToClipboard(val, "value")
}

// copyToClipboard copies text and shows a brief notification.
func copyToClipboard(text, label string) {
	if err := clipboardWrite(text); err != nil {
		// Silently ignore clipboard errors in TUI
		return
	}
	// Could add a toast notification here
}
