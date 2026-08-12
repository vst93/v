package plugin_vc

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	qrcode "github.com/skip2/go-qrcode"
)

// ANSI color codes - terminal-native, no HTML tags.
const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cBlue   = "\033[34m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
)

const (
	defaultHost  = "https://meimingzi.top"
	pluginVer    = "1.0.0"
	pollInterval = 3 * time.Second
	keyLen       = 32
)

type Vc struct {
	name        string
	description string
	command     string
	args        map[string]string
	author      string
}

func (p *Vc) Init() error {
	p.name = "vc"
	p.description = "v-connection: multi-device text sharing via a channel"
	p.command = "vc"
	p.args = map[string]string{
		"-new":  "Create a random channel (default)",
		"-join": "Join a channel with a 4-char code",
		"-copy": "Auto-copy received text to clipboard",
		"-host": "Custom server host (default: " + defaultHost + ")",
		"-h":    "Show help",
	}
	p.author = "vst"
	return nil
}

func (p *Vc) GetName() string            { return p.name }
func (p *Vc) GetVersion() string         { return pluginVer }
func (p *Vc) GetDescription() string     { return p.description }
func (p *Vc) GetCommand() string         { return p.command }
func (p *Vc) GetArgs() map[string]string { return p.args }
func (p *Vc) GetAuthor() string          { return p.author }
func (p *Vc) GetAliases() []string       { return nil }
func (p *Vc) Stop() error                { return nil }

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

type apiResponse struct {
	Data json.RawMessage `json:"data"`
}

type messageItem struct {
	Ts   int64  `json:"ts"`
	Text string `json:"text"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func sendMessage(host, key, text string) error {
	body := fmt.Sprintf("key=%s&text=%s&mode=", key, urlEncode(text))
	req, err := http.NewRequest("POST", host+"/vc/add", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func getMessages(host, key string, ts int64) ([]messageItem, error) {
	u := fmt.Sprintf("%s/vc/get?key=%s&t=%d&mode=", host, key, ts)
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return nil, err
	}
	var items []messageItem
	if err := json.Unmarshal(api.Data, &items); err != nil {
		return nil, nil
	}
	return items, nil
}

func getShortCode(host, key string) (string, error) {
	u := fmt.Sprintf("%s/vc/getCode?key=%s", host, key)
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return "", err
	}
	var code string
	if err := json.Unmarshal(api.Data, &code); err != nil {
		return "", err
	}
	return code, nil
}

func getLink(host, code string) (string, error) {
	u := fmt.Sprintf("%s/vc/getLink?code=%s", host, code)
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return "", err
	}
	var key string
	if err := json.Unmarshal(api.Data, &key); err != nil {
		return "", err
	}
	return key, nil
}

// ---------------------------------------------------------------------------
// Run - entry point
// ---------------------------------------------------------------------------

func (p *Vc) Run(args []string) error {
	var (
		mode     string
		joinCode string
		host     = defaultHost
		autoCopy bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-new":
			mode = "new"
		case "-join":
			mode = "join"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				joinCode = args[i+1]
				i++
			}
		case "-copy":
			autoCopy = true
		case "-host":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "-h", "-help", "--help":
			p.printHelp()
			return nil
		}
	}

	if mode == "" {
		mode = "new"
	}

	switch mode {
	case "join":
		return p.runJoin(host, joinCode, autoCopy)
	default:
		return p.runNew(host, autoCopy)
	}
}

// ---------------------------------------------------------------------------
// Mode: create a new channel
// ---------------------------------------------------------------------------

func (p *Vc) runNew(host string, autoCopy bool) error {
	key := randomStr(keyLen)
	channelURL := host + "/w/?s=" + key

	fmt.Printf("%s%s━━━ Generate Channel ━━━%s\n", cBlue, cBold, cReset)
	fmt.Printf("%s✓ Channel key generated%s\n\n", cGreen, cReset)

	printQR(channelURL)
	fmt.Printf("%sLink: %s%s\n", cCyan, channelURL, cReset)

	if code, err := getShortCode(host, key); err == nil && code != "" {
		fmt.Printf("%sCode: %s%s%s %s(valid 5 min)%s\n\n",
			cGreen, cBold, code, cReset, cYellow, cReset)
	} else {
		fmt.Println()
	}

	return p.channelInteract(host, key, channelURL, autoCopy)
}

// ---------------------------------------------------------------------------
// Mode: join via short code
// ---------------------------------------------------------------------------

func (p *Vc) runJoin(host, code string, autoCopy bool) error {
	if code == "" {
		fmt.Print("Enter 4-char code: ")
		reader := bufio.NewReader(os.Stdin)
		code, _ = reader.ReadString('\n')
		code = strings.TrimSpace(code)
	}

	if len(code) != 4 || !isAlnum(code) {
		return fmt.Errorf("invalid code: must be exactly 4 alphanumeric characters")
	}

	fmt.Print("Verifying code... ")
	key, err := getLink(host, code)
	if err != nil || key == "" {
		fmt.Printf("%sfailed%s\n", cRed, cReset)
		return fmt.Errorf("code invalid or expired (codes are valid for 5 minutes)")
	}

	fmt.Printf("%sOK!%s\n\n", cGreen, cReset)
	channelURL := host + "/w/?s=" + key

	printQR(channelURL)
	fmt.Printf("%sLink: %s%s\n\n", cCyan, channelURL, cReset)

	return p.channelInteract(host, key, channelURL, autoCopy)
}

// ---------------------------------------------------------------------------
// Interactive channel loop
// ---------------------------------------------------------------------------

func (p *Vc) channelInteract(host, key, channelURL string, autoCopy bool) error {
	if autoCopy && !hasClipboard() {
		fmt.Printf("%s⚠ No clipboard backend found; auto-copy disabled%s\n", cYellow, cReset)
		autoCopy = false
	}

	var lastTS int64
	var lastSent string

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	fmt.Printf("%s── Channel active ──%s\n", cGray, cReset)
	fmt.Printf("%sType + Enter to send, empty line to quit, ~help for commands%s\n\n", cGray, cReset)

	// Non-terminal (pipe) mode
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if err := sendMessage(host, key, line); err != nil {
				fmt.Printf("%sSend failed: %v%s\n", cRed, err, cReset)
			} else {
				fmt.Printf("%s✓ Sent: %s%s\n", cGreen, line, cReset)
			}
		}
		fmt.Printf("%sDone.%s\n", cGreen, cReset)
		return nil
	}

	// Terminal mode: interactive loop with polling
	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	inputCh := make(chan string)
	errCh := make(chan error, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			inputCh <- strings.TrimSpace(line)
		}
	}()

	for {
		select {
		case <-sigCh:
			fmt.Printf("\n%sBye~%s\n", cGreen, cReset)
			return nil

		case <-ticker.C:
			p.pollMessages(host, key, &lastTS, autoCopy)

		case line := <-inputCh:
			if line == "" {
				fmt.Printf("%sBye~%s\n", cGreen, cReset)
				return nil
			}

			switch line {
			case "~help":
				fmt.Printf("%sCommands: ~refresh  ~code  ~url  ~quit  ~help%s\n", cYellow, cReset)
				continue
			case "~quit", "~exit":
				fmt.Printf("%sBye~%s\n", cGreen, cReset)
				return nil
			case "~refresh":
				p.pollMessages(host, key, &lastTS, autoCopy)
				continue
			case "~code":
				if code, err := getShortCode(host, key); err == nil && code != "" {
					fmt.Printf("Code: %s%s%s\n", cBold, code, cReset)
				} else {
					fmt.Printf("%sFailed to get code%s\n", cYellow, cReset)
				}
				continue
			case "~url":
				fmt.Printf("Link: %s\n", channelURL)
				fmt.Println()
				printQR(channelURL)
				continue
			}

			if line == lastSent {
				fmt.Printf("%s⚠ Can't send the same text twice in a row%s\n", cYellow, cReset)
				continue
			}

			if err := sendMessage(host, key, line); err != nil {
				fmt.Printf("%sSend failed: %v%s\n", cRed, err, cReset)
			} else {
				lastSent = line
				fmt.Printf("%s%s[%s]%s %s✓ Sent%s\n",
					cGreen, cBold, time.Now().Format("15:04:05"), cReset, cGreen, cReset)
			}

		case err := <-errCh:
			if err == io.EOF {
				fmt.Printf("\n%sBye~%s\n", cGreen, cReset)
				return nil
			}
			return err
		}
	}
}

// ---------------------------------------------------------------------------
// Poll
// ---------------------------------------------------------------------------

func (p *Vc) pollMessages(host, key string, lastTS *int64, autoCopy bool) {
	items, err := getMessages(host, key, *lastTS)
	if err != nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		if item.Ts <= *lastTS {
			continue
		}
		*lastTS = item.Ts
		timeStr := time.Unix(item.Ts, 0).Format("15:04:05")

		if autoCopy {
			if err := clipboard.WriteAll(item.Text); err == nil {
				fmt.Printf("%s%s[%s]%s %scopied → %s%s\n",
					cGreen, cBold, timeStr, cReset, cCyan, preview(item.Text), cReset)
				continue
			}
		}
		fmt.Printf("%s%s[%s]%s %s\n", cGreen, cBold, timeStr, cReset, item.Text)
	}
}

// ---------------------------------------------------------------------------
// QR code - pure Go, no external binary needed
// ---------------------------------------------------------------------------

func printQR(text string) {
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return
	}
	// ToSmallString uses half-height Unicode blocks for a compact terminal QR.
	fmt.Println(qr.ToSmallString(false))
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func randomStr(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	max := big.NewInt(int64(len(charset)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			sb.WriteByte(charset[time.Now().UnixNano()%int64(len(charset))])
			continue
		}
		sb.WriteByte(charset[idx.Int64()])
	}
	return sb.String()
}

func isAlnum(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func preview(text string) string {
	runes := []rune(text)
	if len(runes) <= 10 {
		return text
	}
	return string(runes[:3]) + "..." + string(runes[len(runes)-3:])
}

func urlEncode(s string) string {
	var sb strings.Builder
	for _, b := range []byte(s) {
		switch {
		case (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' || b == '~':
			sb.WriteByte(b)
		default:
			fmt.Fprintf(&sb, "%%%02X", b)
		}
	}
	return sb.String()
}

func hasClipboard() bool {
	return !clipboard.Unsupported
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func (p *Vc) printHelp() {
	fmt.Printf("%s--------------------------------------------------%s\n", cGray, cReset)
	fmt.Printf("%svc - v-connection v%s%s\n\n", cCyan+cBold, pluginVer, cReset)
	fmt.Println("Multi-device text sharing via a channel. Create a channel,")
	fmt.Println("share the link/code/QR, and exchange text in real time.")
	fmt.Printf("%sUsage:%s\n", cBold, cReset)
	fmt.Println("  v vc                   Create a new channel (default)")
	fmt.Printf("  v vc %s-copy%s              Create with auto-copy received text\n", cGreen, cReset)
	fmt.Printf("  v vc %s-join%s              Join a channel (prompts for code)\n", cGreen, cReset)
	fmt.Printf("  v vc %s-join%s ab12         Join channel with code ab12\n", cGreen, cReset)
	fmt.Printf("  v vc %s-host%s https://...  Use a custom server\n", cGreen, cReset)
	fmt.Println()
	fmt.Printf("%sInteractive commands (during session):%s\n", cBold, cReset)
	fmt.Printf("  %s~help%s      Show available commands\n", cGreen, cReset)
	fmt.Printf("  %s~refresh%s   Poll for new messages now\n", cGreen, cReset)
	fmt.Printf("  %s~code%s      Show the short code again\n", cGreen, cReset)
	fmt.Printf("  %s~url%s       Show link + QR code again\n", cGreen, cReset)
	fmt.Printf("  %s~quit%s      Exit session\n", cGreen, cReset)
	fmt.Println()
	fmt.Printf("%sPipe mode: echo \"text\" | v vc -join ab12%s\n", cGray, cReset)
	fmt.Printf("%sServer: %s (override with -host)%s\n", cGray, defaultHost, cReset)
	fmt.Printf("%s--------------------------------------------------%s\n", cGray, cReset)
}
