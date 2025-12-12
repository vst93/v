package plugin_json2excel

import (
	"fmt"
	"strings"
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
		"-i":        "input file path",
		"-c":        "json content, if set, -i will be ignored; pipe mode is auto detected",
		"-o":        "output file path",
		"-k":        "drill down key, use dot to separate keys, e.g. -k a.b.c",
		"-unexpand": "do not expand json data to multiple columns",
	}
	j.author = "v"
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
	fmt.Println(args)
	inputPath := ""
	outputPath := ""
	expand := true
	content := ""
	var err error
	keyDrill := []string{}

	for key, arg := range args {
		if arg == "-i" && len(args) > key+1 {
			inputPath = args[key+1]
		}
		if (arg == "-c" || arg == "-pipe") && len(args) > key+1 {
			content = args[key+1]
		}
		if arg == "-o" && len(args) > key+1 {
			outputPath = args[key+1]
		}
		if arg == "-k" && len(args) > key+1 {
			keyDrillString := args[key+1]
			if keyDrillString != "" {
				keyDrill = strings.Split(keyDrillString, ".")
			}
		}
		if arg == "-unexpand" {
			expand = false
		}
	}
	if len(args) == 1 {
		content = args[0]
	} else {
		if content == "" {
			if inputPath == "" {
				return fmt.Errorf("input file path is required")
			}
			// 1. 读取 JSON 文件
			content, err = ReadFile(inputPath)
			if err != nil {
				return err
			}
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
	// jsonString, _ := json.Marshal(result.Data[0])
	// fmt.Println("json: ", string(jsonString))
	// fmt.Println("fields: ", result.Fields)
	// return nil
	// 4. 导出为 Excel
	filepath, err := result.ExportToExcel(outputPath)
	if err != nil {
		return err
	}
	fmt.Println("export to excel: ", filepath)
	return nil
}
func (j *Json2Excel) Stop() error {
	return nil
}
