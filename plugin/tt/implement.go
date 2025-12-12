package plugin_tt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

		// 解析时间
		t, err := time.Parse(layout, timeStr)
		if err != nil {
			// 尝试另一种可能的格式
			t, err = time.Parse("2006-1-2 15:04:05", timeStr)
			if err != nil {
				return fmt.Sprintf("Error parsing date: %v", err)
			}
		}

		// 返回时间戳
		return fmt.Sprintf("%d", t.Unix())
	}

	// 否则尝试作为时间戳解析, 兼容毫秒时间戳
	if len(input) > 10 {
		input = input[:10]
	}
	timestamp, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return "Error parsing timestamp"
	}

	// 转换时间戳为日期格式
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02 15:04:05")
}
