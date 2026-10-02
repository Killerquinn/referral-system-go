package refreshtoken

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func UnhashToken(b []byte) (string, error) {
	if b == nil {
		return "", fmt.Errorf("%w:%s", fmt.Errorf("refresh token is missing"), "retry your request please")
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
