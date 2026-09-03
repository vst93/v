package plugin_tt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CST 表示东八区时区
var CST = time.FixedZone("CST", 8*3600)

func tt(input string) string {
	// 空输入，返回当前时间戳
	if input == "" {
		return fmt.Sprintf("%d", time.Now().Unix())
	}

	// 尝试判断是否是日期格式（包含"-"）
	if strings.Contains(input, "-") {
		var layout string
		var timeStr string

		// 判断是否包含空格（有时间部分）
		if strings.Contains(input, " ") {
			layout = "2006-01-02 15:04:05"
			timeStr = input
		} else {
			layout = "2006-01-02"
			timeStr = input + " 00:00:00"
			layout = "2006-01-02 15:04:05"
		}

		// 解析时间（东八区）
		t, err := time.ParseInLocation(layout, timeStr, CST)
		if err != nil {
			// 尝试另一种可能的格式
			t, err = time.ParseInLocation("2006-1-2 15:04:05", timeStr, CST)
			if err != nil {
				return fmt.Sprintf("Error parsing date: %v", err)
			}
		}

		// 返回时间戳
		return fmt.Sprintf("%d", t.Unix())
	}

	// 否则尝试作为时间戳解析, 兼容毫秒时间戳
	origInput := input
	if len(input) > 10 {
		input = input[:10]
	}
	timestamp, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return "Error parsing timestamp"
	}

	// 转换时间戳为日期格式（东八区）
	t := time.Unix(timestamp, 0).In(CST)

	// 如果原始输入是毫秒级时间戳，显示提示
	if len(origInput) > 10 {
		return fmt.Sprintf("%s (from milliseconds)", t.Format("2006-01-02 15:04:05"))
	}

	return t.Format("2006-01-02 15:04:05")
}

// formatTimestamp 将时间戳转换为指定格式的日期时间字符串
func formatTimestamp(timestamp int64, format string) string {
	t := time.Unix(timestamp, 0).In(CST)

	switch format {
	case "date":
		return t.Format("2006-01-02")
	case "time":
		return t.Format("15:04:05")
	case "datetime":
		return t.Format("2006-01-02 15:04:05")
	default:
		return t.Format("2006-01-02 15:04:05")
	}
}

// getCurrentTime 获取当前时间的指定格式
func getCurrentTime(format string) string {
	t := time.Now().In(CST)

	switch format {
	case "date":
		return t.Format("2006-01-02")
	case "time":
		return t.Format("15:04:05")
	case "datetime":
		return t.Format("2006-01-02 15:04:05")
	default:
		return t.Format("2006-01-02 15:04:05")
	}
}
