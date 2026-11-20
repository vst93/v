package plugin_translate

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"

	htmlnode "golang.org/x/net/html"
)

var errWordNotFound = errors.New("dictionary entry not found")

// WordInfo stores Chinese-English or English-Chinese definitions and translations.
type WordInfo struct {
	Word           string
	UkPhonetic     string
	UkAudioURL     string
	UsPhonetic     string
	UsAudioURL     string
	SinglePhonetic string
	SingleAudioURL string
	Definitions    []string
	Translation    string
}

func SearchWord(word string) (*WordInfo, error) {
	return searchWord(context.Background(), word)
}

func searchYoudao(ctx context.Context, text, target string) (*WordInfo, error) {
	info, err := searchWord(ctx, text)
	if !errors.Is(err, errWordNotFound) {
		return info, err
	}
	// An unlisted phrase is still translatable. Keep the same provider pending
	// until the separate translation endpoint returns its result.
	translation, err := youdaoTranslateContext(ctx, text, target)
	if err != nil {
		return nil, fmt.Errorf("Youdao translation fallback: %w", err)
	}
	return &WordInfo{Word: text, Translation: translation}, nil
}

func searchWord(ctx context.Context, word string) (*WordInfo, error) {
	if strings.TrimSpace(word) == "" {
		return nil, fmt.Errorf("word must not be empty")
	}
	client := &http.Client{Timeout: requestTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://m.youdao.com/dict", nil)
	if err != nil {
		return nil, fmt.Errorf("create dictionary request: %w", err)
	}
	query := req.URL.Query()
	// "eng" selects the Chinese-English dictionary in both directions.
	query.Set("le", "eng")
	query.Set("q", word)
	req.URL.RawQuery = query.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dictionary request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read dictionary response: %w", err)
	}
	return parseWordInfo(word, string(body))
}

var (
	patternAudio       = regexp.MustCompile(`英[\W\w]*?phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"[\W\w]*?美[\W\w]*?phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"`)
	patternSingleAudio = regexp.MustCompile(`phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"`)
	patternTags        = regexp.MustCompile(`<[^>]*>`)
)

func parseWordInfo(word, content string) (*WordInfo, error) {
	document, err := htmlnode.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse dictionary response: %w", err)
	}
	info := &WordInfo{Word: word}
	if matches := patternAudio.FindStringSubmatch(content); len(matches) >= 5 {
		info.UkPhonetic = cleanText(matches[1])
		info.UkAudioURL = cleanText(matches[2])
		info.UsPhonetic = cleanText(matches[3])
		info.UsAudioURL = cleanText(matches[4])
	} else if matches := patternSingleAudio.FindStringSubmatch(content); len(matches) >= 3 {
		info.SinglePhonetic = cleanText(matches[1])
		info.SingleAudioURL = cleanText(matches[2])
	}
	// Like the uTools plugin, retain the entire definition list, including
	// Chinese entries whose English meanings are text or links outside <li>.
	// Scope each lookup to its section so an empty dictionary cannot consume
	// a later navigation list or unrelated English-only definitions.
	sections := []string{"ec_contentWrp", "ce_contentWrp"}
	if ContainsChinese(word) {
		sections[0], sections[1] = sections[1], sections[0]
	}
	sections = append(sections, "ce_new_contentWrp", "web_trans_contentWrp")
	for _, id := range sections {
		section := findHTMLNode(document, func(n *htmlnode.Node) bool { return htmlAttribute(n, "id") == id })
		if section == nil {
			continue
		}
		list := findHTMLNode(section, func(n *htmlnode.Node) bool { return n.Data == "ul" })
		if list != nil {
			info.Definitions = parseDefinitionList(list)
		}
		if len(info.Definitions) > 0 {
			break
		}
	}
	if section := findHTMLNode(document, func(n *htmlnode.Node) bool {
		return htmlAttribute(n, "id") == "fanyi_contentWrp"
	}); section != nil {
		container := findHTMLNode(section, func(n *htmlnode.Node) bool {
			return strings.Contains(" "+htmlAttribute(n, "class")+" ", " trans-container ")
		})
		if container != nil {
			var paragraphs []string
			collectParagraphs(container, &paragraphs)
			// The first paragraph repeats the input; the rest is the translation.
			if len(paragraphs) > 1 {
				info.Translation = strings.TrimSpace(strings.Join(paragraphs[1:], "\n"))
			}
		}
	}
	if len(info.Definitions) == 0 && info.Translation == "" {
		missing := findHTMLNode(document, func(n *htmlnode.Node) bool {
			return strings.Contains(" "+htmlAttribute(n, "class")+" ", " empty-content ") &&
				strings.Contains(htmlText(n), "该词条暂未被收录")
		})
		if missing != nil {
			return nil, fmt.Errorf("%w: %q", errWordNotFound, word)
		}
		return nil, fmt.Errorf("no dictionary entry for %q", word)
	}
	return info, nil
}

func cleanText(text string) string {
	text = patternTags.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(html.UnescapeString(text)), " ")
}

func parseDefinitionList(list *htmlnode.Node) []string {
	var definitions []string
	var inline strings.Builder
	flushInline := func() {
		for _, text := range strings.FieldsFunc(inline.String(), func(r rune) bool { return r == ';' || r == '；' }) {
			if text = strings.Join(strings.Fields(text), " "); text != "" {
				definitions = append(definitions, text)
			}
		}
		inline.Reset()
	}
	for child := list.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == htmlnode.ElementNode && child.Data == "li" {
			flushInline()
			if text := htmlText(child); text != "" {
				definitions = append(definitions, text)
			}
		} else {
			writeHTMLText(&inline, child)
		}
	}
	flushInline()
	return definitions
}

func findHTMLNode(root *htmlnode.Node, match func(*htmlnode.Node) bool) *htmlnode.Node {
	if root.Type == htmlnode.ElementNode && match(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLNode(child, match); found != nil {
			return found
		}
	}
	return nil
}

func htmlAttribute(node *htmlnode.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func htmlText(node *htmlnode.Node) string {
	var text strings.Builder
	writeHTMLText(&text, node)
	return strings.Join(strings.Fields(text.String()), " ")
}

func writeHTMLText(text *strings.Builder, node *htmlnode.Node) {
	if node.Type == htmlnode.TextNode {
		text.WriteString(node.Data)
		return
	}
	if node.Data == "script" || node.Data == "style" {
		return
	}
	// Network phrases carry their own number in col1; the CLI numbers entries.
	if node.Type == htmlnode.ElementNode && strings.Contains(" "+htmlAttribute(node, "class")+" ", " col1 ") {
		return
	}
	block := node.Data == "div" || node.Data == "p" || node.Data == "br" || node.Data == "li"
	if block {
		text.WriteByte(' ')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeHTMLText(text, child)
	}
	if block {
		text.WriteByte(' ')
	}
}

func collectParagraphs(node *htmlnode.Node, paragraphs *[]string) {
	if node.Type == htmlnode.ElementNode && node.Data == "p" {
		*paragraphs = append(*paragraphs, htmlText(node))
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectParagraphs(child, paragraphs)
	}
}
