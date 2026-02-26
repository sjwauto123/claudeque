package utils

import "time"

// ParseDate 解析时间字符串（使用本地时区）
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

// GetStartOfDay 返回指定日期的 00:00:00
func GetStartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// ParseDateToStart 解析日期字符串并返回当天 00:00:00
func ParseDateToStart(s string) (time.Time, error) {
	t, err := ParseDate(s)
	if err != nil {
		return time.Time{}, err
	}
	return GetStartOfDay(t), nil
}

// GetEndOfDay 返回指定日期的 23:59:59.999999999
func GetEndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

// ParseDateToEnd 解析日期字符串并返回当天 23:59:59.999999999
func ParseDateToEnd(s string) (time.Time, error) {
	t, err := ParseDate(s)
	if err != nil {
		return time.Time{}, err
	}
	return GetEndOfDay(t), nil
}
