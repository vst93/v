package plugin_gcm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
	"github.com/gookit/ini/v2"
	"v/setting"
)

// maxDiffChars caps the diff sent to the model so a large changeset
// cannot overflow the provider's token limit.
const maxDiffChars = 50000

// maxRetries is the number of times to retry on transient failures
// (empty response, network timeout, 5xx).
const maxRetries = 2

const defaultBaseURL = "https://api.openai.com/v1"
const defaultModel = "gpt-4o-mini"

const systemPromptBase = `You are a commit message generator. Analyze the provided git diff and generate a concise commit message following Conventional Commits format.

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

// Gencm is the plugin struct. The command is "gencm" (generate commit message).
// The struct keeps the old name "Gcm" for minimal churn in test files.
type Gcm struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (g *Gcm) Init() error {
	g.name = "gencm"
	g.version = "1.0.0"
	g.description = "Generate commit message from staged changes using AI"
	g.command = "gencm"
	g.args = map[string]string{
		"-add":  "Stage all changes (git add .) before generating",
		"-u":    "Use unstaged changes (git diff) instead of staged",
		"-a":    "Use all changes vs HEAD (git diff HEAD)",
		"-C":    "Run as if git was started in <path> (default: current directory)",
		"-lang": "Commit message language: en, zh, or custom text",
		"-en":   "Generate in English (one-shot, does not change saved config)",
		"-zh":   "Generate in Chinese (one-shot, does not change saved config)",
		"-pipe": "Read diff from stdin/pipe instead of running git (auto-detected)",
		"-copy": "Copy the generated message to clipboard",
		"-h":    "Show help",
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
func (g *Gcm) GetAliases() []string       { return []string{"gc"} }
func (g *Gcm) Stop() error                { return nil }

func (g *Gcm) Run(args []string) error {
	var (
		stageAll bool
		wantPipe bool
		toClip   bool
		source   = sourceStaged
		pipeData string
		hasPipe  bool
		dir      string
		langFlag string
	)

	// Parse args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-add":
			stageAll = true
		case "-copy":
			toClip = true
		case "-u":
			source = sourceUnstaged
		case "-a":
			source = sourceAll
		case "-pipe":
			// main.go appends `-pipe <data>` when stdin is a pipe. A bare
			// -pipe means the user asked for pipe input but sent nothing.
			wantPipe = true
			if i+1 < len(args) {
				pipeData = args[i+1]
				hasPipe = true
				i++
			}
		case "-h", "-help", "--help":
			g.printHelp()
			return nil
		case "-C":
			if i+1 < len(args) {
				dir = expandHome(args[i+1])
				i++
			}
		case "-en":
			langFlag = "en"
		case "-zh":
			langFlag = "zh"
		case "-lang":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				langFlag = args[i+1]
				i++
			} else {
				// Bare -lang: show current setting and re-run setup.
				cur := ini.Default().String("gcm.lang", "")
				if cur == "" {
					cur = "en"
				}
				fmt.Printf("📦 Current language: %s\n", cur)
				fmt.Println()
				newLang := promptLangSetup()
				_ = newLang
				return nil
			}
		default:
			// A bare positional argument that is an existing directory is
			// treated as the git directory, so `v gencm ~/projects/app`
			// works as a shorthand for `v gencm -C ~/projects/app`.
			if !strings.HasPrefix(arg, "-") && dir == "" {
				if fi, err := os.Stat(expandHome(arg)); err == nil && fi.IsDir() {
					dir = expandHome(arg)
				}
			}
		}
	}

	// Resolve the commit message language.
	// Priority: -lang flag > settings.ini > default (en).
	lang := langFlag
	if lang == "" {
		lang = ini.Default().String("gcm.lang", "")
	}

	// First run in interactive mode: walk the user through language setup.
	if lang == "" && !hasPipe && !wantPipe {
		lang = promptLangSetup()
	}

	// Gather the diff
	var diff string
	var stat string
	switch {
	case hasPipe:
		diff = pipeData
		stat = diffFileSummary(diff)
	case wantPipe:
		return fmt.Errorf("no pipe input received. Use: git diff --cached | v gencm")
	default:
		// Check if we're in a git repository before running git commands
		if err := checkGitRepo(dir); err != nil {
			fmt.Print(notGitRepoMessage())
			return nil
		}
		if stageAll {
			if err := runGitAddAll(dir); err != nil {
				return err
			}
			fmt.Println("📦 Staged all changes (git add .)")
		}
		out, err := runGitDiff(dir, source)
		if err != nil {
			return err
		}
		diff = out
		stat, _ = runGitDiffStat(dir, source)
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

	// For large diffs, keep file-level structure instead of a flat cut.
	diff = condenseDiff(diff, maxDiffChars)

	fmt.Printf("📦 Generating commit message (%s)...\n\n", model)

	message, err := generateWithRetry(baseURL, apiKey, model, lang, stat, diff)
	if err != nil {
		return err
	}
	if message == "" {
		return fmt.Errorf("the model returned an empty commit message after %d attempts", maxRetries)
	}

	fmt.Println(message)

	// Pipe mode: print only (optional -copy to copy), no interactive menu
	if hasPipe {
		if toClip {
			if err := clipboard.WriteAll(message); err != nil {
				return fmt.Errorf("failed to copy to clipboard: %w", err)
			}
			fmt.Println("\n✅ Copied to clipboard")
		}
		return nil
	}

	// Non-pipe mode: -copy shortcut skips the menu, just copy
	if toClip {
		if err := clipboard.WriteAll(message); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("\n✅ Copied to clipboard")
		return nil
	}

	// Interactive action menu
	return g.actionMenu(message, dir, source, stageAll)
}

// actionMenu shows an interactive menu after generating a commit message.
func (g *Gcm) actionMenu(message, dir, source string, stageAll bool) error {
	fmt.Println("\n─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─")
	fmt.Println("📦 What would you like to do?")
	fmt.Println()
	fmt.Println("  1) Copy to clipboard (default)")
	fmt.Println("  2) Commit with this message")
	fmt.Println("  3) Commit and push")
	fmt.Println("  4) Do nothing")
	fmt.Println()
	fmt.Print("Choice [1-4]: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	if choice == "" {
		choice = "1"
	}

	switch choice {
	case "1", "":
		if err := clipboard.WriteAll(message); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Println("✅ Copied to clipboard")
	case "2":
		if source == sourceUnstaged || source == sourceAll {
			if err := runGitAddAll(dir); err != nil {
				return err
			}
		}
		if err := gitCommit(message, dir); err != nil {
			return err
		}
		fmt.Println("✅ Committed")
	case "3":
		if source == sourceUnstaged || source == sourceAll {
			if err := runGitAddAll(dir); err != nil {
				return err
			}
		}
		if err := gitCommit(message, dir); err != nil {
			return err
		}
		fmt.Println("✅ Committed")
		if err := gitPush(dir); err != nil {
			return err
		}
		fmt.Println("✅ Pushed")
	case "4":
		fmt.Println("👌 Done, no action taken")
	default:
		fmt.Println("👌 Unknown choice, message printed above")
	}
	return nil
}

// gitCommit runs git commit with the given message.
func gitCommit(message, dir string) error {
	args := []string{"commit", "-m", message}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	return nil
}

// gitPush runs git push.
func gitPush(dir string) error {
	args := []string{"push"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

// checkGitRepo returns nil if the directory is inside a git repository.
func checkGitRepo(dir string) error {
	args := []string{"rev-parse", "--is-inside-work-tree"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run()
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}

// notGitRepoMessage returns a friendly message when not in a git repository.
func notGitRepoMessage() string {
	return "📦 Not a git repository. Run `git init` or navigate to a git project first.\n"
}

// runGitDiff shells out to git for the requested change source.
func runGitDiff(dir, source string) (string, error) {
	var gitArgs []string
	switch source {
	case sourceUnstaged:
		gitArgs = []string{"diff"}
	case sourceAll:
		gitArgs = []string{"diff", "HEAD"}
	default:
		gitArgs = []string{"diff", "--cached"}
	}
	if dir != "" {
		gitArgs = append([]string{"-C", dir}, gitArgs...)
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

// runGitDiffStat returns a compact file-level summary (git diff --stat).
// Failures are non-fatal; an empty string is returned.
func runGitDiffStat(dir, source string) (string, error) {
	var gitArgs []string
	switch source {
	case sourceUnstaged:
		gitArgs = []string{"diff", "--stat"}
	case sourceAll:
		gitArgs = []string{"diff", "HEAD", "--stat"}
	default:
		gitArgs = []string{"diff", "--cached", "--stat"}
	}
	if dir != "" {
		gitArgs = append([]string{"-C", dir}, gitArgs...)
	}

	cmd := exec.Command("git", gitArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return string(out), nil
}

// runGitAddAll stages all changes (git add .) so the subsequent
// git diff --cached captures everything.
func runGitAddAll(dir string) error {
	gitArgs := []string{"add", "."}
	if dir != "" {
		gitArgs = append([]string{"-C", dir}, gitArgs...)
	}
	cmd := exec.Command("git", gitArgs...)
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

// condenseDiff reduces a large diff to fit within maxChars while keeping
// file-level structure. Each file gets a proportional share of the budget;
// oversized files are reduced to headers + a few changed lines per hunk.
// For extremely large diffs, it falls back to file headers only.
func condenseDiff(diff string, maxChars int) string {
	if len(diff) <= maxChars {
		return diff
	}

	files := splitDiffFiles(diff)
	if len(files) == 0 {
		return truncateDiff(diff, maxChars)
	}

	// Give each file a proportional budget.
	budget := maxChars / len(files)

	var result strings.Builder
	for _, f := range files {
		if len(f) <= budget {
			result.WriteString(f)
		} else if budget >= 200 {
			result.WriteString(trimFileDiff(f, budget))
		} else {
			// Too many files: keep headers + hunk headers only.
			result.WriteString(keepHeadersOnly(f))
		}
		result.WriteString("\n")
	}

	if result.Len() > maxChars {
		// Last resort: flat truncation.
		return truncateDiff(result.String(), maxChars)
	}
	return result.String()
}

// splitDiffFiles splits a unified diff into per-file sections.
func splitDiffFiles(diff string) []string {
	lines := strings.Split(diff, "\n")
	var files []string
	var cur strings.Builder
	started := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if started {
				files = append(files, cur.String())
				cur.Reset()
			}
			started = true
		}
		if started {
			cur.WriteString(line)
			cur.WriteString("\n")
		}
	}
	if cur.Len() > 0 {
		files = append(files, cur.String())
	}
	return files
}

// trimFileDiff keeps the file header and hunk headers, plus the first few
// changed lines (+/-) per hunk, trimming the rest to fit the budget.
func trimFileDiff(fileDiff string, budget int) string {
	lines := strings.Split(fileDiff, "\n")
	var result strings.Builder
	var changed int

	for _, line := range lines {
		// Always keep structural headers.
		if strings.HasPrefix(line, "diff --git ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@ ") {
			result.WriteString(line)
			result.WriteString("\n")
			changed = 0
			continue
		}

		// Keep changed lines (+ and -), limit per hunk.
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			changed++
			if changed <= 5 {
				result.WriteString(line)
				result.WriteString("\n")
			} else if changed == 6 {
				result.WriteString("... [more changes trimmed]\n")
			}
		}

		if result.Len() >= budget {
			break
		}
	}

	return result.String()
}

// keepHeadersOnly extracts just the file header and hunk headers,
// discarding all content. Used when there are too many files.
func keepHeadersOnly(fileDiff string) string {
	lines := strings.Split(fileDiff, "\n")
	var result strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") ||
			strings.HasPrefix(line, "@@ ") {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	return result.String()
}

// diffFileSummary builds a compact --stat-like summary from a raw diff
// text, used when the diff comes from a pipe and git --stat is unavailable.
func diffFileSummary(diff string) string {
	files := splitDiffFiles(diff)
	if len(files) == 0 {
		return ""
	}

	var b strings.Builder
	var totalAdd, totalDel int
	for _, f := range files {
		name := extractFileName(f)
		if name == "" {
			continue
		}
		add, del := countChanges(f)
		totalAdd += add
		totalDel += del
		b.WriteString(fmt.Sprintf(" %4d +%-3d -%-3d %s\n", add+del, add, del, name))
	}
	b.WriteString(fmt.Sprintf(" %d files changed, %d insertions(+), %d deletions(-)\n",
		len(files), totalAdd, totalDel))
	return b.String()
}

// extractFileName pulls the file path from a "diff --git a/x b/x" line.
func extractFileName(fileDiff string) string {
	for _, line := range strings.Split(fileDiff, "\n") {
		if strings.HasPrefix(line, "diff --git a/") {
			rest := strings.TrimPrefix(line, "diff --git a/")
			if i := strings.Index(rest, " b/"); i >= 0 {
				return rest[:i]
			}
			return rest
		}
	}
	return ""
}

// countChanges returns (added, deleted) line counts for a single file diff.
func countChanges(fileDiff string) (int, int) {
	var add, del int
	for _, line := range strings.Split(fileDiff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			add++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			del++
		}
	}
	return add, del
}

// truncateDiff keeps the payload under maxChars, appending a note when cut.
func truncateDiff(diff string, maxChars int) string {
	if len(diff) <= maxChars {
		return diff
	}
	var b strings.Builder
	b.WriteString(diff[:maxChars])
	b.WriteString("\n\n[diff truncated: showing the first ")
	b.WriteString(fmt.Sprintf("%d of %d", maxChars, len(diff)))
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
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// buildSystemPrompt returns the base prompt with a language directive appended.
// lang values: "en" or "" (English, default), "zh" (Chinese),
// or any other string treated as a custom instruction.
func buildSystemPrompt(lang string) string {
	switch lang {
	case "", "en":
		return systemPromptBase
	case "zh":
		return systemPromptBase + "\n- Write the commit message in Chinese."
	default:
		return systemPromptBase + "\n- " + lang
	}
}

// buildUserPrompt wraps the diff in a clear instruction. The stat gives
// the model a file-level overview; the diff provides the actual changes.
func buildUserPrompt(stat, diff string) string {
	var b strings.Builder
	b.WriteString("Generate a commit message for the following git diff. ")
	b.WriteString("Output ONLY the commit message, no explanation.\n\n")
	if strings.TrimSpace(stat) != "" {
		b.WriteString("---FILE CHANGES (git diff --stat)---\n")
		b.WriteString(stat)
		b.WriteString("---END FILE CHANGES---\n\n")
	}
	b.WriteString("---GIT DIFF---\n")
	b.WriteString(diff)
	b.WriteString("\n---END GIT DIFF---")
	return b.String()
}

// generateWithRetry calls the API up to maxRetries times. It retries on
// empty responses, transient network errors, and 5xx responses.
func generateWithRetry(baseURL, apiKey, model, lang, stat, diff string) (string, error) {
	userMsg := buildUserPrompt(stat, diff)
	sysPrompt := buildSystemPrompt(lang)
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("⏳ Retry %d/%d...\n", attempt-1, maxRetries-1)
			time.Sleep(1 * time.Second)
		}

		msg, err := generate(baseURL, apiKey, model, sysPrompt, userMsg)
		if err == nil && msg != "" {
			return msg, nil
		}

		// Non-retryable errors
		if err != nil {
			// API key / auth errors - don't retry
			if isAuthError(err) {
				return "", err
			}
			lastErr = err
		} else {
			// Empty message
			lastErr = fmt.Errorf("empty response from model (attempt %d/%d)", attempt, maxRetries)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown error after %d attempts", maxRetries)
	}
	return "", lastErr
}

// isAuthError returns true if the error looks like an API key problem.
func isAuthError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "401") ||
		strings.Contains(s, "403") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "invalid_api_key") ||
		strings.Contains(s, "invalid api key")
}

// generate posts the diff to an OpenAI-compatible chat completions endpoint.
func generate(baseURL, apiKey, model, sysPrompt, userMsg string) (string, error) {
	payload := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   300,
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

	choice := parsed.Choices[0]
	msg := strings.TrimSpace(choice.Message.Content)

	// Detect content filtering or truncation
	if msg == "" {
		switch choice.FinishReason {
		case "content_filter":
			return "", fmt.Errorf("content filtered by provider (finish_reason: content_filter)")
		case "length":
			return "", fmt.Errorf("response truncated by token limit (finish_reason: length)")
		}
	}

	return msg, nil
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
		b.WriteString("  v gencm       Use staged changes (git diff --cached)\n")
		b.WriteString("  v gencm -a    Use all changes vs HEAD (git diff HEAD)\n")
	case sourceAll:
		b.WriteString("📦 No changes found against HEAD.\n\n")
		b.WriteString("Edit some files first, then run v gencm again.\n")
	default:
		b.WriteString("📦 No staged changes found.\n\n")
		b.WriteString("You have unstaged changes - pick one:\n")
		b.WriteString("  v gencm -add    Stage all then generate (git add .)\n")
		b.WriteString("  v gencm -u      Use unstaged changes (git diff)\n")
		b.WriteString("  v gencm -a      Use all changes vs HEAD (git diff HEAD)\n")
	}
	return b.String()
}

func missingKeyMessage() string {
	var b strings.Builder
	b.WriteString("🔑 No API key configured for gencm.\n\n")
	b.WriteString("Add a [gcm] section to ~/.v_tools/settings.ini:\n\n")
	b.WriteString("[gcm]\n")
	b.WriteString("api_key = sk-xxx\n")
	b.WriteString("base_url = https://api.openai.com/v1\n")
	b.WriteString("model = gpt-4o-mini\n\n")
	b.WriteString("⚙ base_url and model are optional; they default to the values above.\n")
	return b.String()
}

// promptLangSetup asks the user to pick a commit message language and
// saves the choice to settings.ini so it persists across runs.
// Returns the resolved lang value ("en", "zh", or a custom string).
func promptLangSetup() string {
	cur := ini.Default().String("gcm.lang", "")
	if cur == "" {
		cur = "en"
	}
	fmt.Printf("📦 Commit message language (current: %s)\n", cur)
	fmt.Println()
	fmt.Println("  1) English")
	fmt.Println("  2) 中文")
	fmt.Println("  3) Custom - describe in your own words")
	fmt.Println()
	fmt.Print("Choice [1-3]: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	var lang string
	switch choice {
	case "2":
		lang = "zh"
	case "3":
		fmt.Println("Describe the language/style (e.g. 'Write in Japanese', 'Write in Spanish, formal tone'):")
		fmt.Print("> ")
		scanner.Scan()
		custom := strings.TrimSpace(scanner.Text())
		if custom == "" {
			lang = "en"
		} else {
			lang = custom
		}
	default:
		lang = "en"
	}

	if err := setting.Set("lang", lang, "gcm"); err != nil {
		fmt.Printf("⚠ Failed to save language setting: %v\n", err)
	} else {
		fmt.Printf("✅ Language saved: %s\n\n", lang)
	}
	return lang
}

func (g *Gcm) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>gencm - Generate Commit Message v%s</>\n\n", g.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v gencm                       Generate from staged changes (default)")
	color.Println("  v gencm <green>-add</>                  Stage all (git add .) then generate")
	color.Println("  v gencm <green>-u</> / <green>-a</>                Unstaged / all-vs-HEAD changes")
	color.Println("  v gencm <green>-C</> ~/projects/myapp        Run in a specific git directory")
	color.Println("  git diff --cached | v gencm   Generate from piped diff")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-add</>   Stage all changes (git add .) before generating")
	color.Println("  <green>-u</>     Use unstaged changes (git diff)")
	color.Println("  <green>-a</>     Use all changes vs HEAD (git diff HEAD)")
	color.Println("  <green>-C</>     <path>  Run as if git was started in <path> (default: cwd)")
	color.Println("  <green>-lang</>   Message language: en, zh, or custom text (default: en)")
	color.Println("  <green>-en</>    Generate in English (one-shot, does not save config)")
	color.Println("  <green>-zh</>    Generate in Chinese (one-shot, does not save config)")
	color.Println()
	color.Println("<gray>I/O: -pipe (auto) · -copy (skip menu) · -h</>")
	color.Println()
	color.Println("<gray>Short command: gc</>")
	color.Println("<gray>Interactive menu (non-pipe): 1) Copy  2) Commit  3) Commit+Push  4) Nothing</>")
	color.Printf("<gray>Diffs > %d chars are condensed per-file to preserve structure. Config in ~/.v_tools/settings.ini [gcm].</>\n", maxDiffChars)
	color.Println("<gray>--------------------------------------------------</>")
}
