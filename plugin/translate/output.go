package plugin_translate

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Preserve the translator's original terminal colors and layout.
const (
	Reset      = "\033[0m"
	Red        = "\033[31m"
	Green      = "\033[32m"
	Yellow     = "\033[33m"
	Magenta    = "\033[35m"
	Cyan       = "\033[36m"
	BoldYellow = "\033[1;33m"
	BoldGreen  = "\033[1;32m"
)

func DisplayWordInfo(info *WordInfo) {
	writeWordInfo(os.Stdout, info)
}

func writeWordInfo(out io.Writer, info *WordInfo) {
	fmt.Fprintf(out, "%s输入:%s %s%s%s\n", BoldGreen, Reset, BoldYellow, displayText(info.Word), Reset)
	writeWordDetails(out, info, true)
}

func writeWordDetails(out io.Writer, info *WordInfo, translationHeading bool) {
	if info.UkPhonetic != "" {
		fmt.Fprintf(out, "%s发音:%s", BoldGreen, Reset)
		fmt.Fprintf(out, " %s英式%s %s%s%s %s美式%s %s%s%s\n",
			Cyan, Reset, Yellow, displayText(info.UkPhonetic), Reset, Cyan, Reset, Yellow, displayText(info.UsPhonetic), Reset)
	}
	if len(info.Definitions) > 0 {
		fmt.Fprintf(out, "%s释义:%s\n", BoldGreen, Reset)
		for i, definition := range info.Definitions {
			fmt.Fprintf(out, "  %s%d.%s %s%s%s\n", Cyan, i+1, Reset, Green, displayText(definition), Reset)
		}
	} else if info.Translation == "" {
		fmt.Fprintf(out, "%s释义:%s %s未找到%s\n", BoldGreen, Reset, Red, Reset)
	}
	if info.Translation != "" {
		if translationHeading {
			fmt.Fprintf(out, "%s翻译:%s\n", BoldGreen, Reset)
		}
		fmt.Fprintf(out, "  %s%s%s\n", Magenta, displayText(info.Translation), Reset)
	}
}

func writeSeparator(out io.Writer) {
	fmt.Fprintf(out, "%s%s%s\n", Cyan, strings.Repeat("-", 60), Reset)
}
