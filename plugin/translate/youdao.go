package plugin_translate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gookit/ini/v2"
)

// WordInfo 存储单词信息
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

// GoogleTranslateResponse 谷歌翻译API响应结构
type GoogleTranslateResponse struct {
	Sentences []struct {
		Trans string `json:"trans"`
	} `json:"sentences"`
}

// CNKITranslateRequest CNKI翻译请求结构
type CNKITranslateRequest struct {
	Words         string `json:"words"`
	TranslateType any    `json:"translateType"`
}

// CNKITranslateResponse CNKI翻译响应结构
type CNKITranslateResponse struct {
	Code int `json:"code"`
	Data struct {
		MResult string `json:"mResult"`
	} `json:"data"`
}

// 颜色定义（ANSI 转义序列）
const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Blue       = "\033[34m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	White      = "\033[37m"
	BoldCyan   = "\033[1;36m"
	BoldYellow = "\033[1;33m"
	BoldGreen  = "\033[1;32m"
)

// SearchWord 查询单词
func SearchWord(word string) (*WordInfo, error) {
	if word == "" {
		return nil, fmt.Errorf("单词不能为空")
	}

	url := fmt.Sprintf("https://m.youdao.com/dict?le=eng&q=%s", word)

	// 发送 HTTP 请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	htmlContent := string(body)
	wordInfo := &WordInfo{Word: word}

	// 1. 解析英式和美式发音
	patternAudio := regexp.MustCompile(`英[\W\w]*?phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"[\W\w]*?美[\W\w]*?phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"`)
	matchesAudio := patternAudio.FindStringSubmatch(htmlContent)

	if matchesAudio != nil && len(matchesAudio) >= 5 {
		wordInfo.UkPhonetic = cleanText(matchesAudio[1])
		wordInfo.UkAudioURL = cleanText(matchesAudio[2])
		wordInfo.UsPhonetic = cleanText(matchesAudio[3])
		wordInfo.UsAudioURL = cleanText(matchesAudio[4])
	} else {
		// 单个读音
		patternSingleAudio := regexp.MustCompile(`phonetic">([\W\w]*?)</span[\W\w]*?data-rel="([\W\w]*?)"`)
		matchesSingleAudio := patternSingleAudio.FindStringSubmatch(htmlContent)

		if matchesSingleAudio != nil && len(matchesSingleAudio) >= 3 {
			wordInfo.SinglePhonetic = cleanText(matchesSingleAudio[1])
			wordInfo.SingleAudioURL = cleanText(matchesSingleAudio[2])
		}
	}

	// 2. 解析基础解释
	patternDefinitions := regexp.MustCompile(`_contentWrp"[\W\w]*?<ul>([\W\w]*?)</ul`)
	matchesDefinitions := patternDefinitions.FindStringSubmatch(htmlContent)

	if matchesDefinitions != nil && len(matchesDefinitions) >= 2 {
		definitions := parseDefinitions(matchesDefinitions[1])
		wordInfo.Definitions = definitions
	}

	// 3. 解析翻译
	patternTranslation := regexp.MustCompile(`fanyi_contentWrp"[\W\w]*?翻译结果[\W\w]*?trans-container[\W\w]*?<p>[\W\w]*?</p>([\W\w]*?)</p>`)
	matchesTranslation := patternTranslation.FindStringSubmatch(htmlContent)

	if matchesTranslation != nil && len(matchesTranslation) >= 2 {
		wordInfo.Translation = cleanText(matchesTranslation[1])
	}

	return wordInfo, nil
}

// cleanText 清理文本，移除 HTML 标签和多余空格
func cleanText(text string) string {
	// 移除 HTML 标签
	re := regexp.MustCompile(`<[^>]*>`)
	text = re.ReplaceAllString(text, "")

	// 替换 HTML 实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// 清理多余空白
	text = strings.TrimSpace(text)
	reWhitespace := regexp.MustCompile(`\s+`)
	text = reWhitespace.ReplaceAllString(text, " ")

	return text
}

// parseDefinitions 解析定义列表
func parseDefinitions(html string) []string {
	// 提取 li 标签内容
	reLi := regexp.MustCompile(`<li>([\W\w]*?)</li>`)
	matches := reLi.FindAllStringSubmatch(html, -1)

	definitions := []string{}
	for _, match := range matches {
		if len(match) >= 2 {
			cleaned := cleanText(match[1])
			if cleaned != "" {
				definitions = append(definitions, cleaned)
			}
		}
	}

	return definitions
}

// GoogleTranslate 谷歌翻译
func GoogleTranslate(str string) (string, error) {
	// 判断目标语言
	tl := "zh"
	if ContainsChinese(str) {
		tl = "en"
	}

	// 构建请求URL
	apiUrl := fmt.Sprintf(
		"https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&ie=UTF-8&dj=1&q=%s",
		tl,
		url.QueryEscape(str),
	)

	// 发送HTTP请求
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiUrl)
	if err != nil {
		return "", fmt.Errorf("谷歌翻译请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析JSON
	var result GoogleTranslateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析JSON失败: %v", err)
	}

	// 构建翻译结果
	if len(result.Sentences) > 0 {
		var translations []string
		for _, sentence := range result.Sentences {
			if sentence.Trans != "" {
				translations = append(translations, sentence.Trans)
			}
		}
		return strings.Join(translations, ""), nil
	}

	return "", nil
}

// ContainsChinese 检查字符串是否包含中文
func ContainsChinese(str string) bool {
	for _, r := range str {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// DisplayWordInfo 在终端显示单词信息
func DisplayWordInfo(wordInfo *WordInfo) {
	// fmt.Printf("%s%s%s\n", BoldCyan, strings.Repeat("=", 60), Reset)
	// fmt.Printf("%s%-30s%s\n", Bold, "查询结果", Reset)
	// fmt.Printf("%s%s%s\n", BoldCyan, strings.Repeat("=", 60), Reset)

	// 显示单词
	fmt.Printf("%s输入:%s %s%s%s\n", BoldGreen, Reset, BoldYellow, wordInfo.Word, Reset)
	// 显示发音
	if wordInfo.UkPhonetic != "" {
		fmt.Printf("%s发音:%s", BoldGreen, Reset)
		fmt.Printf(" %s英式%s %s%s%s %s美式%s %s%s%s\n", Cyan, Reset, Yellow, wordInfo.UkPhonetic, Reset, Cyan, Reset, Yellow, wordInfo.UsPhonetic, Reset)
		// if wordInfo.UkAudioURL != "" {
		// 	fmt.Printf("  %s英式发音URL:%s %s\n", Cyan, Reset, wordInfo.UkAudioURL)
		// }
		// if wordInfo.UsAudioURL != "" {
		// 	fmt.Printf("  %s美式发音URL:%s %s\n", Cyan, Reset, wordInfo.UsAudioURL)
		// }
	} else if wordInfo.SinglePhonetic != "" {
		// fmt.Printf("%s发音:%s %s%s%s\n", BoldGreen, Reset, Yellow, wordInfo.SinglePhonetic, Reset)
		// if wordInfo.SingleAudioURL != "" {
		// 	fmt.Printf("%s发音URL:%s %s\n", Cyan, Reset, wordInfo.SingleAudioURL)
		// }
	} else {
		// fmt.Printf("%s发音:%s %s未找到%s\n", BoldGreen, Reset, Red, Reset)
	}
	// fmt.Println()
	// 显示释义
	if len(wordInfo.Definitions) > 0 {
		fmt.Printf("%s释义:%s\n", BoldGreen, Reset)
		for i, def := range wordInfo.Definitions {
			// 为每个释义编号并添加颜色
			index := fmt.Sprintf("%s%d.%s", Cyan, i+1, Reset)
			fmt.Printf("  %s %s%s%s\n", index, Green, def, Reset)
		}
	} else {
		fmt.Printf("%s释义:%s %s未找到%s\n", BoldGreen, Reset, Red, Reset)
	}
	// fmt.Println()
	// 显示有道翻译
	if wordInfo.Translation != "" {
		fmt.Printf("%s有道翻译:%s\n", BoldGreen, Reset)
		fmt.Printf("  %s%s%s\n", Magenta, wordInfo.Translation, Reset)
	}
	// fmt.Println()
}

// DisplayTranslation 在终端显示翻译结果
func DisplayTranslation(original, googleResult, cnkiResult string) {
	if googleResult != "" {
		fmt.Println("")
		fmt.Printf("%s谷歌翻译:%s\n", BoldGreen, Reset)
		fmt.Printf("  %s%s%s\n", Green, googleResult, Reset)
	}

	if cnkiResult != "" {
		fmt.Println("")
		fmt.Printf("%sCNKI翻译:%s\n", BoldGreen, Reset)
		fmt.Printf("  %s%s%s\n", Green, cnkiResult, Reset)
	}
}

// DisplaySeparator 显示分隔线
func DisplaySeparator() {
	fmt.Printf("%s%s%s\n", Cyan, strings.Repeat("-", 60), Reset)
}

// DisplayError 显示错误信息
func DisplayError(message string, err error) {
	// fmt.Printf("%s错误: %s - %v%s\n", Red, message, err, Reset)
	fmt.Printf("\n%s错误: %s\n", Red, message)
}

// DisplaySuccess 显示成功信息
func DisplaySuccess(message string) {
	fmt.Printf("%s✓ %s%s\n", Green, message, Reset)
}

// DisplayLoading 显示加载信息
func DisplayLoading(message string) {
	fmt.Printf("%s⏳ %s...%s\n", Yellow, message, Reset)
}

// 主函数
func Search(word string) {
	// fmt.Printf("%s查询单词: %s%s%s\n", Bold, Green, word, Reset)
	DisplaySeparator()

	// 并发请求谷歌和CNKI翻译
	chanList := make(chan bool, 2)

	go func() {
		if ini.String("translate.tr_google") != "disable" {
			// 谷歌翻译
			googleTrans, err := GoogleTranslate(word)
			if err != nil {
				DisplayError("谷歌翻译失败", err)
			}
			DisplayTranslation(word, googleTrans, "")
		}
		chanList <- true
	}()
	go func() {
		if ini.String("translate.tr_cnki") != "disable" {
			// CNKI翻译
			cnkiTrans, err := CNKITranslate(word)
			if err != nil {
				DisplayError("CNKI翻译失败", err)
			}
			DisplayTranslation(word, "", cnkiTrans)
		}

		chanList <- true
	}()

	// 查询单词信息
	// DisplayLoading("正在查询有道词典")
	wordInfo, err := SearchWord(word)
	if err != nil {
		DisplayError("查询失败", err)
		DisplaySeparator()
		return
	}
	// DisplaySuccess("有道词典查询完成")

	// 显示单词信息
	DisplayWordInfo(wordInfo)

	// 等待所有翻译完成
	for i := 0; i < 2; i++ {
		<-chanList
	}

	defer DisplaySeparator()
}
