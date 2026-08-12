package plugin_awake

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gookit/color"
	"github.com/rivo/tview"
)

// Awake prevents the operating system from entering idle or system sleep.
// Platform-specific implementations live in awake_<os>.go.
type Awake struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
	stop        func() error
}

func (a *Awake) Init() error {
	a.name = "awake"
	a.version = "0.0.1"
	a.description = "prevent the system from sleeping with a calm full-screen live status display"
	a.command = "awake"
	a.args = map[string]string{
		"-d <duration>": "Keep awake for a duration, for example 30m or 2h",
		"-h":            "Show help",
	}
	a.author = "vst"
	a.stop = nil
	return nil
}

func (a *Awake) GetName() string            { return a.name }
func (a *Awake) GetVersion() string         { return a.version }
func (a *Awake) GetDescription() string     { return a.description }
func (a *Awake) GetCommand() string         { return a.command }
func (a *Awake) GetArgs() map[string]string { return a.args }
func (a *Awake) GetAuthor() string          { return a.author }
func (a *Awake) GetAliases() []string       { return nil }
func (a *Awake) Stop() error {
	if a.stop == nil {
		return nil
	}
	stop := a.stop
	a.stop = nil
	return stop()
}

func (a *Awake) Run(args []string) error {
	duration, help, err := parseArgs(args)
	if err != nil {
		return err
	}
	if help {
		a.printHelp()
		return nil
	}

	stop, err := startPreventSleep()
	if err != nil {
		return err
	}
	a.stop = stop
	defer a.Stop()
	return a.runUI(duration)
}

func parseArgs(args []string) (time.Duration, bool, error) {
	var duration time.Duration
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "-help", "--help":
			return 0, true, nil
		case "-d", "--duration":
			if i+1 >= len(args) {
				return 0, false, fmt.Errorf("-d requires a duration such as 30m or 2h")
			}
			parsed, err := time.ParseDuration(args[i+1])
			if err != nil || parsed <= 0 {
				return 0, false, fmt.Errorf("invalid duration %q: use a positive value such as 30m or 2h", args[i+1])
			}
			duration = parsed
			i++
		default:
			if !strings.HasPrefix(args[i], "-") {
				return 0, false, fmt.Errorf("unexpected argument %q", args[i])
			}
			return 0, false, fmt.Errorf("unknown option %q; run `v awake -h` for help", args[i])
		}
	}
	return duration, false, nil
}

func (a *Awake) runUI(duration time.Duration) error {
	app := tview.NewApplication()
	done := make(chan struct{})

	animation := newPulseView(runtime.GOOS, duration)

	tips := []string{
		"Tip: keep this window open while a long task is running.",
		"Tip: press q, Esc, or Ctrl-C to release the sleep lock.",
		"Tip: use -d 30m for an automatic stop.",
	}
	footer := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	footer.SetBackgroundColor(tcell.ColorDefault)
	footer.SetText("[gray]Press q / Esc / Ctrl-C to exit[white]\n" + tips[0])

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.SetBackgroundColor(tcell.ColorDefault)
	root.AddItem(tview.NewBox().SetBackgroundColor(tcell.ColorDefault), 1, 0, false).
		AddItem(animation, 0, 1, false).
		AddItem(tview.NewBox().SetBackgroundColor(tcell.ColorDefault), 1, 0, false).
		AddItem(footer, 3, 0, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyCtrlC || event.Rune() == 'q' || event.Rune() == 'Q' {
			app.Stop()
			return nil
		}
		return event
	})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case <-signals:
			app.Stop()
		case <-done:
		}
	}()
	var timer *time.Timer
	if duration > 0 {
		timer = time.AfterFunc(duration, app.Stop)
		defer timer.Stop()
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		tip := 0
		lastTip := time.Now()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}
			if time.Since(lastTip) >= 4*time.Second {
				tip = (tip + 1) % len(tips)
				lastTip = time.Now()
			}
			app.QueueUpdateDraw(func() {
				footer.SetText("[gray]Press q / Esc / Ctrl-C to exit[white]\n" + tips[tip])
			})
		}
	}()

	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		close(done)
		return fmt.Errorf("awake UI error: %w", err)
	}
	close(done)
	return nil
}

func (a *Awake) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>☕ awake - Prevent System Sleep v%s</>\n\n", a.version)
	color.Println("Keep the computer awake with a calm full-screen live status display.")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v awake                 Prevent sleep until Ctrl-C")
	color.Println("  v awake -d 30m          Prevent sleep for 30 minutes")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-d, --duration</>  Duration (for example 30s, 30m, 2h)")
	color.Println("  <green>-h</>              Show this help")
	color.Println()
	color.Println("<gray>macOS: caffeinate · Linux: systemd-inhibit · Windows: SetThreadExecutionState</>")
	color.Println("<gray>--------------------------------------------------</>")
}
