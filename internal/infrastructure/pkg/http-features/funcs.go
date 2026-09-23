package httpfeatures

import (
	"fmt"
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
