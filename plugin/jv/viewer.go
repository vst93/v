package plugin_jv

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TreeNode represents a node in the JSON tree.
type JSONNode struct {
	Key       string
	Value     interface{}
	Type      string
	Depth     int
	Parent    *JSONNode
	Children  []*JSONNode
	Collapsed bool
	Path      string
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

	// Collapse deep nodes by default (depth >= 1)
	if depth >= 1 {
		node.Collapsed = true
	}

	return node
}

// nodeLabel returns a clean display label for a tree node.
// Uses standard ANSI-compatible tview color tags that work on both
// light and dark terminal backgrounds.
func nodeLabel(node *JSONNode) string {
	var sb strings.Builder

	// Root node
	if node.Depth == 0 && node.Key == "" {
		switch node.Type {
		case "object":
			count := len(node.Children)
			if node.Collapsed {
				return fmt.Sprintf("[#586e75::b]▸[-:-:-] root [#586e75]{%d}[-:-:-]", count)
			}
			return "[#586e75::b]▾[-:-:-] root"
		case "array":
			count := len(node.Children)
			if node.Collapsed {
				return fmt.Sprintf("[#586e75::b]▸[-:-:-] root [#586e75][%d][-:-:-]", count)
			}
			return "[#586e75::b]▾[-:-:-] root"
		}
	}

	// Expand/collapse indicator for containers (non-empty only)
	if node.Type == "object" || node.Type == "array" {
		hasChildren := len(node.Children) > 0
		if hasChildren {
			if node.Collapsed {
				sb.WriteString("[#586e75]▸[-:-:-] ")
			} else {
				sb.WriteString("[#586e75]▾[-:-:-] ")
			}
		} else {
			sb.WriteString("  ")
		}
	} else {
		sb.WriteString("  ")
	}

	// Key
	if node.Key != "" {
		if strings.HasPrefix(node.Key, "[") {
			// Array index - dim
			sb.WriteString(fmt.Sprintf("[#586e75]%s[-:-:-] ", node.Key))
		} else {
			sb.WriteString(fmt.Sprintf("[blue]%s[-:-:-] ", tview.Escape(node.Key)))
		}
	}

	// Value / summary
	switch node.Type {
	case "object":
		count := len(node.Children)
		if count == 0 {
			sb.WriteString("[#586e75]{}[-:-:-]")
		} else if node.Collapsed {
			sb.WriteString(fmt.Sprintf("[#586e75]{%d}[-:-:-]", count))
		}
	case "array":
		count := len(node.Children)
		if count == 0 {
			sb.WriteString("[#586e75][][-:-:-]")
		} else if node.Collapsed {
			sb.WriteString(fmt.Sprintf("[#586e75][%d][-:-:-]", count))
		}
	case "string":
		val := node.Value.(string)
		if len([]rune(val)) > 60 {
			val = string([]rune(val)[:60]) + "…"
		}
		sb.WriteString(fmt.Sprintf("[green]\"%s\"[-:-:-]", tview.Escape(val)))
	case "number":
		sb.WriteString(fmt.Sprintf("[yellow]%s[-:-:-]", node.Value))
	case "boolean":
		sb.WriteString(fmt.Sprintf("[#d33682]%v[-:-:-]", node.Value))
	case "null":
		sb.WriteString("[red]null[-:-:-]")
	}

	return sb.String()
}

// RunInteractive launches the interactive TUI tree viewer.
func RunInteractive(data interface{}, source string) error {
	app := tview.NewApplication()

	rootNode := BuildTree(data, "", "", 0, nil)

	// Tree view - no graphics lines, clean indentation
	tree := tview.NewTreeView().
		SetGraphics(false).
		SetAlign(false)
	tree.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", source)).
		SetTitleAlign(tview.AlignLeft)

	// Build tview tree nodes with indentation
	var buildTViewNodes func(jn *JSONNode) *tview.TreeNode
	buildTViewNodes = func(jn *JSONNode) *tview.TreeNode {
		node := tview.NewTreeNode(nodeLabel(jn)).
			SetExpanded(!jn.Collapsed).
			SetReference(jn).
			SetIndent(2)

		for _, child := range jn.Children {
			node.AddChild(buildTViewNodes(child))
		}
		return node
	}

	root := buildTViewNodes(rootNode)
	tree.SetRoot(root).SetCurrentNode(root)

	// Selection style: reverse video (works on both light and dark terminals)
	var applySelectedStyle func(tn *tview.TreeNode)
	applySelectedStyle = func(tn *tview.TreeNode) {
		tn.SetSelectedTextStyle(tcell.StyleDefault.Reverse(true))
		for _, child := range tn.GetChildren() {
			applySelectedStyle(child)
		}
	}
	applySelectedStyle(root)

	refreshNode := func(tn *tview.TreeNode) {
		ref := tn.GetReference()
		if ref == nil {
			return
		}
		jn := ref.(*JSONNode)
		tn.SetText(nodeLabel(jn)).
			SetExpanded(!jn.Collapsed)
	}

	var refreshAll func(tn *tview.TreeNode)
	refreshAll = func(tn *tview.TreeNode) {
		refreshNode(tn)
		for _, child := range tn.GetChildren() {
			refreshAll(child)
		}
	}

	// Info bar
	infoBar := tview.NewTextView()
	infoBar.SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	updateInfoBar := func() {
		current := tree.GetCurrentNode()
		ref := current.GetReference()
		if ref == nil {
			return
		}
		jn := ref.(*JSONNode)
		info := fmt.Sprintf(" [blue]Path[-:-:-] %s  [blue]Type[-:-:-] %s",
			jn.Path, jn.Type)
		if jn.Type == "object" || jn.Type == "array" {
			info += fmt.Sprintf("  [blue]Items[-:-:-] %d", len(jn.Children))
		}
		infoBar.SetText(info)
	}
	updateInfoBar()

	// Help bar
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	helpBar.SetText("[#586e75]Enter fold/expand  ↑↓ navigate  c collapse all  e expand all  y copy value  p copy path  q quit[-:-:-]")

	// Layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tree, 0, 1, true).
		AddItem(infoBar, 1, 0, false).
		AddItem(helpBar, 1, 0, false)

	// Tree selection handler
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

	// Keyboard handler
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

	// Mouse wheel: move selection cursor instead of just scrolling view
	tree.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseScrollUp {
			tree.Move(-1)
			return action, nil
		}
		if action == tview.MouseScrollDown {
			tree.Move(1)
			return action, nil
		}
		return action, event
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
		return
	}
}
