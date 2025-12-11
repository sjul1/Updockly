package httpapi

import "time"

// mustTime parses RFC3339 and panics in tests on error.
func mustTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}
