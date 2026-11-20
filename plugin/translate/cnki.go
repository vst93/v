package plugin_translate

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
)

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

type CNKITranslateRequest struct {
	Words         string `json:"words"`
	TranslateType string `json:"translateType"`
}

func PKCS7Padding(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	return append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func AESECBEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	plaintext = PKCS7Padding(plaintext, blockSize)
	ciphertext := make([]byte, len(plaintext))
	for start := 0; start < len(plaintext); start += blockSize {
		block.Encrypt(ciphertext[start:start+blockSize], plaintext[start:start+blockSize])
	}
	return ciphertext, nil
}

func aesEncrypt(text string) (string, error) {
	encrypted, err := AESECBEncrypt([]byte(text), []byte("4e87183cfd3a45fe"))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

func getToken(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://dict.cnki.net/fyzs-front-api/getToken", nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	var result TokenResp
	if err := decodeResponse(client, req, &result); err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	if result.Code != 200 || strings.TrimSpace(result.Data) == "" {
		return "", fmt.Errorf("get token failed (code %d)", result.Code)
	}
	return result.Data, nil
}

func CNKITranslate(text string) (string, error) {
	return cnkiTranslate(text, targetLanguage(text, ""))
}

func cnkiRequest(text, target, token string) (*http.Request, error) {
	encText, err := aesEncrypt(text)
	if err != nil {
		return nil, fmt.Errorf("encrypt text: %w", err)
	}
	// CNKI's language selector uses "0" for English and "1" for Chinese.
	translateType := "1"
	if target == "en" {
		translateType = "0"
	}
	body, err := json.Marshal(CNKITranslateRequest{Words: encText, TranslateType: translateType})
	if err != nil {
		return nil, fmt.Errorf("encode translation request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost,
		"https://dict.cnki.net/fyzs-front-api/translate/literaltranslation", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create CNKI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)
	return req, nil
}

func cnkiTranslate(text, target string) (string, error) {
	return cnkiTranslateContext(context.Background(), text, target)
}

func cnkiTranslateContext(ctx context.Context, text, target string) (string, error) {
	// Keep token cookies within this translation, avoiding shared global state
	// and a platform-specific /tmp cookie file.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("create cookie jar: %w", err)
	}
	client := &http.Client{Timeout: requestTimeout, Jar: jar}
	token, err := getToken(ctx, client)
	if err != nil {
		return "", err
	}
	req, err := cnkiRequest(text, target, token)
	if err != nil {
		return "", err
	}
	var result TranslateResp
	if err := decodeResponse(client, req.WithContext(ctx), &result); err != nil {
		return "", err
	}
	return result.translation()
}

var cnkiAdSuffix = regexp.MustCompile(`\s*\(.*智联招聘.*\)$`)

func (r TranslateResp) translation() (string, error) {
	if r.Code != 200 {
		return "", fmt.Errorf("translation failed (code %d)", r.Code)
	}
	text := cnkiAdSuffix.ReplaceAllString(r.Data.MResult, "")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty translation")
	}
	return text, nil
}
