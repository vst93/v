package plugin_translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gookit/ini/v2"
	"golang.org/x/term"
)

const requestTimeout = 8 * time.Second

func ContainsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func targetLanguage(text, target string) string {
	if target != "" {
		return target
	}
	if ContainsChinese(text) {
		return "en"
	}
	return "zh"
}

// decodeResponse rejects HTTP failures before interpreting a provider's payload.
func decodeResponse(client *http.Client, req *http.Request, result any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type translationResult struct {
	name string
	text string
	word *WordInfo
	err  error
}

func (r translationResult) content() string {
	if r.err != nil {
		return ""
	}
	if strings.TrimSpace(r.text) != "" {
		return r.text
	}
	if r.word != nil {
		if strings.TrimSpace(r.word.Translation) != "" {
			return r.word.Translation
		}
		return strings.Join(r.word.Definitions, "\n")
	}
	return ""
}

func primaryTranslation(results []translationResult) (string, error) {
	var failures []error
	for _, result := range results {
		if text := result.content(); strings.TrimSpace(text) != "" {
			return text, nil
		}
		err := result.err
		if err == nil {
			err = fmt.Errorf("empty translation")
		}
		failures = append(failures, fmt.Errorf("%s: %w", result.name, err))
	}
	if len(failures) == 0 {
		return "", fmt.Errorf("no translation engines enabled: use v tr -enable-google or v tr -enable-cnki")
	}
	return "", fmt.Errorf("translation failed: %w", errors.Join(failures...))
}

// Search keeps automatic direction detection for callers outside the CLI.
func Search(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("no text to translate")
	}
	return search(text, targetLanguage(text, ""), false, false)
}

func search(text, target string, raw, toClipboard bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	providers := enabledProviders()
	completed := requestTranslations(ctx, text, target, providers)
	var primary string
	var err error
	if !raw && term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("TERM") != "dumb" {
		primary, err = displayLiveResults(ctx, os.Stdout, completed, providers, text)
	} else {
		primary, err = displayResults(os.Stdout, completed, providers, text, raw)
	}
	if err != nil {
		if raw {
			return fmt.Errorf("翻译失败")
		}
		// Normal mode has already shown each provider's short error message,
		// matching the original output without appending network details.
		return nil
	}
	if toClipboard {
		if err := clipboard.WriteAll(primary); err != nil {
			return fmt.Errorf("copy translation: %w", err)
		}
	}
	return nil
}

func enabledProviders() []string {
	providers := []string{"Youdao"}
	if ini.String("translate.tr_cnki") != "disable" {
		providers = append(providers, "CNKI")
	}
	if ini.String("translate.tr_google") != "disable" {
		providers = append(providers, "Google")
	}
	return providers
}

func requestTranslations(ctx context.Context, text, target string, providers []string) <-chan translationResult {
	completed := make(chan translationResult, 3)
	started := make(chan struct{})
	signalStarted := sync.OnceFunc(func() { close(started) })
	// Start Youdao's HTTP request first. GetConn runs before DNS/connection
	// setup, so the other providers never wait for Youdao's response.
	youdaoContext := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GetConn: func(string) { signalStarted() },
	})
	go func() {
		word, err := searchYoudao(youdaoContext, text, target)
		signalStarted() // Also release the other requests if setup failed.
		completed <- translationResult{name: "Youdao", word: word, err: err}
	}()
	<-started

	for _, name := range providers[1:] {
		go func() {
			result := translationResult{name: name}
			switch name {
			case "CNKI":
				result.text, result.err = cnkiTranslateContext(ctx, text, target)
			case "Google":
				result.text, result.err = googleTranslateContext(ctx, text, target)
			}
			completed <- result
		}()
	}
	return completed
}

func displayResults(out io.Writer, completed <-chan translationResult, providers []string, input string, raw bool) (string, error) {
	view := newTranslationView(input, providers)
	var results []translationResult
	printedRaw := false
	for range providers {
		result := <-completed
		results = append(results, result)
		view.accept(result)
		if text := result.content(); raw && !printedRaw && strings.TrimSpace(text) != "" {
			fmt.Fprintln(out, text)
			printedRaw = true
		}
	}
	if !raw {
		fmt.Fprintln(out, view.render(0, 0, 0))
	}
	return primaryTranslation(results)
}
