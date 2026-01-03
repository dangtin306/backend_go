package play_sql

import (
	"fmt"
	"strconv"
	"strings"

	"hust_backend/main/database"
)

// Row keeps the PHP-style helpers (FetchArray/Fetch_array) via type alias.
type Row = database.Row

// Query proxies to database.Query for shorter import path.
func Query(query string, args ...any) Row {
	return database.Query(query, args...)
}

func FetchArrayString(row Row) map[string]string {
	data := row.Fetch_array()
	if data == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(data))
	for key, value := range data {
		out[key] = toString(value)
	}
	return out
}

func ToString(value any) string {
	if value == nil {
		return ""
	}
	return toString(value)
}

func ToInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func ToFloat(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprint(v)
	}
}
