package plugin_translate

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestYoudaoTranslationRequestPreservesInputAndDirection(t *testing.T) {
	input := "直播巡检 & Go + 100% #test\n检查网络状态。"
	for _, target := range []string{"en", "zh"} {
		req, err := youdaoTranslationRequest(input, target)
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != http.MethodPost || req.URL.String() != "https://m.youdao.com/translate" {
			t.Fatalf("unexpected translation endpoint: %s %s", req.Method, req.URL)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if err := req.Body.Close(); err != nil {
			t.Fatal(err)
		}
		wantType := "ZH_CN2EN"
		if target == "zh" {
			wantType = "EN2ZH_CN"
		}
		if req.PostForm.Get("inputtext") != input || req.PostForm.Get("type") != wantType {
			t.Fatalf("input or direction lost: %v", req.PostForm)
		}
	}
	if _, err := youdaoTranslationRequest(" \n", "en"); err == nil {
		t.Error("empty translation input should fail")
	}
	if _, err := youdaoTranslationRequest(input, "fr"); err == nil {
		t.Error("unsupported translation direction should fail")
	}
}

func TestYoudaoTranslationResponsePreservesLinesAndIgnoresPageContent(t *testing.T) {
	content := `<textarea name="inputtext">直播巡检</textarea>
<div class="generate"><p class="tip">译文</p><ul id="translateResult">
<li>Live inspection
Check the network status.</li>
<li><span>Hello &amp; welcome.</span><br>Goodbye.<script>unrelated script</script></li>
</ul></div><ul><li>有道首页</li></ul>`
	got, err := parseYoudaoTranslation(strings.NewReader(content))
	want := "Live inspection\nCheck the network status.\nHello & welcome.\nGoodbye."
	if err != nil || got != want {
		t.Fatalf("translation = %q, %v; want %q", got, err, want)
	}
	got, err = parseYoudaoTranslation(strings.NewReader(`<ul id="translateResult"><li>请在活动开始前查看直播。</li></ul>`))
	if err != nil || got != "请在活动开始前查看直播。" {
		t.Fatalf("English-Chinese translation = %q, %v", got, err)
	}
}

func TestYoudaoTranslationRejectsMissingOrEmptyResults(t *testing.T) {
	for _, content := range []string{
		`<html>service unavailable</html>`,
		`<textarea>original text</textarea><ul><li>有道首页</li></ul>`,
		`<ul id="translateResult"></ul>`,
		`<ul id="translateResult"><li> &nbsp; <br><script>ignored</script></li></ul>`,
		`<ul id="translateResult"><li>`,
	} {
		got, err := parseYoudaoTranslation(strings.NewReader(content))
		if err == nil || got != "" || errors.Is(err, errWordNotFound) {
			t.Errorf("invalid response = %q, %v; want translation failure", got, err)
		}
	}
}

func TestYoudaoTranslationHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := youdaoTranslateContext(ctx, "直播巡检", "en"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled translation returned %v", err)
	}
}
