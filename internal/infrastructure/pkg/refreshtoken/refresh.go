package refreshtoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GenerateRefreshToken(secret []byte) (string, string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", "", err
	}

	raw := hex.EncodeToString(b)
	hash := HashToken(raw, secret)

	return raw, hash, nil
}

func HashToken(raw string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}
