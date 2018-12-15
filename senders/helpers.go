package kontur

import "encoding/base64"

func fromBytesToBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func useString(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

func useFloat64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
