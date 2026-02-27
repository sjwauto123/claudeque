package utils

import "time"

var local = time.Local // 固定时区

func ParseStartDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	t, err := time.ParseInLocation("2006-01-02", s, local)
	if err != nil {
		return time.Time{}, err
	}

	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return start, nil
}

func ParseEndDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	t, err := time.ParseInLocation("2006-01-02", s, local)
	if err != nil {
		return time.Time{}, err
	}

	// 推荐写法：次日 00:00:00 - 1ns
	end := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location()).Add(-1)

	return end, nil
}
