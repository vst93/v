package service

import (
	"strings"
	"testing"
)

func TestHelpShowsAliases(t *testing.T) {
	help := Help()
	for _, want := range []string{"aliases: gp", "aliases: gc", "aliases: cc", "aliases: j2e", "aliases: s2f"} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q", want)
		}
	}
}
