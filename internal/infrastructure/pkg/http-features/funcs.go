package httpfeatures

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

func BuildUrl(baseUrl string, username string) string {
	return fmt.Sprintf("%s/users/%s", baseUrl, username)
}

func TimePtrToStringPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}

	// RFC3339 for JSON/API
	formatted := t.Format(time.RFC3339)
	return &formatted
}

func ParseCursor(cursor string) (parsedTime time.Time, parsedID string, err error) {
	decodedCursor, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", sharederrors.ErrInvalidCursorFormat
	}

	slice := strings.SplitN(string(decodedCursor), "|", 2)
	if len(slice) != 2 {
		return time.Time{}, "", sharederrors.ErrInvalidCursorFormat
	}

	preparedTime, err := time.Parse(time.RFC3339Nano, slice[0])
	if err != nil {
		return time.Time{}, "", sharederrors.ErrInvalidCursorTime
	}

	return preparedTime, slice[1], nil
}

func GenerateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func HashCode(rawCode, userID, purpose string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(userID + ":" + purpose + ":" + rawCode))
	return hex.EncodeToString(mac.Sum(nil))
}
