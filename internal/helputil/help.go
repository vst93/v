// Package helputil provides utilities for consistent help formatting across plugins.
package helputil

import (
	"fmt"
	"strings"

	"github.com/gookit/color"
)

// HelpBuilder constructs formatted help output for plugins.
type HelpBuilder struct {
	name        string
	version     string
	tagline     string
	usage       []string
	modes       []ModeItem
	options     []OptionItem
	examples    []ExampleItem
	footer      string
	shortCmd    string
}

// ModeItem represents a mode/subcommand in the help output.
type ModeItem struct {
	Flag string
	Desc string
}

// OptionItem represents an option/flag in the help output.
type OptionItem struct {
	Flag string
	Desc string
}

// ExampleItem represents a usage example.
type ExampleItem struct {
	Cmd  string
	Desc string
}

// New creates a new HelpBuilder.
func New(name, version, tagline string) *HelpBuilder {
	return &HelpBuilder{
		name:    name,
		version: version,
		tagline: tagline,
	}
}

// WithShortCommand sets the short alias for the command.
func (h *HelpBuilder) WithShortCommand(short string) *HelpBuilder {
	h.shortCmd = short
	return h
}

// AddUsage adds usage patterns.
func (h *HelpBuilder) AddUsage(patterns ...string) *HelpBuilder {
	h.usage = append(h.usage, patterns...)
	return h
}

// AddMode adds a mode/subcommand.
func (h *HelpBuilder) AddMode(flag, desc string) *HelpBuilder {
	h.modes = append(h.modes, ModeItem{Flag: flag, Desc: desc})
	return h
}

// AddOption adds an option/flag.
func (h *HelpBuilder) AddOption(flag, desc string) *HelpBuilder {
	h.options = append(h.options, OptionItem{Flag: flag, Desc: desc})
	return h
}

// AddExample adds a usage example.
func (h *HelpBuilder) AddExample(cmd, desc string) *HelpBuilder {
	h.examples = append(h.examples, ExampleItem{Cmd: cmd, Desc: desc})
	return h
}

// WithFooter sets the footer text.
func (h *HelpBuilder) WithFooter(footer string) *HelpBuilder {
	h.footer = footer
	return h
}

// Build generates the formatted help output.
func (h *HelpBuilder) Build() string {
	var sb strings.Builder

	// Header
	sb.WriteString(color.Gray.Sprint("--------------------------------------------------") + "\n")
	sb.WriteString(color.Style{color.FgCyan, color.OpBold}.Sprintf("%s v%s", h.name, h.version))
	if h.shortCmd != "" {
		sb.WriteString(color.Gray.Sprintf(" (alias: %s)", h.shortCmd))
	}
	sb.WriteString("\n")
	if h.tagline != "" {
		sb.WriteString(h.tagline + "\n")
	}
	sb.WriteString("\n")

	// Usage
	if len(h.usage) > 0 {
		sb.WriteString(color.Style{color.FgMagenta, color.OpBold}.Sprint("Usage:") + "\n")
		for _, u := range h.usage {
			sb.WriteString("  " + colorizeFlags(u) + "\n")
		}
		sb.WriteString("\n")
	}

	// Modes
	if len(h.modes) > 0 {
		sb.WriteString(color.Style{color.FgMagenta, color.OpBold}.Sprint("Modes:") + "\n")
		for _, m := range h.modes {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", color.Green.Sprint(m.Flag), m.Desc))
		}
		sb.WriteString("\n")
	}

	// Options
	if len(h.options) > 0 {
		sb.WriteString(color.Style{color.FgMagenta, color.OpBold}.Sprint("Options:") + "\n")
		// Calculate max flag width for alignment
		maxWidth := 0
		for _, opt := range h.options {
			if len(opt.Flag) > maxWidth {
				maxWidth = len(opt.Flag)
			}
		}
		for _, opt := range h.options {
			padding := strings.Repeat(" ", maxWidth-len(opt.Flag))
			sb.WriteString(fmt.Sprintf("  %s%s  %s\n", color.Green.Sprint(opt.Flag), padding, opt.Desc))
		}
		sb.WriteString("\n")
	}

	// Examples
	if len(h.examples) > 0 {
		sb.WriteString(color.Style{color.FgMagenta, color.OpBold}.Sprint("Examples:") + "\n")
		for _, ex := range h.examples {
			if ex.Desc != "" {
				sb.WriteString(color.Gray.Sprintf("  # %s\n", ex.Desc))
			}
			sb.WriteString("  " + colorizeFlags(ex.Cmd) + "\n")
		}
		sb.WriteString("\n")
	}

	// Footer
	if h.footer != "" {
		sb.WriteString(color.Gray.Sprint(h.footer) + "\n")
	}

	sb.WriteString(color.Gray.Sprint("--------------------------------------------------"))

	return sb.String()
}

// colorizeFlags adds color to flags in a command string.
func colorizeFlags(s string) string {
	// Simple approach: highlight -flag patterns
	words := strings.Fields(s)
	var result []string
	for _, w := range words {
		if strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") {
			result = append(result, color.Green.Sprint(w))
		} else if strings.HasPrefix(w, "--") {
			result = append(result, color.Green.Sprint(w))
		} else {
			result = append(result, w)
		}
	}
	return strings.Join(result, " ")
}

// Print prints the help output to stdout.
func (h *HelpBuilder) Print() {
	fmt.Println(h.Build())
}
