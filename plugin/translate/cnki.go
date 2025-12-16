package plugin_translate

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var cookie string

type TokenResp struct {
	Code int    `json:"code"`
	Data string `json:"data"`
}

type TranslateResp struct {
	Code int `json:"code"`
	Data struct {
		MResult string `json:"mResult"`
	} `json:"data"`
}

// PKCS7Padding 实现PKCS7填充
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// PKCS7UnPadding 去除PKCS7填充
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

// AESECBEncrypt AES ECB模式加密
func AESECBEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// ECB模式需要手动处理块
	blockSize := block.BlockSize()
	plaintext = PKCS7Padding(plaintext, blockSize)
	ciphertext := make([]byte, len(plaintext))

	for bs, be := 0, blockSize; bs < len(plaintext); bs, be = bs+blockSize, be+blockSize {
		block.Encrypt(ciphertext[bs:be], plaintext[bs:be])
	}

	return ciphertext, nil
}
func aesEncrypt(text string) (string, error) {
	// 密钥
	n := "4e87183cfd3a45fe"

	// 将字符串转换为字节数组
	plaintext := []byte(text)
	key := []byte(n)

	// AES ECB加密
	encrypted, err := AESECBEncrypt(plaintext, key)
	if err != nil {
		return "", err
	}

	// Base64编码
	base64Str := base64.StdEncoding.EncodeToString(encrypted)

	// 字符替换："/" -> "_", "+" -> "-"
	r := strings.ReplaceAll(base64Str, "/", "_")
	r = strings.ReplaceAll(r, "+", "-")

	return r, nil
}
func getToken() (string, error) {
	if cookie == "" {
		cookie = getCookie()
	}

	client := http.Client{Timeout: 3 * time.Second}
	// 创建自定义请求
	req, err := http.NewRequest("GET", "https://dict.cnki.net/fyzs-front-api/getToken", nil)
	if err != nil {
		return "", err
	}
	// 添加 JSON 请求头
	// req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "dict.cnki.net")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Cache-Control", "max-age=0")

	// // 可以添加其他需要的header，例如：
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36 Edg/136.0.0.0")

	// 发送请求
	resp, err := client.Do(req)
	// fmt.Println(resp)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tr TokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}

	if tr.Code != 200 {
		return "", fmt.Errorf("token获取失败: %d", tr.Code)
	}
	// 记录返回的 cookie
	resultCookie := resp.Header["Set-Cookie"]
	// fmt.Println(resultCookie)
	for _, v := range resultCookie {
		setCookie(v)
	}

	return tr.Data, nil
}

func CNKITranslate(text string) (string, error) {
	token, err := getToken()

	// fmt.Println(token)
	if err != nil {
		return "", err
	}

	encText, err := aesEncrypt(text)
	if err != nil {
		return "", err
	}

	// fmt.Println(encText)
	// encText = "01N1X4IEWxdx7h6em8SnzQ=="

	reqBody, _ := json.Marshal(map[string]interface{}{
		"words": encText,
	})

	// fmt.Println(string(reqBody))

	req, err := http.NewRequest("POST",
		"https://dict.cnki.net/fyzs-front-api/translate/literaltranslation",
		bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "dict.cnki.net")
	req.Header.Set("Token", token)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Cache-Control", "max-age=0")

	// client := &http.Client{}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)

	// fmt.Println(resp)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tr TranslateResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}

	if tr.Code != 200 {
		// fmt.Println(tr)
		return "", fmt.Errorf("翻译失败: %d", tr.Code)
	}

	cleaned := regexp.MustCompile(`\s*\(.*智联招聘.*\)$`).ReplaceAllString(tr.Data.MResult, "")

	// 记录返回的 cookie
	resultCookie := resp.Header["Set-Cookie"]
	// fmt.Println(resultCookie)
	for _, v := range resultCookie {
		setCookie(v)
	}

	return cleaned, nil
}

func getCookie() string {
	// 从 /tmp/v_cookie 文件中读取 cookie
	cookie, err := ioutil.ReadFile("/tmp/v_cookie")
	if err != nil {
		return ""
	}
	// fmt.Println("getCookie", string(cookie))
	return string(cookie)
}

func setCookie(c string) {
	// 判断其中是否有 SF_cookie_97
	if !strings.Contains(c, "SF_cookie_97") {
		return
	}
	c = strings.Split(c, ";")[0]
	// 将 cookie 写入 /tmp/v_cookie 文件
	ioutil.WriteFile("/tmp/v_cookie", []byte(c), 0644)
	cookie = c
}
