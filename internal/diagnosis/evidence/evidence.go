package evidence

import "fmt"

// Line formats a single evidence string "key=value" or "key: detail".
func Line(key, detail string) string {
	if detail == "" {
		return key
	}
	return key + ": " + detail
}

// KV formats key=value evidence.
func KV(key string, value any) string {
	return fmt.Sprintf("%s=%v", key, value)
}
