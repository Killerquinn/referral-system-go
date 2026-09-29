package httpfeatures

import (
	"errors"
	"fmt"
	"strings"
	"time"
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
	slice := strings.SplitN(cursor, "_", 2)
	if len(slice) != 2 {
		return time.Time{}, "", errors.New("invalid cursor format")
	}

	preparedTime, err := time.Parse(time.RFC3339, slice[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid time in cursor: %w", err)
	}

	return preparedTime, slice[1], nil
}
