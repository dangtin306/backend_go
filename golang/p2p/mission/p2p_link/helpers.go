package p2p_link

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"hust_backend/main/play_sql"
)

type partnerInfo struct {
	api_url          string
	api_key          string
	servicecode      string
	api_categorymini string
	time             string
	level            string
}

func fetchPartnerInfo(id_misson_main string) partnerInfo {
	id_misson_main = strings.TrimSpace(id_misson_main)
	if id_misson_main == "" {
		return partnerInfo{}
	}
	row := play_sql.Query("SELECT `api_url`, `api_key`, `servicecode`, `api_categorymini`, `time`, `level` FROM `mission_partner_info` WHERE `id_misson_main` = '" + id_misson_main + "' ")
	data := row.Fetch_array()
	return partnerInfo{
		api_url:          toString(data["api_url"]),
		api_key:          toString(data["api_key"]),
		servicecode:      toString(data["servicecode"]),
		api_categorymini: toString(data["api_categorymini"]),
		time:             toString(data["time"]),
		level:            toString(data["level"]),
	}
}

func mergePartnerInfo(row map[string]string) {
	if row == nil {
		return
	}
	info := fetchPartnerInfo(row["id"])
	row["api_url"] = info.api_url
	row["api_key"] = info.api_key
	row["servicecode"] = info.servicecode
	row["api_categorymini"] = info.api_categorymini
	row["time"] = info.time
	row["level"] = info.level
}

func toString(value any) string {
	if value == nil {
		return ""
	}
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

func toInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func toFloat(raw string) float64 {
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

func computeTimeDelaySeconds(created string, timeLimit int) (int, bool) {
	if created == "" || timeLimit <= 0 {
		return 0, false
	}
	parsed, ok := parseTime(created)
	if !ok {
		return 0, false
	}
	diff := int(time.Since(parsed).Seconds())
	if diff < 0 {
		diff = -diff
	}
	if diff >= timeLimit {
		return 0, false
	}
	remaining := timeLimit - diff
	if remaining < 0 {
		return 0, false
	}
	return remaining, true
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		location = time.Local
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
