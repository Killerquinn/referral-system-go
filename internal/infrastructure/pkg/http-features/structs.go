package httpfeatures

import (
	"fmt"
)

type Header struct {
	Key   string
	Value string
}

func BuildUrl(baseUrl string, username string) string {
	return fmt.Sprintf("%s/users/%s", baseUrl, username)
}
