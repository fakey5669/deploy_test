package utils

import (
	"encoding/base64"
)

// Base64Encode 문자열을 base64로 인코딩합니다.
func Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
