package plugin_translate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	htmlnode "golang.org/x/net/html"
)

func youdaoTranslationRequest(text, target string) (*http.Request, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text must not be empty")
	}
	var direction string
	switch target {
	case "en":
		direction = "ZH_CN2EN"
	case "zh":
		direction = "EN2ZH_CN"
	default:
		return nil, fmt.Errorf("unsupported target language: %q", target)
	}
	// This is the mobile translation form, separate from /dict word lookup.
	form := url.Values{"inputtext": {text}, "type": {direction}}
	req, err := http.NewRequest(http.MethodPost, "https://m.youdao.com/translate", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create Youdao translation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func youdaoTranslateContext(ctx context.Context, text, target string) (string, error) {
	req, err := youdaoTranslationRequest(text, target)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("Youdao translation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	return parseYoudaoTranslation(resp.Body)
}

func parseYoudaoTranslation(body io.Reader) (string, error) {
	document, err := htmlnode.Parse(body)
	if err != nil {
		return "", fmt.Errorf("parse Youdao translation: %w", err)
	}
	container := findHTMLNode(document, func(n *htmlnode.Node) bool {
		return n.Data == "ul" && htmlAttribute(n, "id") == "translateResult"
	})
	if container == nil {
		return "", fmt.Errorf("missing Youdao translation result")
	}
	var text strings.Builder
	for child := container.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == htmlnode.ElementNode && child.Data == "li" {
			writeYoudaoTranslation(&text, child)
		}
	}
	var lines []string
	for _, line := range strings.Split(text.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("empty Youdao translation")
	}
	return strings.Join(lines, "\n"), nil
}

// Preserve line breaks within a translated paragraph as well as separate
// result items. The page also contains the original text and navigation, which
// must never be mistaken for a translation.
func writeYoudaoTranslation(text *strings.Builder, node *htmlnode.Node) {
	if node.Type == htmlnode.TextNode {
		text.WriteString(node.Data)
		return
	}
	if node.Type != htmlnode.ElementNode || node.Data == "script" || node.Data == "style" {
		return
	}
	block := node.Data == "li" || node.Data == "p" || node.Data == "div" || node.Data == "br"
	if block {
		text.WriteByte('\n')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeYoudaoTranslation(text, child)
	}
	if block {
		text.WriteByte('\n')
	}
}
