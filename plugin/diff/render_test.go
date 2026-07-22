package plugin_diff

import (
	"strings"
	"testing"
)

// TestRenderLayout verifies the new rendering emits full-width background
// tints, word-level highlight backgrounds, and hatch fills for blank regions.
func TestRenderLayout(t *testing.T) {
	// Mix of op types: equal, change, pure delete, pure add.
	// "DELETEME" (left only) and "ADDME" (right only) are separated by the
	// common "mid" line so they are not paired into a change.
	left := "same\nhello world\nDELETEME\nmid\nend\n"
	right := "same\nhello go\nmid\nADDME\nend\n"
	lines := DiffLines(left, right)

	dv := &DiffViewer{lines: lines, panelWidth: 40}
	lc, rc := dv.renderLines(lines)

	t.Logf("LEFT content (tags visible):\n%s", lc)
	t.Logf("RIGHT content (tags visible):\n%s", rc)

	// Changed line (hello world / hello go) should tint both sides.
	if !strings.Contains(lc, colorDelBg) {
		t.Errorf("left side missing del/change background %q", colorDelBg)
	}
	if !strings.Contains(lc, colorDelWordBg) {
		t.Errorf("left side missing word-highlight background %q", colorDelWordBg)
	}
	if !strings.Contains(rc, colorAddBg) {
		t.Errorf("right side missing add/change background %q", colorAddBg)
	}
	if !strings.Contains(rc, colorAddWordBg) {
		t.Errorf("right side missing word-highlight background %q", colorAddWordBg)
	}

	// Pure delete: right side blank -> hatch. Pure add: left side blank -> hatch.
	hatchCount := strings.Count(lc, hatchChar)
	if hatchCount == 0 {
		t.Errorf("left side missing hatch fill for a pure-addition row")
	}
	hatchCountR := strings.Count(rc, hatchChar)
	if hatchCountR == 0 {
		t.Errorf("right side missing hatch fill for a pure-deletion row")
	}

	// Background padding should fill to panelWidth: a changed line on the left
	// must contain a run of bg-coloured spaces.
	if !strings.Contains(lc, "[-:"+colorDelBg+"]") {
		t.Errorf("left side missing background padding segment")
	}
	if !strings.Contains(rc, "[-:"+colorAddBg+"]") {
		t.Errorf("right side missing background padding segment")
	}

	// Regions must be present for highlight navigation.
	if !strings.Contains(lc, `["L0"]`) || !strings.Contains(lc, `["L5"]`) {
		t.Errorf("left side missing region tags")
	}
}

// TestBarStyling verifies the jv-style help/status bar helpers emit well-formed
// pill tags (teal background, bold) and " · "-joined status segments.
func TestBarStyling(t *testing.T) {
	p := pill("Ctrl+D")
	if !strings.Contains(p, "["+colorPillFg+":"+colorPillBg+":b]") {
		t.Errorf("pill missing teal bg tag: %q", p)
	}
	if !strings.Contains(p, "Ctrl+D") {
		t.Errorf("pill missing key text: %q", p)
	}

	ht := helpText([]helpItem{{"Tab", "switch"}, {"q", "quit"}})
	if !strings.Contains(ht, "["+colorPillFg+":"+colorPillBg+":b] Tab [-:-:-]") {
		t.Errorf("helpText missing Tab pill: %q", ht)
	}
	if !strings.Contains(ht, "["+colorBarDim+"]switch[-:-:-]") {
		t.Errorf("helpText missing dim description: %q", ht)
	}
	// Two pills separated by a double space.
	if strings.Count(ht, "  ") < 1 {
		t.Errorf("helpText pills not separated: %q", ht)
	}
}
