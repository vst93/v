package plugin_awake

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// awakeStyle indexes awakeStyles. The display uses a restrained neutral
// palette with a single green accent for the active status and progress fill.
type awakeStyle int

const (
	styleDim awakeStyle = iota
	styleMuted
	styleText
	styleAccent
)

var awakeStyles = []tcell.Style{
	tcell.StyleDefault.Foreground(tcell.NewHexColor(0x6c7086)),
	tcell.StyleDefault.Foreground(tcell.NewHexColor(0x585b70)),
	tcell.StyleDefault.Foreground(tcell.NewHexColor(0xcdd6f4)).Bold(true),
	tcell.StyleDefault.Foreground(tcell.NewHexColor(0xa6e3a1)),
}

// pulseCell is one character of the status canvas.
type pulseCell struct {
	ch    rune
	style awakeStyle
}

// pulseGrid is a bounded character canvas. Later draws overwrite earlier ones.
type pulseGrid struct {
	w, h  int
	cells [][]pulseCell
}

func newPulseGrid(w, h int) *pulseGrid {
	g := &pulseGrid{w: w, h: h, cells: make([][]pulseCell, h)}
	for y := range g.cells {
		g.cells[y] = make([]pulseCell, w)
		for x := range g.cells[y] {
			g.cells[y][x] = pulseCell{ch: ' '}
		}
	}
	return g
}

func (g *pulseGrid) set(x, y int, ch rune, style awakeStyle) {
	if ch == ' ' || ch == 0 || x < 0 || y < 0 || x >= g.w || y >= g.h {
		return
	}
	g.cells[y][x] = pulseCell{ch: ch, style: style}
}

// pulseView is the animation area. It redraws the status panel on every draw,
// so the elapsed timer and progress bar stay current after resize.
type pulseView struct {
	*tview.Box
	start    time.Time
	duration time.Duration
	osName   string
}

func newPulseView(osName string, duration time.Duration) *pulseView {
	return &pulseView{Box: tview.NewBox().SetBackgroundColor(tcell.ColorDefault), start: time.Now(), duration: duration, osName: osName}
}

func (p *pulseView) Draw(screen tcell.Screen) {
	p.Box.DrawForSubclass(screen, p)
	x, y, w, h := p.Box.GetInnerRect()
	if w <= 0 || h <= 0 {
		return
	}
	elapsed := time.Since(p.start)
	remaining := time.Duration(0)
	hasLimit := p.duration > 0
	if hasLimit {
		remaining = p.duration - elapsed
		if remaining < 0 {
			remaining = 0
		}
	}
	grid := renderAwakeStatus(w, h, p.start, elapsed, remaining, hasLimit, p.osName)
	for row := 0; row < h && row < grid.h; row++ {
		for col := 0; col < w && col < grid.w; col++ {
			cell := grid.cells[row][col]
			if cell.ch == ' ' {
				continue
			}
			screen.SetContent(x+col, y+row, cell.ch, nil, awakeStyles[cell.style])
		}
	}
}

type panelRow struct {
	label      string
	value      string
	valueStyle awakeStyle
}

// renderAwakeStatus builds one frame of the status panel: a centered title
// with a thin divider, aligned key/value rows, and an optional progress bar.
func renderAwakeStatus(w, h int, started time.Time, elapsed, remaining time.Duration, hasLimit bool, osName string) *pulseGrid {
	g := newPulseGrid(w, h)
	if w < 12 || h < 4 {
		return g
	}
	cx := w / 2

	rows := []panelRow{
		{label: "Status:", value: "Active", valueStyle: styleAccent},
		{label: "System:", value: osName},
		{label: "Started:", value: started.Format("15:04:05")},
		{label: "Elapsed:", value: formatClock(elapsed)},
	}
	if hasLimit {
		rows = append(rows, panelRow{label: "Remaining:", value: formatClock(remaining)})
	}

	maxValue := 0
	for _, row := range rows {
		if v := len([]rune(row.value)); v > maxValue {
			maxValue = v
		}
	}
	const labelColWidth = 10 // fixed column matches the widest label, "Remaining:"
	panelWidth := labelColWidth + 2 + maxValue
	if panelWidth < 12 {
		panelWidth = 12
	}
	if panelWidth > w-4 {
		panelWidth = w - 4
	}
	panelX := cx - panelWidth/2
	valueX := panelX + labelColWidth + 2
	block := 1 + 1 + len(rows)
	if hasLimit {
		block += 2 // blank line and progress bar
	}
	y := (h - block) / 2
	if y < 0 {
		y = 0
	}

	drawCenteredText(g, cx, y, "AWAKE", styleText)
	y++
	drawDivider(g, cx, y, panelWidth)
	y++
	for _, row := range rows {
		labelLen := len([]rune(row.label))
		drawText(g, valueX-2-labelLen, y, row.label, styleDim)
		drawText(g, valueX, y, row.value, row.valueStyle)
		y++
	}
	if hasLimit {
		y++
		drawProgressBar(g, cx, y, elapsed, remaining, panelWidth)
	}
	return g
}

func drawText(g *pulseGrid, x, y int, text string, style awakeStyle) {
	for i, r := range []rune(text) {
		g.set(x+i, y, r, style)
	}
}

func drawCenteredText(g *pulseGrid, cx, y int, text string, style awakeStyle) {
	runes := []rune(text)
	drawText(g, cx-len(runes)/2, y, text, style)
}

func drawDivider(g *pulseGrid, cx, y, width int) {
	start := cx - width/2
	for i := 0; i < width; i++ {
		g.set(start+i, y, '─', styleDim)
	}
}

func drawProgressBar(g *pulseGrid, cx, y int, elapsed, remaining time.Duration, panelWidth int) {
	total := elapsed + remaining
	fraction := 0.0
	if total > 0 {
		fraction = elapsed.Seconds() / total.Seconds()
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	barWidth := panelWidth - 6
	if barWidth < 8 {
		barWidth = 8
	}
	pct := fmt.Sprintf("%d%%", int(fraction*100+0.5))
	totalLen := barWidth + 2 + 1 + len(pct)
	start := cx - totalLen/2

	g.set(start, y, '[', styleDim)
	for i := 0; i < barWidth; i++ {
		ch, style := '░', styleMuted
		if float64(i)/float64(barWidth) < fraction {
			ch, style = '█', styleAccent
		}
		g.set(start+1+i, y, ch, style)
	}
	g.set(start+1+barWidth, y, ']', styleDim)
	x := start + barWidth + 3
	for _, r := range pct {
		g.set(x, y, r, styleText)
		x++
	}
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d/time.Minute) % 60
	s := int(d/time.Second) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
