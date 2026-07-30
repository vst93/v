package plugin_json2excel

import (
	"fmt"
	"os"
	"strings"

	"github.com/gookit/color"
)

type Json2Excel struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (j *Json2Excel) Init() error {
	j.name = "json2excel"
	j.version = "0.0.1"
	j.description = "convert json data to excel file"
	j.command = "json2excel"
	j.args = map[string]string{
		"-file":     "Input JSON file path",
		"-out":      "Output file path (.xlsx or .csv; defaults to ~/Downloads)",
		"-k":        "drill down key, use dot to separate keys, e.g. -k a.b.c",
		"-unexpand": "do not expand json data to multiple columns",
		"-pipe":     "Read JSON from pipe/stdin (auto-detected)",
		"-h":        "Show help",
	}
	j.author = "vst"
	return nil
}

func (j *Json2Excel) GetName() string {
	return j.name
}
func (j *Json2Excel) GetVersion() string {
	return j.version
}
func (j *Json2Excel) GetDescription() string {
	return j.description
}
func (j *Json2Excel) GetCommand() string {
	return j.command
}
func (j *Json2Excel) GetArgs() map[string]string {
	return j.args
}
func (j *Json2Excel) GetAuthor() string {
	return j.author
}

func (j *Json2Excel) Run(args []string) error {
	inputPath := ""
	outputPath := ""
	expand := true
	content := ""
	var err error
	keyDrill := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-file", "-i": // -i kept as a back-compat alias
			if i+1 < len(args) {
				inputPath = args[i+1]
				i++
			}
		case "-out", "-o": // -o kept as a back-compat alias
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			}
		case "-pipe", "-c": // -c kept as a back-compat alias for inline content
			if i+1 < len(args) {
				content = args[i+1]
				i++
			}
		case "-k":
			if i+1 < len(args) {
				if args[i+1] != "" {
					keyDrill = strings.Split(args[i+1], ".")
				}
				i++
			}
		case "-unexpand":
			expand = false
		case "-h", "-help", "--help":
			j.printHelp()
			return nil
		default:
			// Positional argument: inline JSON content.
			if !strings.HasPrefix(arg, "-") && content == "" {
				content = arg
			}
		}
	}

	if content == "" {
		if inputPath == "" {
			return fmt.Errorf("no input: use -file <path>, pass JSON as an argument, or pipe it in")
		}
		// 1. 读取 JSON 文件
		content, err = ReadFile(expandHome(inputPath))
		if err != nil {
			return err
		}
	}

	// 2. 创建处理器
	processor := &JSONProcessor{
		Flatten:  expand, // 展开嵌套对象
		Escape:   false,  // 不转义引号
		KeyDrill: keyDrill,
	}
	// 3. 处理 JSON
	result, err := processor.GetJSONData(content)
	if err != nil {
		return err
	}
	// 4. 导出为 Excel
	filepath, err := result.ExportToExcel(expandHome(outputPath))
	if err != nil {
		return err
	}
	fmt.Println("export to excel: ", filepath)
	return nil
}

func (j *Json2Excel) printHelp() {
	color.Println("<gray>--------------------------------------------------</>")
	color.Printf("<fg=cyan;op=bold>json2excel - JSON to Excel Converter v%s</>\n\n", j.version)
	color.Println("<fg=magenta;op=bold>Usage:</>")
	color.Println("  v json2excel <green>-file</> data.json              Convert a JSON file")
	color.Println("  v json2excel '<json>'                     Convert inline JSON")
	color.Println("  cat data.json | v json2excel              Convert piped JSON")
	color.Println("  v json2excel <green>-file</> d.json <green>-k</> data.list    Drill into a nested key")
	color.Println()
	color.Println("<fg=magenta;op=bold>Options:</>")
	color.Println("  <green>-k</> <a.b.c>     Drill down key, dot separated")
	color.Println("  <green>-unexpand</>      Keep nested JSON as strings instead of expanding to columns")
	color.Println()
	color.Println("<gray>I/O: -pipe (auto) · -file <path> · -out <path> · -h</>")
	color.Println("<gray>     Priority: pipe > -file > positional argument. Output defaults to ~/Downloads.</>")
	color.Println("<gray>--------------------------------------------------</>")
}

// expandHome expands ~ to the user's home directory.
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

func (j *Json2Excel) Stop() error {
	return nil
}
