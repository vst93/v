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
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

const (
	defaultHost = "https://meimingzi.top"
	version     = "1.0.0"
	pollInterval = 3 * time.Second
)

type Vc struct {
	name        string
	pluginVer   string
	description string
	command     string
	args        map[string]string
	author      string
}

func (p *Vc) Init() error {
	p.name = "vc"
	p.pluginVer = version
	p.description = "v-connection: multi-device text sharing via a channel"
	p.command = "vc"
	p.args = map[string]string{
		"-new":     "Create a random channel (default)",
		"-join":    "Join a channel with a 4-char code",
		"-copy":    "Auto-copy received text to clipboard",
		"-host":    "Custom server host (default: " + defaultHost + ")",
		"-h":       "Show help",
	}
	p.author = "vst"
	return nil
}

func (p *Vc) GetName() string            { return p.name }
func (p *Vc) GetVersion() string         { return p.pluginVer }
func (p *Vc) GetDescription() string     { return p.description }
func (p *Vc) GetCommand() string         { return p.command }
func (p *Vc) GetArgs() map[string]string { return p.args }
func (p *Vc) GetAuthor() string          { return p.author }
func (p *Vc) Stop() error                { return nil }

// ---------------------------------------------------------------------------
// API structs
// ---------------------------------------------------------------------------

type apiResponse struct {
	Data json.RawMessage `json:"data"`
}

type messageItem struct {
	Ts   int64  `json:"ts"`
	Text string `json:"text"`
}

// ---------------------------------------------------------------------------
// Run - entry point
// ---------------------------------------------------------------------------

func (p *Vc) Run(args []string) error {
	var (
		mode     string // "new", "join"
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
	key := randomStr(32)
	channelURL := host + "/w/?s=" + key

	color.Println("<fg=blue;op=bold>━━━ Generate Channel ━━━</>")
	color.Println("<green>✓ Channel key generated</>\n")

	// QR code (terminal) if qrencode is available
	if hasQRCode() {
		color.Println("<cyan>━━━ Scan QR Code ━━━</>")
		printQRCode(channelURL)
		color.Println("<cyan>━━━━━━━━━━━━━━━━━━━━</>\n")
	}
	color.Printf("<cyan>Link: %s</>\n", channelURL)

	// Short code
	code, err := getShortCode(host, key)
	if err == nil && code != "" {
		color.Printf("<green>Code: <bold>%s</></> <yellow>(valid 5 min)</>\n", code)
	}

	fmt.Println()
	return p.channelInteract(host, key, channelURL, autoCopy)
}

// ---------------------------------------------------------------------------
// Mode: join an existing channel via short code
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
		color.Println("<red>failed</>")
		return fmt.Errorf("code invalid or expired (codes are valid for 5 minutes)")
	}

	color.Println("<green>OK!</>")
	channelURL := host + "/w/?s=" + key
	fmt.Println()

	if hasQRCode() {
		color.Println("<cyan>━━━ Channel QR ━━━</>")
		printQRCode(channelURL)
		color.Println("<cyan>━━━━━━━━━━━━━━━━━━</>\n")
	}
	color.Printf("<cyan>Link: %s</>\n\n", channelURL)

	return p.channelInteract(host, key, channelURL, autoCopy)
}

// ---------------------------------------------------------------------------
// Interactive channel loop: send + poll
// ---------------------------------------------------------------------------

func (p *Vc) channelInteract(host, key, channelURL string, autoCopy bool) error {
	if autoCopy && !hasClipboard() {
		color.Println("<yellow>⚠ No clipboard tool found; auto-copy disabled</>")
		autoCopy = false
	}

	var lastTS int64
	var lastSent string

	// Graceful cleanup on Ctrl-C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	color.Println("<gray>── Channel active ──</>")
	color.Println("<gray>Type and Enter to send, empty line to quit, ~help for commands</>\n")

	// Non-terminal (pipe) mode: read all lines and send them
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if err := sendMessage(host, key, line); err != nil {
				color.Printf("<red>Send failed: %v</>\n", err)
			} else {
				color.Printf("<green>✓ Sent: %s</>\n", line)
			}
		}
		color.Println("<green>Done.</>")
		return nil
	}

	// Terminal mode: interactive loop with polling
	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Channel for input lines
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
			color.Println("\n<green>Bye~</>")
			return nil

		case <-ticker.C:
			p.pollMessages(host, key, &lastTS, autoCopy)

		case line := <-inputCh:
			if line == "" {
				color.Println("<green>Bye~</>")
				return nil
			}

			switch line {
			case "~help":
				color.Println("<yellow>Commands: ~refresh  ~code  ~url  ~quit  ~help</>")
				continue
			case "~quit", "~exit":
				color.Println("<green>Bye~</>")
				return nil
			case "~refresh":
				p.pollMessages(host, key, &lastTS, autoCopy)
				continue
			case "~code":
				code, err := getShortCode(host, key)
				if err == nil && code != "" {
					color.Printf("<bold>Code: %s</>\n", code)
				} else {
					color.Println("<yellow>Failed to get code</>")
				}
				continue
			case "~url":
				color.Printf("Link: %s\n", channelURL)
				if hasQRCode() {
					fmt.Println()
					printQRCode(channelURL)
				}
				continue
			}

			if line == lastSent {
				color.Println("<yellow>⚠ Can't send the same text twice in a row</>")
				continue
			}

			if err := sendMessage(host, key, line); err != nil {
				color.Printf("<red>Send failed: %v</>\n", err)
			} else {
				lastSent = line
				color.Printf("<green><bold>[%s]</> <green>✓ Sent</>\n", time.Now().Format("15:04:05"))
			}

		case err := <-errCh:
			if err == io.EOF {
				color.Println("\n<green>Bye~</>")
				return nil
			}
			return err
		}
	}
}

// ---------------------------------------------------------------------------
// Poll for new messages
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
		copied := false
		if autoCopy {
			if err := clipboard.WriteAll(item.Text); err == nil {
				copied = true
			}
		}

		if copied {
			color.Printf("<green><bold>[%s]</> <cyan>copied → %s</>\n", timeStr, preview(item.Text))
		} else {
			color.Printf("<green><bold>[%s]</> %s\n", timeStr, item.Text)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

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
	url := fmt.Sprintf("%s/vc/get?key=%s&t=%d&mode=", host, key, ts)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var api apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return nil, err
	}

	// data can be an array or empty
	var items []messageItem
	if err := json.Unmarshal(api.Data, &items); err != nil {
		return nil, nil // no data
	}
	return items, nil
}

func getShortCode(host, key string) (string, error) {
	url := fmt.Sprintf("%s/vc/getCode?key=%s", host, key)
	resp, err := httpClient.Get(url)
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
	url := fmt.Sprintf("%s/vc/getLink?code=%s", host, code)
	resp, err := httpClient.Get(url)
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
// Utility functions
// ---------------------------------------------------------------------------

// randomStr generates a cryptographically secure random alphanumeric string.
func randomStr(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	max := big.NewInt(int64(len(charset)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			// Fallback should never happen, but be safe
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
	// atotto/clipboard handles xclip, wl-copy, pbcopy, and termux.
	// Unsupported is set true when no clipboard backend is found.
	return !clipboard.Unsupported
}

// ---------------------------------------------------------------------------
// QR code (delegates to system qrencode if available)
// ---------------------------------------------------------------------------

func hasQRCode() bool {
	_, err := exec.LookPath("qrencode")
	return err == nil
}

func printQRCode(text string) {
	// Use qrencode -t UTF8 for terminal output
	cmd := exec.Command("qrencode", "-t", "UTF8", text)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func (p *Vc) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>vc - v-connection v%s</>\n\n", p.pluginVer)
	color.Println("Multi-device text sharing via a channel. Create a channel,")
	color.Println("share the link/code/QR, and exchange text in real time.\n")
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v vc                   Create a new channel (default)")
	color.Println("  v vc <green>-copy</>              Create with auto-copy received text")
	color.Println("  v vc <green>-join</>              Join a channel (prompts for code)")
	color.Println("  v vc <green>-join</> ab12         Join channel with code ab12")
	color.Println("  v vc <green>-host</> https://...  Use a custom server\n")
	color.Println("<fg=magenta;op=bold>Interactive commands (during session):</>")
	color.Println("  <green>~help</>      Show available commands")
	color.Println("  <green>~refresh</>   Poll for new messages now")
	color.Println("  <green>~code</>      Show the short code again")
	color.Println("  <green>~url</>       Show link + QR code again")
	color.Println("  <green>~quit</>      Exit session")
	color.Println()
	color.Println("<gray>Pipe mode: echo \"text\" | v vc -join ab12</>")
	color.Println("<gray>Server: " + defaultHost + " (override with -host)</>")
	color.Println("<gray>--------------------------------------------------</>")
}
