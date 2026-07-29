package plugin_gcm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateRequestAndParse(t *testing.T) {
	var gotPath, gotAuth, gotCT string
	var gotBody chatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"  feat(gcm): add generator\n\n"}}]}`)
	}))
	defer srv.Close()

	// trailing slash on base_url must not produce a double slash
	msg, err := generate(srv.URL+"/v1/", "sk-test", "gpt-4o-mini", "diff body")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if msg != "feat(gcm): add generator" {
		t.Errorf("content not trimmed: %q", msg)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotBody.Model != "gpt-4o-mini" || gotBody.MaxTokens != 200 || gotBody.Temperature != 0.3 {
		t.Errorf("body = %+v", gotBody)
	}
	if len(gotBody.Messages) != 2 ||
		gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != systemPrompt ||
		gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "diff body" {
		t.Errorf("messages = %+v", gotBody.Messages)
	}
}

func TestGenerateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid api key","type":"auth"}}`)
	}))
	defer srv.Close()

	_, err := generate(srv.URL, "bad", "m", "d")
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("want api error, got %v", err)
	}
}

func TestTruncateDiff(t *testing.T) {
	short := strings.Repeat("a", 100)
	if truncateDiff(short) != short {
		t.Error("short diff must pass through unchanged")
	}
	long := strings.Repeat("b", maxDiffChars+500)
	got := truncateDiff(long)
	if !strings.HasPrefix(got, strings.Repeat("b", maxDiffChars)) {
		t.Error("truncated diff must keep the first maxDiffChars bytes")
	}
	if strings.Count(got, "b") != maxDiffChars {
		t.Errorf("kept %d chars, want %d", strings.Count(got, "b"), maxDiffChars)
	}
	if !strings.Contains(got, "diff truncated") {
		t.Errorf("missing truncation note: %q", got[maxDiffChars:])
	}
}
