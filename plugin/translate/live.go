package plugin_translate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"v/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var loadingFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type translationSection struct {
	name   string
	done   bool
	result translationResult
}

type translationView struct {
	input    string
	sections []translationSection
}

func newTranslationView(input string, providers []string) translationView {
	v := translationView{input: input}
	for _, name := range []string{"Youdao", "CNKI", "Google"} {
		for _, provider := range providers {
			if provider == name {
				v.sections = append(v.sections, translationSection{name: name})
				break
			}
		}
	}
	return v
}

func (v *translationView) accept(result translationResult) {
	for i := range v.sections {
		if v.sections[i].name == result.name {
			v.sections[i].result = result
			v.sections[i].done = true
			return
		}
	}
}

// render keeps the provider order independent of arrival order. Bodies only
// occupy the rows they need; a height limit keeps all headings visible while
// loading. The final transcript has no height limit and includes every line.
func (v translationView) render(frame, width, height int) string {
	if height > 0 && height < 3+3*len(v.sections) {
		return v.compact(frame, width)
	}
	separatorWidth := 60
	if width > 0 {
		separatorWidth = min(separatorWidth, width)
	}
	separator := Cyan + strings.Repeat("-", separatorWidth) + Reset
	input := fmt.Sprintf("%s输入:%s %s%s%s", BoldGreen, Reset, BoldYellow, displayText(v.input), Reset)
	if width > 0 && height > 0 {
		input = ansi.Truncate(strings.ReplaceAll(input, "\n", " "), width, "…") + Reset
	}
	lines := []string{separator}
	lines = append(lines, wrapOutput(input, width)...)
	bodyHeight := 0
	if height > 0 && len(v.sections) > 0 {
		bodyHeight = max(1, (height-3-2*len(v.sections))/len(v.sections))
	}
	for _, section := range v.sections {
		title := map[string]string{"Youdao": "翻译", "CNKI": "CNKI翻译", "Google": "谷歌翻译"}[section.name]
		lines = append(lines, "", BoldGreen+title+":"+Reset)
		var body strings.Builder
		switch {
		case !section.done:
			fmt.Fprintf(&body, "  %s%s 加载中%s\n", Cyan, loadingFrames[frame%len(loadingFrames)], Reset)
		case errors.Is(section.result.err, errWordNotFound):
			fmt.Fprintf(&body, "  %s未收录%s\n", Yellow, Reset)
		case section.result.err != nil:
			message := map[string]string{"Youdao": "查询失败", "CNKI": "CNKI翻译失败", "Google": "谷歌翻译失败"}[section.name]
			fmt.Fprintf(&body, "  %s错误: %s%s\n", Red, message, Reset)
		case section.result.word != nil:
			writeWordDetails(&body, section.result.word, false)
		default:
			fmt.Fprintf(&body, "  %s%s%s\n", Green, displayText(section.result.text), Reset)
		}
		content := strings.TrimSuffix(body.String(), "\n")
		bodyLines := wrapOutput(content, width)
		if bodyHeight > 0 && len(bodyLines) > bodyHeight {
			bodyLines = bodyLines[:bodyHeight]
			bodyLines[len(bodyLines)-1] = ansi.Truncate(bodyLines[len(bodyLines)-1], max(1, width-1), "") + Reset + "…"
		}
		lines = append(lines, bodyLines...)
	}
	lines = append(lines, separator)
	if width > 0 {
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], width, "…")
		}
	}
	return strings.Join(lines, "\n")
}

var outputSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Each physical row carries its own color, since the terminal renderer may skip
// unchanged rows when only a loading indicator needs repainting.
func wrapOutput(text string, width int) []string {
	if width > 0 {
		text = ansi.Hardwrap(text, width, true)
	}
	lines := strings.Split(text, "\n")
	style := ""
	for i, line := range lines {
		prefix := style
		for _, code := range outputSGR.FindAllString(line, -1) {
			style = code
			if code == Reset {
				style = ""
			}
		}
		lines[i] = prefix + line + Reset
	}
	return lines
}

func (v translationView) compact(frame, width int) string {
	var lines []string
	for _, section := range v.sections {
		label := map[string]string{"Youdao": "翻译", "CNKI": "CNKI", "Google": "谷歌"}[section.name]
		content := Cyan + loadingFrames[frame%len(loadingFrames)] + " 加载中" + Reset
		if section.done {
			if errors.Is(section.result.err, errWordNotFound) {
				content = Yellow + "未收录" + Reset
			} else if section.result.err != nil {
				content = Red + "查询失败" + Reset
			} else {
				content = Green + strings.ReplaceAll(displayText(section.result.content()), "\n", " ") + Reset
			}
		}
		lines = append(lines, ansi.Truncate(BoldGreen+label+" "+Reset+content, width, "…")+Reset)
	}
	return strings.Join(lines, "\n")
}

// Provider/user text must not move the cursor while updating the inline view.
func displayText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r == '\n' || (r >= ' ' && (r < 0x7f || r > 0x9f)) {
			return r
		}
		return -1
	}, ansi.Strip(text))
}

type loadingTick struct{}

type liveModel struct {
	ctx       context.Context
	view      translationView
	completed <-chan translationResult
	results   []translationResult
	remaining int
	frame     int
	width     int
	height    int
	finished  bool
}

func (m liveModel) Init() tea.Cmd {
	return tea.Batch(m.nextResult(), nextLoadingTick())
}

func (m liveModel) nextResult() tea.Cmd {
	return func() tea.Msg {
		select {
		case result := <-m.completed:
			return result
		case <-m.ctx.Done():
			return nil
		}
	}
}

func nextLoadingTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return loadingTick{} })
}

func (m liveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width-1), min(24, max(1, msg.Height-1))
	case loadingTick:
		if !m.finished {
			m.frame++
			return m, nextLoadingTick()
		}
	case translationResult:
		m.view.accept(msg)
		m.results = append(m.results, msg)
		m.remaining--
		if m.remaining == 0 {
			m.finished = true
			// Print the complete transcript into scrollback, including any lines
			// clipped in a short terminal, then restore the cursor automatically.
			return m, tea.Sequence(tea.Println(m.view.render(m.frame, m.width, 0)), tea.Quit)
		}
		return m, m.nextResult()
	}
	return m, nil
}

func (m liveModel) View() string {
	if m.finished {
		return ""
	}
	return m.view.render(m.frame, m.width, m.height)
}

func displayLiveResults(ctx context.Context, out io.Writer, completed <-chan translationResult, providers []string, input string) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	theme.Init(true)
	model := liveModel{
		ctx: ctx, view: newTranslationView(input, providers), completed: completed,
		remaining: len(providers), width: 79, height: 23,
	}
	// This is an inline progress display: keep stdin available for pipes and
	// leave the terminal's normal Ctrl-C/signal handling intact.
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithOutput(out), tea.WithInput(nil), tea.WithFPS(30), tea.WithoutBracketedPaste())
	final, err := program.Run()
	if err != nil {
		return "", err
	}
	return primaryTranslation(final.(liveModel).results)
}
