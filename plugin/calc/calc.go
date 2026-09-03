package plugin_calc

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gookit/color"
)

type Calc struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (c *Calc) Init() error {
	c.name = "calc"
	c.version = "1.0.0"
	c.description = "Advanced calculator with TUI interface"
	c.command = "calc"
	c.args = map[string]string{
		"-e": "evaluate expression directly (e.g., calc -e '2+2')",
		"-h": "show help",
	}
	c.author = "vst"
	return nil
}

func (c *Calc) GetName() string        { return c.name }
func (c *Calc) GetVersion() string     { return c.version }
func (c *Calc) GetDescription() string { return c.description }
func (c *Calc) GetCommand() string     { return c.command }
func (c *Calc) GetArgs() map[string]string { return c.args }
func (c *Calc) GetAuthor() string      { return c.author }
func (c *Calc) GetAliases() []string   { return []string{"calculator"} }

func (c *Calc) Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "-help", "--help":
			c.printHelp()
			return nil
		case "-e":
			if len(args) < 2 {
				return fmt.Errorf("missing expression after -e")
			}
			expr := strings.Join(args[1:], " ")
			result, err := evaluate(expr)
			if err != nil {
				return fmt.Errorf("error: %v", err)
			}
			fmt.Printf("%s = %s\n", expr, formatResult(result))
			return nil
		}
	}

	// 启动 TUI 模式（全屏）
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (c *Calc) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>calc - Advanced Calculator v%s (Alias: calculator)</>\n\n", c.version)
	color.Println("Interactive TUI calculator supporting advanced mathematical expressions.")
	color.Println()
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v calc                    Launch interactive TUI calculator")
	color.Println("  v calc <green>-e</>  '<expr>'       Evaluate expression directly")
	color.Println("  v calculator              Same as calc (alias)")
	color.Println()
	color.Println("<fg=magenta;op=bold>Supported Operations:</>")
	color.Println("  <green>Basic:</>       +, -, *, /, % (modulo), ^ (power)")
	color.Println("  <green>Functions:</> sin, cos, tan, sqrt, log, ln, abs")
	color.Println("  <green>Constants:</> pi, e")
	color.Println("  <green>Grouping:</>  ( ) for precedence")
	color.Println()
	color.Println("<fg=magenta;op=bold>Examples:</>")
	color.Println("  <gray># Launch interactive mode</>")
	color.Println("  v calc")
	color.Println()
	color.Println("  <gray># Direct evaluation</>")
	color.Println("  v calc <green>-e</> '2 + 2'")
	color.Println("  <gray># Output: 2 + 2 = 4</>")
	color.Println()
	color.Println("  <gray># Advanced expressions</>")
	color.Println("  v calc <green>-e</> 'sqrt(16) + 2^3'")
	color.Println("  <gray># Output: sqrt(16) + 2^3 = 12</>")
	color.Println()
	color.Println("  v calc <green>-e</> 'sin(pi/2)'")
	color.Println("  <gray># Output: sin(pi/2) = 1</>")
	color.Println()
	color.Println("  v calc <green>-e</> '(10 + 5) * 2 / 3'")
	color.Println("  <gray># Output: (10 + 5) * 2 / 3 = 10</>")
	color.Println()
	color.Println("<fg=magenta;op=bold>TUI Controls:</>")
	color.Println("  <green>Type</>      Enter mathematical expression")
	color.Println("  <green>Enter</>     Evaluate expression")
	color.Println("  <green>Ctrl+C</>    Exit calculator")
	color.Println("  <green>Ctrl+L</>    Clear input")
	color.Println()
	color.Println("<gray>All trigonometric functions use radians</>")
	color.Println("<gray>--------------------------------------------------</>")
}

func (c *Calc) Stop() error {
	return nil
}

// TUI Model
type model struct {
	textInput textinput.Model
	result    string
	err       error
	history   []string
	width     int
	height    int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter expression (e.g., 2+2, sqrt(16), sin(pi/2))"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	return model{
		textInput: ti,
		result:    "",
		err:       nil,
		history:   []string{},
		width:     0,
		height:    0,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 20
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlL:
			m.textInput.Reset()
			m.result = ""
			m.err = nil
			return m, nil
		case tea.KeyEnter:
			expr := strings.TrimSpace(m.textInput.Value())
			if expr != "" {
				result, err := evaluate(expr)
				if err != nil {
					m.err = err
					m.result = ""
				} else {
					m.err = nil
					m.result = formatResult(result)
					m.history = append([]string{fmt.Sprintf("%s = %s", expr, m.result)}, m.history...)
					if len(m.history) > 5 {
						m.history = m.history[:5]
					}
				}
			}
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🧮 Advanced Calculator"))
	b.WriteString("\n\n")

	// Input
	inputLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Render("Expression: ")

	b.WriteString(inputLabel)
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Result or Error
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		b.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
		b.WriteString("\n")
	} else if m.result != "" {
		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true).
			Width(40)

		resultLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render("Result: ")

		b.WriteString(resultLabel)
		b.WriteString(resultStyle.Render(m.result))
		b.WriteString("\n")
	}

	// History
	if len(m.history) > 0 {
		b.WriteString("\n")
		historyTitle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Bold(true).
			Render("History:")
		b.WriteString(historyTitle)
		b.WriteString("\n")

		historyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(2)

		for i, h := range m.history {
			if i >= 5 {
				break
			}
			b.WriteString(historyStyle.Render(fmt.Sprintf("%d. %s", i+1, h)))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	help := "Enter: evaluate | Ctrl+L: clear | Ctrl+C: quit | Supports: +,-,*,/,^,sqrt,sin,cos,tan,log,ln,pi,e"
	b.WriteString(helpStyle.Render(help))

	content := b.String()

	// 全屏居中显示
	if m.width > 0 && m.height > 0 {
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Width(m.width - 4).
			Height(m.height - 2)

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxStyle.Render(content))
	}

	// 回退方案：非全屏时使用原有样式
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2)

	return boxStyle.Render(content)
}
