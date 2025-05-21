// Package utils는 여러 패키지에서 공통으로 사용하는 유틸리티 함수를 제공합니다.
package utils

// TruncateString은 긴 문자열을 잘라서 로그나 UI에 표시할 때 사용하는 유틸리티 함수입니다.
func TruncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... (생략됨)"
}
