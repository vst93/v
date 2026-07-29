package plugin_gcm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gookit/ini/v2"
)

// maxDiffChars caps the diff sent to the model so a large changeset
// cannot overflow the provider's token limit.
const maxDiffChars = 50000

const defaultBaseURL = "https://api.openai.com/v1"
const defaultModel = "gpt-4o-mini"

const systemPrompt = `You are a commit message generator. Analyze the provided git diff and generate a concise commit message following Conventional Commits format.

Rules:
- Format: type(scope): description (scope optional)
- Types: feat, fix, refactor, docs, style, test, chore, perf, ci, build
- 1-3 lines max. Simple changes = 1 line only.
- No explanations, no signatures, no markdown.
- Describe WHAT changed, not WHY (the diff shows why).
- Output the commit message only, nothing else.`

// git diff sources
const (
	sourceStaged   = "staged"
	sourceUnstaged = "unstaged"
	sourceAll      = "all"
)

type Gcm struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (g *Gcm) Init() error {
	g.name = "gcm"
	g.version = "1.0.0"
	g.description = "Generate commit message from staged changes using AI"
	g.command = "gcm"
	g.args = map[string]string{
		"-add":  "Stage all changes (git add .) before generating",
		"-p":    "Read diff from stdin/pipe instead of running git",
		"-c":    "Copy the generated message to clipboard",
		"-u":    "Use unstaged changes (git diff) instead of staged",
		"-a":    "Use all changes vs HEAD (git diff HEAD)",
		"-pipe": "Pipe payload (auto-appended when stdin is a pipe)",
	}
	g.author = "vst"
	return nil
}

func (g *Gcm) GetName() string            { return g.name }
func (g *Gcm) GetVersion() string         { return g.version }
func (g *Gcm) GetDescription() string     { return g.description }
func (g *Gcm) GetCommand() string         { return g.command }
func (g *Gcm) GetArgs() map[string]string { return g.args }
func (g *Gcm) GetAuthor() string          { return g.author }
func (g *Gcm) Stop() error                { return nil }

func (g *Gcm) Run(args []string) error {
	var (
		stageAll bool
		usePipe  bool
		toClip   bool
		source   = sourceStaged
		pipeData string
		hasPipe  bool
	)

	// Parse args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-add", "--add":
			stageAll = true
		case "-p", "--pipe-mode":
			usePipe = true
		case "-c", "--copy":
			toClip = true
		case "-u", "--unstaged":
			source = sourceUnstaged
		case "-a", "--all":
			source = sourceAll
		case "-pipe", "--pipe":
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "--help":
			g.printHelp()
			return nil
		}
	}

	// Gather the diff
	var diff string
	switch {
	case hasPipe:
		diff = pipeData
	case usePipe:
		return fmt.Errorf("no pipe input received. Use: git diff --cached | v gcm -p")
	default:
		if stageAll {
			if err := runGitAddAll(); err != nil {
				return err
			}
			fmt.Println("📦 Staged all changes (git add .)")
		}
		out, err := runGitDiff(source)
		if err != nil {
			return err
		}
		diff = out
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Print(emptyDiffMessage(source, hasPipe))
		return nil
	}

	apiKey := ini.Default().String("gcm.api_key", "")
	if apiKey == "" {
		fmt.Print(missingKeyMessage())
		return nil
	}
	baseURL := ini.Default().String("gcm.base_url", defaultBaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := ini.Default().String("gcm.model", defaultModel)
	if model == "" {
		model = defaultModel
	}

	fmt.Printf("📦 Generating commit message (%s)...\n\n", model)

	message, err := generate(baseURL, apiKey, model, truncateDiff(diff))
	if err != nil {
		return err
	}
	if message == "" {
		return fmt.Errorf("the model returned an empty commit message")
	}

	fmt.Println(message)

	if toClip {
		if err := clipboard.WriteAll(message); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("\n✅ Copied to clipboard")
	}
	return nil
}

// runGitDiff shells out to git for the requested change source.
func runGitDiff(source string) (string, error) {
	var gitArgs []string
	switch source {
	case sourceUnstaged:
		gitArgs = []string{"diff"}
	case sourceAll:
		gitArgs = []string{"diff", "HEAD"}
	default:
		gitArgs = []string{"diff", "--cached"}
	}

	cmd := exec.Command("git", gitArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("failed to run git %s: %w: %s", strings.Join(gitArgs, " "), err, detail)
		}
		return "", fmt.Errorf("failed to run git %s: %w", strings.Join(gitArgs, " "), err)
	}
	return string(out), nil
}

// runGitAddAll stages all changes (git add .) so the subsequent
// git diff --cached captures everything.
func runGitAddAll() error {
	cmd := exec.Command("git", "add", ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("failed to run git add .: %w: %s", err, detail)
		}
		return fmt.Errorf("failed to run git add .: %w", err)
	}
	return nil
}

// truncateDiff keeps the payload under maxDiffChars, appending a note when cut.
func truncateDiff(diff string) string {
	if len(diff) <= maxDiffChars {
		return diff
	}
	var b strings.Builder
	b.WriteString(diff[:maxDiffChars])
	b.WriteString("\n\n[diff truncated: showing the first ")
	b.WriteString(fmt.Sprintf("%d of %d", maxDiffChars, len(diff)))
	b.WriteString(" characters]")
	return b.String()
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// generate posts the diff to an OpenAI-compatible chat completions endpoint.
func generate(baseURL, apiKey, model, diff string) (string, error) {
	payload := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: diff},
		},
		MaxTokens:   200,
		Temperature: 0.3,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %w", err)
	}

	endpoint := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	var parsed chatResponse
	raw, err := readAll(resp)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("failed to decode response (HTTP %d): %w: %s", resp.StatusCode, err, truncateForError(string(raw)))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("api error (HTTP %d): %s", resp.StatusCode, parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api returned HTTP %d: %s", resp.StatusCode, truncateForError(string(raw)))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("api returned no choices: %s", truncateForError(string(raw)))
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func readAll(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return buf.Bytes(), nil
}

// truncateForError keeps error output readable when a provider returns HTML or a large body.
func truncateForError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 300 {
		return s
	}
	return s[:300] + "..."
}

func emptyDiffMessage(source string, fromPipe bool) string {
	var b strings.Builder
	if fromPipe {
		b.WriteString("📦 The piped diff is empty, nothing to describe.\n")
		return b.String()
	}

	switch source {
	case sourceUnstaged:
		b.WriteString("📦 No unstaged changes found.\n\n")
		b.WriteString("Your working tree is clean, or everything is already staged:\n")
		b.WriteString("  v gcm       Use staged changes (git diff --cached)\n")
		b.WriteString("  v gcm -a    Use all changes vs HEAD (git diff HEAD)\n")
	case sourceAll:
		b.WriteString("📦 No changes found against HEAD.\n\n")
		b.WriteString("Edit some files first, then run v gcm again.\n")
	default:
		b.WriteString("📦 No staged changes found.\n\n")
		b.WriteString("Stage your changes first:\n")
		b.WriteString("  git add <file>    Stage specific files\n")
		b.WriteString("  git add .         Stage everything\n\n")
		b.WriteString("Or read from another source:\n")
		b.WriteString("  v gcm -u          Use unstaged changes (git diff)\n")
		b.WriteString("  v gcm -a          Use all changes vs HEAD (git diff HEAD)\n")
	}
	return b.String()
}

func missingKeyMessage() string {
	var b strings.Builder
	b.WriteString("🔑 No API key configured for gcm.\n\n")
	b.WriteString("Add a [gcm] section to ~/.v_tools/settings.ini:\n\n")
	b.WriteString("[gcm]\n")
	b.WriteString("api_key = sk-xxx\n")
	b.WriteString("base_url = https://api.openai.com/v1\n")
	b.WriteString("model = gpt-4o-mini\n\n")
	b.WriteString("⚙ base_url and model are optional; they default to the values above.\n")
	return b.String()
}

func (g *Gcm) printHelp() {
	fmt.Printf("gcm - Generate Commit Message v%s\n\n", g.version)
	fmt.Println("Usage:")
	fmt.Println("  v gcm                          Generate from staged changes (git diff --cached)")
	fmt.Println("  v gcm -add                     Stage all (git add .) then generate")
	fmt.Println("  v gcm -u                       Generate from unstaged changes (git diff)")
	fmt.Println("  v gcm -a                       Generate from all changes vs HEAD (git diff HEAD)")
	fmt.Println("  v gcm -c                       Generate and copy to clipboard")
	fmt.Println("  git diff --cached | v gcm -p   Generate from piped diff")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -add   Stage all changes (git add .) before generating")
	fmt.Println("  -p     Read diff from stdin/pipe instead of running git")
	fmt.Println("  -c     Copy the generated message to clipboard")
	fmt.Println("  -u     Use unstaged changes (git diff)")
	fmt.Println("  -a     Use all changes vs HEAD (git diff HEAD)")
	fmt.Println("  -h     Show this help")
	fmt.Println()
	fmt.Println("Short command: gc (alias for gcm)")
	fmt.Println()
	fmt.Println("Configuration (~/.v_tools/settings.ini):")
	fmt.Println("  [gcm]")
	fmt.Println("  api_key = sk-xxx")
	fmt.Println("  base_url = https://api.openai.com/v1")
	fmt.Println("  model = gpt-4o-mini")
	fmt.Println()
	fmt.Printf("Diffs longer than %d characters are truncated before the API call.\n", maxDiffChars)
}
