package plugin_translate

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestLiveResultsKeepOrderAndUpdateInPlace(t *testing.T) {
	providers := []string{"Youdao", "CNKI", "Google"}
	model := liveModel{view: newTranslationView("hello", providers), remaining: 3, width: 60, height: 23}
	initial := ansi.Strip(model.View())
	if strings.Count(initial, "加载中") != 3 {
		t.Fatalf("expected three independent loaders: %q", initial)
	}
	if strings.Contains(initial, "\n\n\n") || strings.Contains(initial, "有道") {
		t.Fatalf("loading view contains reserved blank rows or a redundant title: %q", initial)
	}
	var headingRows []int
	for _, title := range []string{"翻译:", "CNKI翻译:", "谷歌翻译:"} {
		pos := strings.Index(initial, title)
		if pos < 0 {
			t.Fatalf("missing heading %q", title)
		}
		headingRows = append(headingRows, strings.Count(initial[:pos], "\n"))
	}
	if headingRows[0] >= headingRows[1] || headingRows[1] >= headingRows[2] {
		t.Fatalf("wrong provider order: %v", headingRows)
	}

	// Google finishes first, but its result must stay in the third region.
	updated, _ := model.Update(translationResult{name: "Google", text: "你好"})
	model = updated.(liveModel)
	partial := ansi.Strip(model.View())
	if strings.Count(partial, "加载中") != 2 || !strings.Contains(partial, "谷歌翻译:\n  你好") {
		t.Fatalf("Google result not shown while other engines are loading: %q", partial)
	}
	if model.finished {
		t.Fatal("loading stopped before the other requests finished")
	}
	updated, _ = model.Update(loadingTick{})
	model = updated.(liveModel)
	nextFrame := ansi.Strip(model.View())
	if nextFrame == partial || strings.Count(nextFrame, "⠙") != 2 {
		t.Fatalf("pending spinners did not advance: %q", nextFrame)
	}
	if nextFrame[strings.Index(nextFrame, "谷歌翻译:"):] != partial[strings.Index(partial, "谷歌翻译:"):] {
		t.Fatal("completed Google region changed on a loading tick")
	}

	updated, _ = model.Update(translationResult{name: "Youdao", word: &WordInfo{Definitions: []string{"第一条", "第二条", "第三条"}}})
	model = updated.(liveModel)
	partial = ansi.Strip(model.View())
	previous := -1
	for _, title := range []string{"翻译:", "CNKI翻译:", "谷歌翻译:"} {
		pos := strings.Index(partial, title)
		if pos <= previous {
			t.Errorf("provider order changed after content expanded: %q", partial)
		}
		previous = pos
	}
	if strings.Contains(partial, "\n\n\n") {
		t.Fatalf("completed content still has reserved blank rows: %q", partial)
	}
	updated, _ = model.Update(translationResult{name: "CNKI", err: errors.New("HTTP 500")})
	model = updated.(liveModel)
	if !model.finished || model.View() != "" {
		t.Fatal("finished live display should clear before leaving the final transcript")
	}
	final := ansi.Strip(model.view.render(0, 0, 0))
	if strings.Contains(final, "加载中") || !strings.Contains(final, "CNKI翻译失败") || strings.Contains(final, "HTTP") {
		t.Fatalf("wrong final state: %q", final)
	}
}

func TestWordOutputPreservesOriginalStyle(t *testing.T) {
	var out strings.Builder
	writeSeparator(&out)
	writeWordInfo(&out, &WordInfo{
		Word: "hello", UkPhonetic: "[həˈləʊ]", UsPhonetic: "[həˈloʊ]",
		Definitions: []string{"int. 你好"}, Translation: "你好",
	})
	want := "\033[36m" + strings.Repeat("-", 60) + "\033[0m\n" +
		"\033[1;32m输入:\033[0m \033[1;33mhello\033[0m\n" +
		"\033[1;32m发音:\033[0m \033[36m英式\033[0m \033[33m[həˈləʊ]\033[0m \033[36m美式\033[0m \033[33m[həˈloʊ]\033[0m\n" +
		"\033[1;32m释义:\033[0m\n  \033[36m1.\033[0m \033[32mint. 你好\033[0m\n" +
		"\033[1;32m翻译:\033[0m\n  \033[35m你好\033[0m\n"
	if got := out.String(); got != want {
		t.Fatalf("original output style changed:\ngot  %q\nwant %q", got, want)
	}
}

func TestYoudaoFallbackUpdatesSameSectionAndSupportsRawOutput(t *testing.T) {
	result := translationResult{name: "Youdao", word: &WordInfo{Word: "直播巡检", Translation: "Live inspection"}}
	view := newTranslationView("直播巡检", []string{"Youdao", "CNKI", "Google"})
	view.accept(translationResult{name: "Google", text: "Live stream inspection"})
	if strings.Count(ansi.Strip(view.render(0, 60, 23)), "加载中") != 2 {
		t.Fatal("pending Youdao fallback should keep loading alongside CNKI")
	}
	view.accept(result)
	output := ansi.Strip(view.render(0, 60, 23))
	if !strings.Contains(output, "翻译:\n  Live inspection") || strings.Count(output, "加载中") != 1 {
		t.Fatalf("fallback did not replace its loader: %q", output)
	}
	for _, unwanted := range []string{"释义", "未找到", "未收录", "失败", "有道", "目标语言"} {
		if strings.Contains(output, unwanted) {
			t.Errorf("fallback output includes redundant %q: %q", unwanted, output)
		}
	}
	completed := make(chan translationResult, 1)
	completed <- result
	var out strings.Builder
	primary, err := displayResults(&out, completed, []string{"Youdao"}, "直播巡检", true)
	if err != nil || primary != "Live inspection" || out.String() != "Live inspection\n" {
		t.Fatalf("fallback unavailable for raw/copy: output=%q, primary=%q, err=%v", out.String(), primary, err)
	}
}

func TestRedirectedResultsKeepOrderAndErrorsStayBrief(t *testing.T) {
	var out strings.Builder
	completed := make(chan translationResult, 3)
	for _, name := range []string{"Google", "Youdao", "CNKI"} {
		completed <- translationResult{name: name, err: errors.New("GET https://example.com: HTTP 500; EOF")}
	}
	if _, err := displayResults(&out, completed, []string{"Youdao", "CNKI", "Google"}, "hello", false); err == nil {
		t.Fatal("all failed providers should be recognized as failures")
	}
	output := ansi.Strip(out.String())
	if !strings.Contains(output, "错误: 查询失败") || !strings.Contains(output, "错误: 谷歌翻译失败") || !strings.Contains(output, "错误: CNKI翻译失败") {
		t.Fatalf("missing short error messages: %q", output)
	}
	if strings.Index(output, "翻译:") > strings.Index(output, "CNKI翻译:") || strings.Index(output, "CNKI翻译:") > strings.Index(output, "谷歌翻译:") {
		t.Fatalf("redirected results not in fixed order: %q", output)
	}
	for _, forbidden := range []string{"HTTP", "example.com", "EOF", "加载中", "目标语言", "\033[?", "\033[K"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("output contains unwanted detail/control %q", forbidden)
		}
	}
}

func TestRawOutputIncludesYoudaoAndUsesFirstArrival(t *testing.T) {
	var out strings.Builder
	completed := make(chan translationResult, 3)
	completed <- translationResult{name: "CNKI", err: errors.New("unavailable")}
	completed <- translationResult{name: "Youdao", word: &WordInfo{Word: "你好", Definitions: []string{"hello", "hi"}}}
	completed <- translationResult{name: "Google", text: "Hello"}
	primary, err := displayResults(&out, completed, []string{"Youdao", "CNKI", "Google"}, "你好", true)
	if err != nil || primary != "hello\nhi" || out.String() != "hello\nhi\n" {
		t.Fatalf("raw output = %q, primary = %q, err = %v", out.String(), primary, err)
	}
}

func TestRawOutputDoesNotWaitForSlowerProviders(t *testing.T) {
	completed := make(chan translationResult, 2)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		close(completed)
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	done := make(chan error, 1)
	go func() {
		_, err := displayResults(writer, completed, []string{"Youdao", "Google"}, "hello", true)
		done <- err
	}()
	completed <- translationResult{name: "Google", text: "你好"}
	output := make(chan string, 1)
	go func() {
		data := make([]byte, len("你好\n"))
		if _, err := io.ReadFull(reader, data); err != nil {
			output <- err.Error()
			return
		}
		output <- string(data)
	}()
	select {
	case got := <-output:
		if got != "你好\n" {
			t.Fatalf("raw output = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("raw translation blocked by a slower provider")
	}
	completed <- translationResult{name: "Youdao", err: errors.New("unavailable")}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("raw output did not finish")
	}
}

func TestLiveLayoutFitsNarrowAndShortTerminals(t *testing.T) {
	view := newTranslationView(strings.Repeat("中文输入", 30), []string{"Youdao", "CNKI", "Google"})
	longText := strings.Repeat("很长的译文👨‍👩‍👧‍👦\n", 30) + "完整结尾"
	view.accept(translationResult{name: "Youdao", word: &WordInfo{Definitions: []string{longText}}})
	for _, size := range [][2]int{{30, 23}, {14, 12}, {8, 5}} {
		frame := view.render(0, size[0], size[1])
		lines := strings.Split(frame, "\n")
		if len(lines) > size[1] {
			t.Errorf("frame exceeds height %d: %d lines", size[1], len(lines))
		}
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > size[0] {
				t.Errorf("line width %d exceeds %d: %q", w, size[0], line)
			}
		}
	}
	if !strings.Contains(view.render(0, 30, 0), "完整结尾") {
		t.Fatal("complete transcript lost content clipped during loading")
	}
}

func TestDisabledProvidersDoNotKeepLoading(t *testing.T) {
	view := newTranslationView("你好", []string{"Youdao"})
	frame := ansi.Strip(view.render(0, 60, 20))
	if strings.Count(frame, "加载中") != 1 || strings.Contains(frame, "CNKI") || strings.Contains(frame, "谷歌") {
		t.Fatalf("disabled providers shown as pending: %q", frame)
	}
}

func TestUnlistedPhraseShowsBriefStatusAndKeepsOtherTranslations(t *testing.T) {
	providers := []string{"Youdao", "CNKI", "Google"}
	view := newTranslationView("直播巡检", providers)
	view.accept(translationResult{name: "Youdao", err: errWordNotFound})
	for _, size := range [][2]int{{60, 23}, {20, 4}} {
		output := ansi.Strip(view.render(0, size[0], size[1]))
		if !strings.Contains(output, "未收录") || strings.Contains(output, "失败") || strings.Count(output, "加载中") != 2 {
			t.Fatalf("unlisted entry is shown as a failure or stopped other engines: %q", output)
		}
	}
	for _, raw := range []bool{false, true} {
		completed := make(chan translationResult, 3)
		completed <- translationResult{name: "Youdao", err: errWordNotFound}
		completed <- translationResult{name: "Google", text: "Live stream inspection"}
		completed <- translationResult{name: "CNKI", text: "Live broadcast inspection"}
		var out strings.Builder
		primary, err := displayResults(&out, completed, providers, "直播巡检", raw)
		if err != nil || primary != "Live stream inspection" {
			t.Fatalf("missing dictionary entry discarded another translation: %q, %v", primary, err)
		}
		if raw && out.String() != "Live stream inspection\n" {
			t.Fatalf("raw output includes dictionary status: %q", out.String())
		}
		if !raw && (!strings.Contains(out.String(), "未收录") || strings.Contains(out.String(), "失败")) {
			t.Fatalf("redirected output contains misleading errors: %q", out.String())
		}
	}
}

func TestLiveOutputDoesNotInterpretTextAsTerminalCommands(t *testing.T) {
	input := "hello\033[2J\033]0;title\a\rworld\t!"
	if got := displayText(input); got != "helloworld !" {
		t.Fatalf("unsafe terminal content: %q", got)
	}
}

func TestCompactLayoutKeepsAllLoadingIndicatorsVisible(t *testing.T) {
	view := newTranslationView("hello", []string{"Youdao", "CNKI", "Google"})
	output := ansi.Strip(view.render(0, 8, 4))
	if strings.Count(output, loadingFrames[0]) != 3 {
		t.Fatalf("provider names obscured loading indicators: %q", output)
	}
	if !strings.Contains(output, "翻译") || !strings.Contains(output, "CNKI") || !strings.Contains(output, "谷歌") {
		t.Fatalf("missing provider names in compact view: %q", output)
	}
}

func TestClosingLiveViewCancelsPendingResultWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := liveModel{ctx: ctx, completed: make(chan translationResult)}
	done := make(chan struct{})
	go func() {
		model.nextResult()()
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("closing the view left a pending result wait")
	}
}

func TestFinalTranscriptPreservesWrappedInputAndColors(t *testing.T) {
	input := "第一行很长的中文输入内容\n第二行也要完整显示"
	view := newTranslationView(input, []string{"Youdao"})
	view.accept(translationResult{name: "Youdao", word: &WordInfo{Definitions: []string{"hello world"}}})
	final := ansi.Strip(view.render(0, 12, 0))
	if !strings.Contains(strings.ReplaceAll(final, "\n", ""), strings.ReplaceAll(input, "\n", "")) {
		t.Fatalf("input truncated in final transcript: %q", final)
	}
	lines := wrapOutput(Green+"中文中文中文中文"+Reset, 4)
	if len(lines) != 4 {
		t.Fatalf("expected four wrapped rows, got %q", lines)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, Green) || !strings.HasSuffix(line, Reset) {
			t.Errorf("wrapped row depends on previous row's terminal style: %q", line)
		}
	}
}
