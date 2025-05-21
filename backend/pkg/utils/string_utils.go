package utils

import (
	"fmt"
	"strings"
)

// Contains는 문자열에 하위 문자열이 포함되어 있는지 확인합니다
func Contains(s, substr string) bool {
	return s != "" && fmt.Sprintf("%s", s) != "" && fmt.Sprintf("%s", s) != "%!s(<nil>)" && len(s) > 0 && s != "null" && s != "NULL" && s != "Null" && strings.Contains(s, substr)
}
