package plugin_awake

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestParseArgs(t *testing.T) {
	duration, help, err := parseArgs([]string{"-d", "1h30m"})
	if err != nil {
		t.Fatalf("parseArgs returned an error: %v", err)
	}
	if help {
		t.Fatal("parseArgs unexpectedly requested help")
	}
	if duration != 90*time.Minute {
		t.Errorf("duration = %s, want %s", duration, 90*time.Minute)
	}
}

func TestParseArgsHelp(t *testing.T) {
	_, help, err := parseArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("parseArgs returned an error: %v", err)
	}
	if !help {
		t.Fatal("parseArgs did not request help")
	}
}

func TestParseArgsRejectsInvalidDuration(t *testing.T) {
	if _, _, err := parseArgs([]string{"-d", "tomorrow"}); err == nil {
		t.Fatal("parseArgs accepted an invalid duration")
	}
	if _, _, err := parseArgs([]string{"-d", "0s"}); err == nil {
		t.Fatal("parseArgs accepted a zero duration")
	}
}

func TestFormatClock(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00:00"},
		{3661 * time.Second, "01:01:01"},
		{90 * time.Minute, "01:30:00"},
		{-time.Second, "00:00:00"},
	}
	for _, c := range cases {
		if got := formatClock(c.in); got != c.want {
			t.Errorf("formatClock(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

func gridRows(g *pulseGrid) []string {
	rows := make([]string, len(g.cells))
	for y, row := range g.cells {
		var b strings.Builder
		for _, cell := range row {
			if cell.ch == ' ' {
				b.WriteByte(' ')
			} else {
				b.WriteRune(cell.ch)
			}
		}
		rows[y] = strings.TrimRight(b.String(), " ")
	}
	return rows
}

func joinRows(rows []string) string {
	return strings.Join(rows, "\n")
}

func TestAwakeStatusFrameSize(t *testing.T) {
	const w, h = 60, 17
	started := time.Date(2025, 1, 2, 14, 32, 5, 0, time.Local)
	grid := renderAwakeStatus(w, h, started, 90*time.Minute, 30*time.Minute, true, "linux")
	if grid.w != w || grid.h != h {
		t.Fatalf("grid size = %dx%d, want %dx%d", grid.w, grid.h, w, h)
	}
	content := 0
	for row := range grid.cells {
		for col, cell := range grid.cells[row] {
			if cell.ch == ' ' {
				continue
			}
			content++
			if col < 0 || col >= w || row < 0 || row >= h {
				t.Fatalf("cell outside canvas at (%d,%d)", col, row)
			}
		}
	}
	if content == 0 {
		t.Fatal("awake status frame is empty")
	}
}

func TestAwakeStatusShowsInfo(t *testing.T) {
	const w, h = 60, 17
	started := time.Date(2025, 1, 2, 14, 32, 5, 0, time.Local)
	grid := renderAwakeStatus(w, h, started, 90*time.Minute, 30*time.Minute, true, "linux")
	text := joinRows(gridRows(grid))

	for _, want := range []string{
		"AWAKE",
		"Status:",
		"Active",
		"System:",
		"linux",
		"Started:",
		"14:32:05",
		"Elapsed:",
		"01:30:00",
		"Remaining:",
		"00:30:00",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("status panel missing %q", want)
		}
	}
	if !strings.Contains(text, "[") || !strings.Contains(text, "]") || !strings.Contains(text, "75%") {
		t.Error("status panel missing the progress bar")
	}
}

func TestAwakeStatusNoLimit(t *testing.T) {
	const w, h = 60, 17
	started := time.Date(2025, 1, 2, 14, 32, 5, 0, time.Local)
	grid := renderAwakeStatus(w, h, started, 75*time.Second, 0, false, "linux")
	text := joinRows(gridRows(grid))

	if !strings.Contains(text, "Elapsed:") || !strings.Contains(text, "00:01:15") {
		t.Errorf("status panel missing elapsed time, got:\n%s", text)
	}
	if strings.Contains(text, "Remaining") {
		t.Errorf("status panel should not show Remaining without a duration")
	}
	if strings.Contains(text, "[") {
		t.Errorf("status panel should not show a progress bar without a duration")
	}
}

func TestAwakeStatusProgressChanges(t *testing.T) {
	const w, h = 60, 17
	started := time.Date(2025, 1, 2, 14, 32, 5, 0, time.Local)

	countFilled := func(grid *pulseGrid) int {
		n := 0
		for _, row := range grid.cells {
			for _, cell := range row {
				if cell.ch == '█' {
					n++
				}
			}
		}
		return n
	}

	empty := renderAwakeStatus(w, h, started, 0, time.Minute, true, "linux")
	half := renderAwakeStatus(w, h, started, 30*time.Second, 30*time.Second, true, "linux")
	if countFilled(empty) != 0 {
		t.Errorf("empty progress has %d filled cells, want 0", countFilled(empty))
	}
	if countFilled(half) == 0 {
		t.Error("half progress has no filled cells")
	}
}

func TestAwakeStatusTinyCanvas(t *testing.T) {
	started := time.Now()
	for _, size := range [][2]int{{4, 4}, {6, 4}, {11, 3}, {20, 8}} {
		w, h := size[0], size[1]
		grid := renderAwakeStatus(w, h, started, 0, 0, false, "linux")
		if grid.w != w || grid.h != h {
			t.Fatalf("grid size = %dx%d, want %dx%d", grid.w, grid.h, w, h)
		}
	}
}

func TestPulseViewDrawSmoke(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(60, 17)

	view := newPulseView("linux", 0)
	view.SetRect(0, 0, 60, 17)
	view.Draw(s)
	s.Show()

	_, _, st, _ := s.GetContent(0, 0)
	if _, bg, _ := st.Decompose(); bg != tcell.ColorDefault {
		t.Errorf("animation background = %v, want ColorDefault", bg)
	}

	found := false
	for y := 0; y < 17; y++ {
		for x := 0; x < 60; x++ {
			c, _, _, _ := s.GetContent(x, y)
			if c == 'A' || c == 'K' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("AWAKE title not found on the pulse view")
	}
}
