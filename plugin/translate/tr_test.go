package plugin_translate

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestTranslationDirection(t *testing.T) {
	cases := []struct{ text, target, want string }{
		{"Hello world", "", "zh"},
		{"hello", "", "zh"},
		{"你好，世界", "", "en"},
		{"繁體中文", "", "en"},
		{"Go 语言很好用", "", "en"},
		{"𠀀", "", "en"},
		{"Go 语言很好用", "zh", "zh"},
		{"Hello world", "en", "en"},
	}
	for _, c := range cases {
		if got := targetLanguage(c.text, c.target); got != c.want {
			t.Errorf("targetLanguage(%q, %q) = %q, want %q", c.text, c.target, got, c.want)
		}
	}
}

func TestPipeInputTakesPriorityAndPreservesText(t *testing.T) {
	input := "第一行 & Go\n第二行 + 100%\n"
	opts, err := parseOptions([]string{"ignored", "words", "-clip", "-to", "zh", "-raw", "-copy", "-pipe", input})
	if err != nil {
		t.Fatal(err)
	}
	if opts.input != input || !opts.hasInput || opts.target != "zh" || !opts.raw || !opts.copy || !opts.clip {
		t.Fatalf("unexpected pipe options: %+v", opts)
	}
	if got := targetLanguage(opts.input, opts.target); got != "zh" {
		t.Fatalf("explicit target ignored: %q", got)
	}
}

func TestPositionalInputAndFlags(t *testing.T) {
	opts, err := parseOptions([]string{"Hello", "-to", "ZH", "world", "-clip"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.input != "Hello world" || !opts.hasInput || opts.target != "zh" {
		t.Fatalf("unexpected positional options: %+v", opts)
	}
}

func TestPipeContentIsNeverParsedAsFlags(t *testing.T) {
	opts, err := parseOptions([]string{"-pipe", "--help"})
	if err != nil || opts.help || opts.input != "--help" {
		t.Fatalf("pipe content interpreted as option: %+v, %v", opts, err)
	}
}

func TestEmptyInputReturnsError(t *testing.T) {
	plugin := &Tr{}
	for _, args := range [][]string{nil, {" "}, {"ignored", "-pipe", "\n\t"}, {"-clip", "-pipe", ""}} {
		if err := plugin.Run(args); err == nil || !strings.Contains(err.Error(), "no text") {
			t.Errorf("Run(%q) = %v, want empty-input error", args, err)
		}
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"-to"}, {"-pipe"}, {"-to", "fr"}, {"-to", "-raw"}, {"-typo"},
		{"-enable-google", "hello"}, {"-enable-google", "-disable-cnki"},
		{"-enable-cnki", "-copy"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("parseOptions(%q) should fail", args)
		}
	}
}

func TestHelpAndConfigurationParsing(t *testing.T) {
	for _, flag := range []string{"-h", "-help", "--help"} {
		opts, err := parseOptions([]string{flag, "-pipe", "ignored"})
		if err != nil || !opts.help {
			t.Errorf("help %q: %+v, %v", flag, opts, err)
		}
	}
	for _, flag := range []string{"-enable-google", "-disable-google", "-enable-cnki", "-disable-cnki"} {
		opts, err := parseOptions([]string{flag})
		if err != nil || opts.config != flag {
			t.Errorf("configuration %q: %+v, %v", flag, opts, err)
		}
	}
}

func TestGoogleRequestPreservesInputAndDirection(t *testing.T) {
	for _, input := range []string{"你好，世界 & Go\n第二行", "Hello + world? 100% #test"} {
		for _, target := range []string{"en", "zh"} {
			req, err := googleRequest(input, target)
			if err != nil {
				t.Fatal(err)
			}
			wantSource := "en"
			if target == "en" {
				wantSource = "zh-CN"
			}
			q := req.URL.Query()
			if q.Get("q") != input || q.Get("tl") != target || q.Get("sl") != wantSource {
				t.Errorf("unexpected Google request: %s", req.URL)
			}
		}
	}
}

func TestCNKIRequestPreservesInputAndDirection(t *testing.T) {
	for _, input := range []string{"你好，世界 & Go\n第二行", "Hello + world? 100% #test"} {
		for _, target := range []string{"en", "zh"} {
			req, err := cnkiRequest(input, target, "test-token")
			if err != nil {
				t.Fatal(err)
			}
			var body CNKITranslateRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if err := req.Body.Close(); err != nil {
				t.Fatal(err)
			}
			wantType := "0"
			if target == "zh" {
				wantType = "1"
			}
			if body.TranslateType != wantType || req.Header.Get("Token") != "test-token" {
				t.Fatalf("unexpected CNKI direction or token: %+v", body)
			}
			ciphertext, err := base64.URLEncoding.DecodeString(body.Words)
			if err != nil {
				t.Fatal(err)
			}
			block, err := aes.NewCipher([]byte("4e87183cfd3a45fe"))
			if err != nil {
				t.Fatal(err)
			}
			if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
				t.Fatalf("invalid encrypted request length: %d", len(ciphertext))
			}
			for i := 0; i < len(ciphertext); i += block.BlockSize() {
				block.Decrypt(ciphertext[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
			}
			padding := int(ciphertext[len(ciphertext)-1])
			if padding < 1 || padding > block.BlockSize() {
				t.Fatalf("invalid padding: %d", padding)
			}
			if got := string(ciphertext[:len(ciphertext)-padding]); got != input {
				t.Errorf("CNKI input = %q, want %q", got, input)
			}
		}
	}
}

func TestTranslationResponses(t *testing.T) {
	var google GoogleTranslateResponse
	if err := json.Unmarshal([]byte(`{"sentences":[{"trans":"Hello.\n"},{"trans":"World."}]}`), &google); err != nil {
		t.Fatal(err)
	}
	if got, err := google.translation(); err != nil || got != "Hello.\nWorld." {
		t.Errorf("Google response: %q, %v", got, err)
	}
	if _, err := (GoogleTranslateResponse{}).translation(); err == nil {
		t.Error("empty Google response should fail")
	}
	var cnki TranslateResp
	if err := json.Unmarshal([]byte(`{"code":200,"data":{"mResult":"你好，世界"}}`), &cnki); err != nil {
		t.Fatal(err)
	}
	if got, err := cnki.translation(); err != nil || got != "你好，世界" {
		t.Errorf("CNKI response: %q, %v", got, err)
	}
	for _, response := range []TranslateResp{{Code: 200}, {Code: 1004}} {
		if _, err := response.translation(); err == nil {
			t.Errorf("invalid CNKI response should fail: %+v", response)
		}
	}
}

func TestChineseDictionaryResponseParsing(t *testing.T) {
	info, err := parseWordInfo("你好", `<div id="ce_contentWrp"><h4>基本释义</h4><div class="trans-container ce"><ul><a class="clickable" href="/dict?q=hello">hello</a>; <a class="clickable" href="/dict?q=hi">hi</a>;</ul></div></div>`)
	if err != nil || len(info.Definitions) != 2 || info.Definitions[0] != "hello" || info.Definitions[1] != "hi" {
		t.Fatalf("unexpected Chinese-English entry: %+v, %v", info, err)
	}
	if got := (translationResult{word: info}).content(); got != "hello\nhi" {
		t.Fatalf("Chinese definitions unavailable for raw/copy: %q", got)
	}
}

func TestYoudaoSentenceTranslationParsing(t *testing.T) {
	info, err := parseWordInfo("今天天气很好", `<div id="fanyi_contentWrp"><h4>翻译结果</h4><div class="trans-container"><p>今天天气很好</p><p>The weather is nice today.</p></div></div>`)
	if err != nil || info.Translation != "The weather is nice today." {
		t.Fatalf("unexpected sentence translation: %+v, %v", info, err)
	}
}

func TestDictionaryResponseParsing(t *testing.T) {
	info, err := parseWordInfo("hello", `<div id="ec_contentWrp"><ul><li>int. <b>你好</b>&nbsp;&amp; 问候</li><li>n. 招呼</li></ul></div>`)
	if err != nil || len(info.Definitions) != 2 || info.Definitions[0] != "int. 你好 & 问候" {
		t.Fatalf("unexpected dictionary entry: %+v, %v", info, err)
	}
	if _, err := parseWordInfo("hello", "<html>service unavailable</html>"); err == nil {
		t.Error("unrecognized dictionary page should fail")
	}
}

func TestChineseDefinitionsKeepPlainTextAndMixedListItems(t *testing.T) {
	info, err := parseWordInfo("测试", `<div id='ce_contentWrp'><ul class='definitions'>
v. <a href='/dict?q=test'>test</a> (a system); measurement；<span>trial &amp; error</span>;
<li class='meaning'><b>n.</b> test; examination</li>
<a href='/dict?q=check'>check</a>;
</ul></div>`)
	if err != nil {
		t.Fatal(err)
	}
	want := "v. test (a system)\nmeasurement\ntrial & error\nn. test; examination\ncheck"
	if got := strings.Join(info.Definitions, "\n"); got != want {
		t.Fatalf("Chinese-English definitions lost list content:\ngot %q\nwant %q", got, want)
	}
}

func TestYoudaoDefinitionsStayWithinTheirSection(t *testing.T) {
	content := `<div id="ce_contentWrp"><div class="trans-container"></div></div>
<div id="ee_contentWrp"><ul><li>an unrelated English definition</li></ul></div>
<div id="dictNav"><ul><li>基本释义</li><li>网络释义</li></ul></div>`
	if info, err := parseWordInfo("未收录词", content); err == nil {
		t.Fatalf("unrelated list mistaken for a translation: %+v", info)
	}
}

func TestYoudaoUnlistedPhraseIsNotARequestFailure(t *testing.T) {
	content := `<div id="bd"><div class="content-wrp"><p class="empty-content">
<a href="javascript:;">该词条暂未被收录</a>
</p></div></div><div id="ft"><ul><li>有道首页</li></ul></div>`
	info, err := parseWordInfo("直播巡检", content)
	if info != nil || !errors.Is(err, errWordNotFound) {
		t.Fatalf("unlisted phrase = %+v, %v; want missing-entry status", info, err)
	}
	if _, err := parseWordInfo("直播巡检", "<html>service unavailable</html>"); err == nil || errors.Is(err, errWordNotFound) {
		t.Fatalf("unexpected response must not be treated as an unlisted phrase: %v", err)
	}
}

func TestYoudaoChineseDefinitionsPrecedeNetworkPhrases(t *testing.T) {
	content := `<div id="web_trans_contentWrp"><ul><li>unrelated phrase</li></ul></div>
<div id="ce_contentWrp"><ul><a href="/dict?q=optimize">optimize</a>; optimizing;</ul></div>`
	info, err := parseWordInfo("优化", content)
	if err != nil || strings.Join(info.Definitions, "; ") != "optimize; optimizing" {
		t.Fatalf("did not select Chinese-English basic definitions: %+v, %v", info, err)
	}
}

func TestYoudaoNetworkPhrasesPreserveTranslationText(t *testing.T) {
	content := `<div id="web_trans_contentWrp"><div class="webPhrase"><ul>
<li class="mcols-layout"><span class="col1 grey">1.</span><div class="col2"><a href="/dict?q=把中文转换成英文">把中文转换成英文</a><p>switch Chinese into English</p></div></li>
</ul></div></div>`
	info, err := parseWordInfo("中文转英文", content)
	if err != nil || len(info.Definitions) != 1 || info.Definitions[0] != "把中文转换成英文 switch Chinese into English" {
		t.Fatalf("network phrase translation lost: %+v, %v", info, err)
	}
}

func TestYoudaoTranslationKeepsAllParagraphs(t *testing.T) {
	content := `<div id='fanyi_contentWrp'><h4>翻译结果</h4><div class='trans-container fanyi'>
<p class='source'>你好。再见。</p><p class='target'>Hello &amp; welcome.</p><p>Goodbye.</p>
</div></div><p>unrelated footer</p>`
	info, err := parseWordInfo("你好。再见。", content)
	if err != nil || info.Translation != "Hello & welcome.\nGoodbye." {
		t.Fatalf("translation paragraphs missing or contaminated: %+v, %v", info, err)
	}
}

func TestPrimaryTranslationSurvivesProviderFailures(t *testing.T) {
	failure := errors.New("provider unavailable")
	results := []translationResult{
		{name: "Google", err: failure},
		{name: "CNKI", text: "你好"},
		{name: "Youdao", err: failure},
	}
	if got, err := primaryTranslation(results); err != nil || got != "你好" {
		t.Fatalf("fallback result = %q, %v", got, err)
	}
	results[0] = translationResult{name: "Google", text: "您好"}
	if got, err := primaryTranslation(results); err != nil || got != "您好" {
		t.Fatalf("preferred result = %q, %v", got, err)
	}
	results[0].err = failure
	results[1].err = failure
	if _, err := primaryTranslation(results); !errors.Is(err, failure) || !strings.Contains(err.Error(), "CNKI") {
		t.Fatalf("all failures should be reported and wrapped: %v", err)
	}
	if _, err := primaryTranslation(nil); err == nil {
		t.Error("no engines should produce an actionable error")
	}
}
