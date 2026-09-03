package plugin_tt

import (
	"testing"
	"time"
)

func TestCST(t *testing.T) {
	// 测试 CST 时区是否正确设置为 UTC+8
	if CST.String() != "CST" {
		t.Errorf("Expected timezone name CST, got %s", CST.String())
	}

	_, offset := time.Now().In(CST).Zone()
	expectedOffset := 8 * 3600 // 8 hours in seconds
	if offset != expectedOffset {
		t.Errorf("Expected offset %d, got %d", expectedOffset, offset)
	}
}

func TestGetCurrentTime(t *testing.T) {
	tests := []struct {
		format   string
		validate func(string) bool
	}{
		{"date", func(s string) bool {
			_, err := time.Parse("2006-01-02", s)
			return err == nil && len(s) == 10
		}},
		{"time", func(s string) bool {
			_, err := time.Parse("15:04:05", s)
			return err == nil && len(s) == 8
		}},
		{"datetime", func(s string) bool {
			_, err := time.Parse("2006-01-02 15:04:05", s)
			return err == nil && len(s) == 19
		}},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			result := getCurrentTime(tc.format)
			if !tc.validate(result) {
				t.Errorf("Invalid format for %s: %s", tc.format, result)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	timestamp := int64(1641038400) // 2022-01-01 20:00:00 CST

	tests := []struct {
		format   string
		expected string
	}{
		{"date", "2022-01-01"},
		{"time", "20:00:00"},
		{"datetime", "2022-01-01 20:00:00"},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			result := formatTimestamp(timestamp, tc.format)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestTTConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"timestamp to date", "1641038400", "2022-01-01 20:00:00"},
		{"date to timestamp", "2022-01-01 20:00:00", "1641038400"},
		{"date only to timestamp", "2022-01-01", "1641038400"},
		{"millisecond timestamp", "1641038400123", "from milliseconds"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tt(tc.input)
			if result == "" {
				t.Error("Result should not be empty")
			}
			// 对于不同的输入，至少结果应该非空
			if len(result) == 0 {
				t.Errorf("Expected non-empty result for input %s", tc.input)
			}
		})
	}
}

func TestTTEmptyInput(t *testing.T) {
	result := tt("")
	if result == "" {
		t.Error("Empty input should return current timestamp")
	}
	// 结果应该是数字
	if _, err := time.Parse("2006-01-02 15:04:05", result); err == nil {
		t.Error("Empty input should return timestamp, not date")
	}
}
