package plugin_json2excel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// JSONProcessor 处理 JSON 数据
type JSONProcessor struct {
	Flatten  bool
	Escape   bool
	KeyDrill []string
}

// ProcessResult 处理结果
type ProcessResult struct {
	Data   []map[string]interface{}
	Fields []string
}

// GetJSONData 解析和处理 JSON 字符串
func (jp *JSONProcessor) GetJSONData(jsonStr string) (*ProcessResult, error) {
	var data interface{}

	// 解析 JSON
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	for _, key := range jp.KeyDrill {
		if m, ok := data.(map[string]interface{}); ok {
			if value, exists := m[key]; exists {
				data = value
			}
		}
	}

	// 转换为数组格式
	var dataArray []map[string]interface{}
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				dataArray = append(dataArray, m)
			}
		}
	case map[string]interface{}:
		dataArray = []map[string]interface{}{v}
	default:
		return nil, fmt.Errorf("unsupported JSON format")
	}

	// 处理数据
	fieldsMap := make(map[string]bool)
	processedData := make([]map[string]interface{}, len(dataArray))

	for i, item := range dataArray {
		processedItem := make(map[string]interface{})

		for key, value := range item {
			result := jp.dataLoop(key, value)

			// 检查返回的是否是展开的对象
			if nestedMap, ok := result.(map[string]interface{}); ok {
				// 展开的嵌套对象
				for nestedKey, nestedValue := range nestedMap {
					fieldsMap[nestedKey] = true
					processedItem[nestedKey] = nestedValue
				}
			} else {
				// 普通值
				fieldsMap[key] = true
				processedItem[key] = result
			}
		}
		processedData[i] = processedItem
	}

	// 提取字段列表
	fields := make([]string, 0, len(fieldsMap))
	for field := range fieldsMap {
		fields = append(fields, field)
	}

	// // 确保所有数据都有所有字段
	for i := range processedData {
		for _, field := range fields {
			if _, exists := processedData[i][field]; !exists {
				processedData[i][field] = nil
			}
		}
	}

	return &ProcessResult{
		Data:   processedData,
		Fields: fields,
	}, nil
}

// dataLoop 递归处理数据
func (jp *JSONProcessor) dataLoop(prefix string, data interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		if jp.Flatten {
			result := make(map[string]interface{})
			pre := prefix
			if pre != "" {
				pre = pre + "_"
			}
			for key, value := range v {
				nestedResult := jp.dataLoop(pre+key, value)
				if nestedMap, ok := nestedResult.(map[string]interface{}); ok {
					// 合并嵌套的 map
					for k, val := range nestedMap {
						result[k] = val
					}
				} else {
					result[pre+key] = nestedResult
				}
			}
			return result
		} else {
			// 转换为 JSON 字符串
			jsonBytes, _ := json.Marshal(v)
			str := string(jsonBytes)
			if jp.Escape {
				str = strings.ReplaceAll(str, "\"", "'")
			}
			return str
		}

	case []interface{}:
		if jp.Flatten {
			// 展开数组（如果数组元素是对象）
			if len(v) > 0 {
				if _, isMap := v[0].(map[string]interface{}); isMap {
					result := make(map[string]interface{})
					for i, item := range v {
						nestedResult := jp.dataLoop(fmt.Sprintf("%s_%d", prefix, i), item)
						if nestedMap, ok := nestedResult.(map[string]interface{}); ok {
							for k, val := range nestedMap {
								result[k] = val
							}
						}
					}
					return result
				}
			}
		}
		// 转换为 JSON 字符串
		jsonBytes, _ := json.Marshal(v)
		str := string(jsonBytes)
		if jp.Escape {
			str = strings.ReplaceAll(str, "\"", "'")
		}
		return str

	default:
		// 基础类型直接返回
		return v
	}
}

// ExportToExcel 导出为 Excel 文件
func (pr *ProcessResult) ExportToExcel(downloadPath string) (string, error) {
	f := excelize.NewFile()
	defer f.Close()

	// 设置工作表名
	sheetName := "Sheet1"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)

	// 写入表头
	for col, field := range pr.Fields {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, field)
	}

	// 写入数据
	for row, data := range pr.Data {
		for col, field := range pr.Fields {
			cell, _ := excelize.CoordinatesToCellName(col+1, row+2)
			value := data[field]
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// 自动调整列宽
	for col := range pr.Fields {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		f.SetColWidth(sheetName, colName, colName, 20)
	}

	// 保存文件
	filename := fmt.Sprintf("export_%d.xlsx", time.Now().UnixNano())

	if downloadPath == "" {
		downloadPath = filepath.Join(os.Getenv("HOME"), "Downloads", filename)
	}
	// fmt.Printf("保存到: %s\n", downloadPath)
	if err := f.SaveAs(downloadPath); err != nil {
		return "", err
	}

	return downloadPath, nil
}

// ExportToCSV 导出为 CSV 文件
func (pr *ProcessResult) ExportToCSV() (string, error) {
	filename := fmt.Sprintf("export_%d.csv", time.Now().UnixNano())
	downloadPath := filepath.Join(os.Getenv("HOME"), "Downloads", filename)

	file, err := os.Create(downloadPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 使用 UTF-8 BOM 处理中文
	writer := transform.NewWriter(file, unicode.UTF8BOM.NewEncoder())
	defer writer.Close()

	// 写入表头
	headers := make([]string, len(pr.Fields))
	for i, field := range pr.Fields {
		// CSV 转义处理
		if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
			field = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
		}
		headers[i] = field
	}
	fmt.Fprintln(writer, strings.Join(headers, ","))

	// 写入数据
	for _, data := range pr.Data {
		row := make([]string, len(pr.Fields))
		for i, field := range pr.Fields {
			value := data[field]
			var strValue string

			if value == nil {
				strValue = ""
			} else {
				switch v := value.(type) {
				case string:
					strValue = v
				case float64:
					strValue = strconv.FormatFloat(v, 'f', -1, 64)
				case bool:
					strValue = strconv.FormatBool(v)
				case int, int64, int32:
					strValue = fmt.Sprintf("%v", v)
				default:
					strValue = fmt.Sprintf("%v", v)
				}

				// CSV 转义处理
				if strings.Contains(strValue, ",") || strings.Contains(strValue, "\"") || strings.Contains(strValue, "\n") {
					strValue = `"` + strings.ReplaceAll(strValue, `"`, `""`) + `"`
				}
			}
			row[i] = strValue
		}
		fmt.Fprintln(writer, strings.Join(row, ","))
	}

	return downloadPath, nil
}

// ReadFile 读取文件内容
func ReadFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ExportJSON 导出 JSON 数据
func ExportJSON(data []map[string]interface{}, ext string, onComplete func()) (string, error) {
	pr := &ProcessResult{
		Data: data,
	}

	// 从数据中提取字段
	fieldsMap := make(map[string]bool)
	for _, item := range data {
		for key := range item {
			fieldsMap[key] = true
		}
	}
	pr.Fields = make([]string, 0, len(fieldsMap))
	for field := range fieldsMap {
		pr.Fields = append(pr.Fields, field)
	}

	var filepath string
	var err error

	if strings.ToLower(ext) == "csv" {
		filepath, err = pr.ExportToCSV()
	} else {
		filepath, err = pr.ExportToExcel("")
	}

	if onComplete != nil {
		onComplete()
	}

	return filepath, err
}

// func main() {
// 	// 示例用法
// 	jsonStr := `[
// 		{"name": "张三", "age": 25, "address": {"city": "北京", "street": "长安街"}},
// 		{"name": "李四", "age": 30, "address": {"city": "上海", "street": "南京路"}}
// 	]`

// 	// 创建处理器
// 	processor := &JSONProcessor{
// 		Flatten: true,  // 是否展开嵌套对象
// 		Escape:  false, // 是否转义引号
// 	}

// 	// 处理 JSON 数据
// 	result, err := processor.GetJSONData(jsonStr)
// 	if err != nil {
// 		fmt.Printf("处理失败: %v\n", err)
// 		return
// 	}

// 	fmt.Printf("字段列表: %v\n", result.Fields)
// 	fmt.Printf("处理后的数据: %v\n", result.Data)

// 	// 导出为 Excel
// 	filepath, err := result.ExportToExcel()
// 	if err != nil {
// 		fmt.Printf("导出失败: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("已导出到: %s\n", filepath)

// 	// 或者导出为 CSV
// 	// filepath, err := result.ExportToCSV()
// 	// if err != nil {
// 	//     fmt.Printf("导出失败: %v\n", err)
// 	//     return
// 	// }
// 	// fmt.Printf("已导出到: %s\n", filepath)
// }
