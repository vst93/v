package plugin_tt

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/color"
)

type TT struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (t *TT) Init() error {
	t.name = "tt"
	t.version = "1.0.0"
	t.description = "Timestamp converter and live clock display"
	t.command = "tt"
	t.args = map[string]string{
		"-m":        "return millisecond timestamp",
		"-date":     "return current date (2026-01-01)",
		"-time":     "return current time (23:00:01)",
		"-datetime": "return current date and time (2026-01-01 23:00:01)",
		"-live":     "live clock display (real-time updates)",
		"-h":        "Show help",
	}
	t.author = "vst"
	return nil
}

func (t *TT) GetName() string {
	return t.name
}
func (t *TT) GetVersion() string {
	return t.version
}
func (t *TT) GetDescription() string {
	return t.description
}
func (t *TT) GetCommand() string {
	return t.command
}
func (t *TT) GetArgs() map[string]string {
	return t.args
}
func (t *TT) GetAuthor() string {
	return t.author
}

func (t *TT) GetAliases() []string { return nil }

func (t *TT) Run(args []string) error {
	if len(args) == 0 {
		// print now timestamp
		fmt.Println(time.Now().Unix())
		return nil
	}

	input := args[0]
	switch input {
	case "-m":
		// print now timestamp in millisecond
		fmt.Println(time.Now().UnixNano() / 1e6)
		return nil
	case "-date":
		// return current date only
		fmt.Println(getCurrentTime("date"))
		return nil
	case "-time":
		// return current time only
		fmt.Println(getCurrentTime("time"))
		return nil
	case "-datetime":
		// return current datetime
		fmt.Println(getCurrentTime("datetime"))
		return nil
	case "-live":
		// live clock display
		return t.liveDisplay()
	case "-h", "-help", "--help":
		t.printHelp()
		return nil
	}

	// 如果第一个参数是时间戳，检查是否有第二个参数指定格式
	if len(args) >= 2 {
		timestamp, err := strconv.ParseInt(args[0], 10, 64)
		if err == nil {
			// 第一个参数是时间戳，第二个参数是格式
			format := strings.TrimPrefix(args[1], "-")
			fmt.Println(formatTimestamp(timestamp, format))
			return nil
		}
	}

	fmt.Println(tt(input))
	return nil
}

func (t *TT) liveDisplay() error {
	// 隐藏光标
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h") // 恢复光标

	fmt.Println("Live Clock Display (CST/UTC+8) - Press Ctrl+C to exit")
	fmt.Println()

	// 用于清除行的 ANSI 转义序列
	clearLine := "\033[2K\r"

	for {
		now := time.Now().In(CST)

		// 移动到开始位置并清除
		fmt.Print(clearLine)

		// 显示日期和时间
		fmt.Printf("\r📅 Date: %s  |  🕐 Time: %s  |  📊 Timestamp: %d",
			now.Format("2006-01-02"),
			now.Format("15:04:05"),
			now.Unix())

		// 等待一秒
		time.Sleep(time.Second)
	}
}

func (t *TT) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>tt - Timestamp Converter v%s</>\n\n", t.version)
	color.Println("Convert between timestamps and dates, with live clock display (CST/UTC+8).")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v tt                         Current Unix timestamp (seconds)")
	color.Println("  v tt <green>-m</>                      Current Unix timestamp (milliseconds)")
	color.Println("  v tt <green>-date</>                   Current date (2026-01-01)")
	color.Println("  v tt <green>-time</>                   Current time (23:00:01)")
	color.Println("  v tt <green>-datetime</>               Current datetime (2026-01-01 23:00:01)")
	color.Println("  v tt <green>-live</>                   Live clock display (real-time)")
	color.Println("  v tt 1641038400              Timestamp → date (CST)")
	color.Println("  v tt 1641038400 <green>-date</>        Timestamp → date only")
	color.Println("  v tt 1641038400 <green>-time</>        Timestamp → time only")
	color.Println("  v tt '2022-01-01 12:00:00'   Date → timestamp")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-m</>                Return millisecond timestamp")
	color.Println("  <green>-date</>             Return/convert to date only (YYYY-MM-DD)")
	color.Println("  <green>-time</>             Return/convert to time only (HH:MM:SS)")
	color.Println("  <green>-datetime</>         Return/convert to datetime (YYYY-MM-DD HH:MM:SS)")
	color.Println("  <green>-live</>             Display live updating clock")
	color.Println("  <green>-h</>                Show this help")
	color.Println()
	color.Println("<fg=magenta;op=bold>Examples:</>")
	color.Println("  <gray># Get current timestamp</>")
	color.Println("  v tt")
	color.Println("  <gray># Output: 1735977601</>")
	color.Println()
	color.Println("  <gray># Get current date and time formatted</>")
	color.Println("  v tt <green>-datetime</>")
	color.Println("  <gray># Output: 2026-09-03 14:30:15</>")
	color.Println()
	color.Println("  <gray># Convert timestamp to date and time</>")
	color.Println("  v tt 1641038400")
	color.Println("  <gray># Output: 2022-01-02 00:00:00</>")
	color.Println()
	color.Println("  <gray># Convert timestamp to date only</>")
	color.Println("  v tt 1641038400 <green>-date</>")
	color.Println("  <gray># Output: 2022-01-02</>")
	color.Println()
	color.Println("  <gray># Convert date to timestamp</>")
	color.Println("  v tt '2026-09-03 14:30:00'")
	color.Println("  <gray># Output: 1788591000</>")
	color.Println()
	color.Println("  <gray># Live clock display</>")
	color.Println("  v tt <green>-live</>")
	color.Println("  <gray># Shows: 📅 Date: 2026-09-03  |  🕐 Time: 14:30:15  |  📊 Timestamp: 1735977615</>")
	color.Println()
	color.Println("<gray>All times are in CST/UTC+8 timezone by default</>")
	color.Println("<gray>Input detection: text with '-' is parsed as date, numbers as timestamp</>")
	color.Println("<gray>--------------------------------------------------</>")
}

func (t *TT) Stop() error {
	return nil
}
