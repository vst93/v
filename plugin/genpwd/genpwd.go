package plugin_genpwd

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/atotto/clipboard"
	"github.com/gookit/color"
)

const (
	lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
	uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars     = "0123456789"
	specialChars   = "!@#$%^&*()-_=+[]{}|;:,.<>?"
)

// PasswordConfig holds the password generation rules.
type PasswordConfig struct {
	Length    int
	Lowercase bool
	Uppercase bool
	Digits    bool
	Special   bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() PasswordConfig {
	return PasswordConfig{
		Length:    16,
		Lowercase: true,
		Uppercase: true,
		Digits:    true,
		Special:   true,
	}
}

// CharsetSize returns the number of possible characters based on config.
func (c PasswordConfig) CharsetSize() int {
	n := 0
	if c.Lowercase {
		n += len(lowercaseChars)
	}
	if c.Uppercase {
		n += len(uppercaseChars)
	}
	if c.Digits {
		n += len(digitChars)
	}
	if c.Special {
		n += len(specialChars)
	}
	return n
}

// Entropy returns the entropy in bits for the given config.
func (c PasswordConfig) Entropy() float64 {
	cs := c.CharsetSize()
	if cs == 0 {
		return 0
	}
	return math.Log2(float64(cs)) * float64(c.Length)
}

// GeneratePassword generates a cryptographically secure random password.
func GeneratePassword(config PasswordConfig) (string, error) {
	if config.Length < 1 {
		config.Length = 1
	}

	var charSets []string
	var allChars string

	if config.Lowercase {
		charSets = append(charSets, lowercaseChars)
		allChars += lowercaseChars
	}
	if config.Uppercase {
		charSets = append(charSets, uppercaseChars)
		allChars += uppercaseChars
	}
	if config.Digits {
		charSets = append(charSets, digitChars)
		allChars += digitChars
	}
	if config.Special {
		charSets = append(charSets, specialChars)
		allChars += specialChars
	}

	if len(allChars) == 0 {
		allChars = lowercaseChars
		charSets = []string{lowercaseChars}
	}

	password := make([]byte, config.Length)

	// Ensure at least one character from each selected set
	pos := 0
	for _, charset := range charSets {
		if pos >= config.Length {
			break
		}
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		password[pos] = charset[idx.Int64()]
		pos++
	}

	// Fill the rest with random characters from all sets
	for ; pos < config.Length; pos++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allChars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		password[pos] = allChars[idx.Int64()]
	}

	// Fisher-Yates shuffle using crypto/rand
	for i := len(password) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("failed to shuffle: %w", err)
		}
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}

	return string(password), nil
}

// --- Plugin ---

type Genpwd struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (g *Genpwd) Init() error {
	g.name = "genpwd"
	g.version = "1.0.0"
	g.description = "Random password generator with interactive TUI - configure rules and generate secure passwords"
	g.command = "genpwd"
	g.args = map[string]string{
		"-l":  "Password length (default 16)",
		"-n":  "Number of passwords to generate (non-interactive mode)",
		"-c":  "Copy generated password to clipboard (non-interactive mode)",
		"-nl": "Exclude lowercase letters (non-interactive mode)",
		"-nu": "Exclude uppercase letters (non-interactive mode)",
		"-nd": "Exclude digits (non-interactive mode)",
		"-ns": "Exclude special characters (non-interactive mode)",
		"-i":  "Force interactive TUI mode",
	}
	g.author = "vst"
	return nil
}

func (g *Genpwd) GetName() string        { return g.name }
func (g *Genpwd) GetVersion() string     { return g.version }
func (g *Genpwd) GetDescription() string { return g.description }
func (g *Genpwd) GetCommand() string     { return g.command }
func (g *Genpwd) GetArgs() map[string]string { return g.args }
func (g *Genpwd) GetAuthor() string      { return g.author }
func (g *Genpwd) Stop() error            { return nil }

func (g *Genpwd) Run(args []string) error {
	var (
		length     int  = 16
		count      int  = 1
		copyClip   bool
		noLower    bool
		noUpper    bool
		noDigit    bool
		noSpecial  bool
		forceInter bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-l":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n < 1 {
					return fmt.Errorf("invalid length: %s", args[i+1])
				}
				length = n
				i++
			}
		case "-n":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n < 1 {
					return fmt.Errorf("invalid count: %s", args[i+1])
				}
				count = n
				i++
			}
		case "-c":
			copyClip = true
		case "-nl":
			noLower = true
		case "-nu":
			noUpper = true
		case "-nd":
			noDigit = true
		case "-ns":
			noSpecial = true
		case "-i":
			forceInter = true
		case "-h", "--help":
			g.printHelp()
			return nil
		}
	}

	// Determine mode: interactive if no args or -i flag
	hasFlags := len(args) > 0
	if !hasFlags || forceInter {
		config := DefaultConfig()
		config.Length = length
		config.Lowercase = !noLower
		config.Uppercase = !noUpper
		config.Digits = !noDigit
		config.Special = !noSpecial
		ui := NewPasswordUI(config)
		return ui.Run()
	}

	// Non-interactive mode
	config := PasswordConfig{
		Length:    length,
		Lowercase: !noLower,
		Uppercase: !noUpper,
		Digits:    !noDigit,
		Special:   !noSpecial,
	}

	if config.CharsetSize() == 0 {
		return fmt.Errorf("all character sets disabled - enable at least one")
	}

	for i := 0; i < count; i++ {
		pwd, err := GeneratePassword(config)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
		if copyClip && count == 1 {
			if err := clipboard.WriteAll(pwd); err != nil {
				return fmt.Errorf("failed to copy to clipboard: %w", err)
			}
			fmt.Println(pwd)
			fmt.Println(color.Green.Sprint("✅ Copied to clipboard"))
		} else {
			fmt.Println(pwd)
		}
	}

	return nil
}

func (g *Genpwd) printHelp() {
	fmt.Printf("genpwd - Random Password Generator v%s\n\n", g.version)
	fmt.Println("Usage:")
	fmt.Println("  v gp                         Interactive TUI (default, short command)")
	fmt.Println("  v genpwd                     Same as above (full name)")
	fmt.Println("  v gp -l 20                   Generate a 20-char password to stdout")
	fmt.Println("  v gp -l 12 -n 5              Generate 5 passwords of length 12")
	fmt.Println("  v gp -l 16 -c                Generate and copy to clipboard")
	fmt.Println("  v gp -l 16 -ns               No special characters")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -l N    Password length (default 16)")
	fmt.Println("  -n N    Number of passwords (non-interactive, default 1)")
	fmt.Println("  -c      Copy to clipboard (non-interactive)")
	fmt.Println("  -nl     Exclude lowercase letters")
	fmt.Println("  -nu     Exclude uppercase letters")
	fmt.Println("  -nd     Exclude digits")
	fmt.Println("  -ns     Exclude special characters")
	fmt.Println("  -i      Force interactive TUI mode")
	fmt.Println("  -h      Show this help")
	fmt.Println()
	fmt.Println("Interactive TUI keys:")
	fmt.Println("  ← →     Adjust length (presets: 8 12 16 20 24 32 48 64)")
	fmt.Println("  Enter   Custom length input (on length row)")
	fmt.Println("  Space   Next preset / toggle checkbox")
	fmt.Println("  Tab/↑↓  Navigate between options")
	fmt.Println("  r       Regenerate password")
	fmt.Println("  y       Copy password to clipboard")
	fmt.Println("  q       Quit")
	fmt.Println()
	fmt.Println("Character sets:")
	fmt.Printf("  Lowercase: %s (%d)\n", lowercaseChars, len(lowercaseChars))
	fmt.Printf("  Uppercase: %s (%d)\n", uppercaseChars, len(uppercaseChars))
	fmt.Printf("  Digits:    %s (%d)\n", digitChars, len(digitChars))
	fmt.Printf("  Special:   %s (%d)\n", specialChars, len(specialChars))
}
