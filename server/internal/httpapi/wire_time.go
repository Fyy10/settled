package httpapi

import "time"

func parseDate(value string) (time.Time, error) {
	return time.Parse(time.DateOnly, value)
}

func formatDate(value time.Time) string {
	return value.Format(time.DateOnly)
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
