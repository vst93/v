package plugin_translate

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GoogleTranslateResponse struct {
	Sentences []struct {
		Trans string `json:"trans"`
	} `json:"sentences"`
}

func GoogleTranslate(text string) (string, error) {
	return googleTranslate(text, targetLanguage(text, ""))
}

func googleRequest(text, target string) (*http.Request, error) {
	// Pin the opposite source language: Google's auto detection may classify a
	// mixed sentence as the target language and return it without translating.
	source := "en"
	if target == "en" {
		source = "zh-CN"
	}
	query := url.Values{
		"client": {"gtx"}, "sl": {source}, "tl": {target},
		"dt": {"t"}, "ie": {"UTF-8"}, "dj": {"1"}, "q": {text},
	}
	return http.NewRequest(http.MethodGet, "https://translate.googleapis.com/translate_a/single?"+query.Encode(), nil)
}

func googleTranslate(text, target string) (string, error) {
	return googleTranslateContext(context.Background(), text, target)
}

func googleTranslateContext(ctx context.Context, text, target string) (string, error) {
	req, err := googleRequest(text, target)
	if err != nil {
		return "", fmt.Errorf("create Google request: %w", err)
	}
	var result GoogleTranslateResponse
	if err := decodeResponse(&http.Client{Timeout: requestTimeout}, req.WithContext(ctx), &result); err != nil {
		return "", err
	}
	return result.translation()
}

func (r GoogleTranslateResponse) translation() (string, error) {
	var result strings.Builder
	for _, sentence := range r.Sentences {
		result.WriteString(sentence.Trans)
	}
	if strings.TrimSpace(result.String()) == "" {
		return "", fmt.Errorf("empty translation")
	}
	return result.String(), nil
}
