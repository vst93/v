package plugin_jv

import "github.com/atotto/clipboard"

// clipboardWrite writes text to the system clipboard.
func clipboardWrite(text string) error {
	return clipboard.WriteAll(text)
}
