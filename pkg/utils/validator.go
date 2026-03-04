package utils

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ValidatePassword 验证密码格式（6-18位，不包含空格）
func ValidatePassword(pwd string) error {
	if strings.Contains(pwd, " ") {
		return errors.New("密码不能包含空格")
	}
	length := utf8.RuneCountInString(pwd)
	if length < 6 || length > 18 {
		return errors.New("密码长度必须在6-18位之间")
	}
	return nil
}
