package utility

import (
	"encoding/base64"
	"fmt"
)

func basicAuthHeader(username, password string) string {
	str := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(str))
}
