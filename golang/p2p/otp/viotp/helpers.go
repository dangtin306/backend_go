package otp_viotp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Token           string
	BaseURL         string
	DefaultCountry  string
	PriceMultiplier float64
	TimeoutMinutes  int
	SpamPenalty     float64
	AddLeadingZero  bool
}

func LoadConfig() Config {
	cfg := Config{
		Token:           "",
		BaseURL:         "https://api.viotp.com",
		DefaultCountry:  "vn",
		PriceMultiplier: 1.3,
		TimeoutMinutes:  12,
		SpamPenalty:     10,
		AddLeadingZero:  false,
	}

	values := map[string]string{}
	if data, err := os.ReadFile(configPath()); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			values[key] = value
		}
	}

	cfg.Token = firstNonEmpty(getEnv("VIOTP_TOKEN"), values["viotp_token"], cfg.Token)
	cfg.BaseURL = strings.TrimRight(firstNonEmpty(getEnv("VIOTP_BASE_URL"), values["viotp_base_url"], cfg.BaseURL), "/")
	cfg.DefaultCountry = normalizeCountry(firstNonEmpty(getEnv("VIOTP_DEFAULT_COUNTRY"), values["viotp_default_country"], cfg.DefaultCountry), "vn")
	cfg.PriceMultiplier = firstNonEmptyFloat(getEnvFloat("VIOTP_PRICE_MULTIPLIER"), parseFloat(values["viotp_price_multiplier"]), cfg.PriceMultiplier)
	cfg.TimeoutMinutes = firstNonEmptyInt(getEnvInt("VIOTP_TIMEOUT_MINUTES"), parseInt(values["viotp_timeout_minutes"]), cfg.TimeoutMinutes)
	cfg.SpamPenalty = firstNonEmptyFloat(getEnvFloat("VIOTP_SPAM_PENALTY"), parseFloat(values["viotp_spam_penalty"]), cfg.SpamPenalty)
	cfg.AddLeadingZero = firstNonEmptyBool(getEnvBool("VIOTP_ADD_LEADING_ZERO"), parseBool(values["viotp_add_leading_zero"]), cfg.AddLeadingZero)

	return cfg
}

func configPath() string {
	baseDir, err := os.Getwd()
	if err != nil || strings.TrimSpace(baseDir) == "" {
		return "viotp_config.txt"
	}
	return filepath.Join(baseDir, "p2p", "otp", "viotp", "viotp_config.txt")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptyInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyFloat(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyBool(values ...bool) bool {
	for _, value := range values {
		if value {
			return true
		}
	}
	return false
}

func getEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func getEnvInt(key string) int {
	return parseInt(os.Getenv(key))
}

func getEnvFloat(key string) float64 {
	return parseFloat(os.Getenv(key))
}

func getEnvBool(key string) bool {
	return parseBool(os.Getenv(key))
}

func parseInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func parseFloat(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeCountry(raw string, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = filterLetters(value)
	if value == "" {
		value = filterLetters(strings.ToLower(strings.TrimSpace(fallback)))
	}
	return value
}

func filterLetters(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == math.Trunc(v) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprint(v)
	}
}

func toInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		return parseInt(v)
	case []byte:
		return parseInt(string(v))
	default:
		return 0
	}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		return parseFloat(v)
	case []byte:
		return parseFloat(string(v))
	default:
		return 0
	}
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return parseBool(v)
	case []byte:
		return parseBool(string(v))
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func toMap(value any) map[string]any {
	if data, ok := value.(map[string]any); ok {
		return data
	}
	return map[string]any{}
}

func normalizeNetwork(raw string) string {
	networkRaw := strings.TrimSpace(raw)
	networkLower := strings.ToLower(networkRaw)
	networkMap := map[string]string{
		"mobifone":     "MOBIFONE",
		"vinaphone":    "VINAPHONE",
		"viettel":      "VIETTEL",
		"vietnamobile": "VIETNAMOBILE",
		"itelecom":     "ITELECOM",
		"wintel":       "WINTEL",
		"unitel":       "UNITEL",
		"etl":          "ETL",
		"beeline":      "BEELINE",
		"laotel":       "LAOTEL",
	}
	if networkLower == "" || networkLower == "auto" {
		return ""
	}
	if mapped, ok := networkMap[networkLower]; ok {
		return mapped
	}
	return strings.ToUpper(networkRaw)
}

func normalizeVNPhone(raw string) string {
	phone := strings.TrimSpace(raw)
	digits := digitsOnly(phone)
	if strings.HasPrefix(phone, "+84") {
		return "+84" + strings.TrimLeft(phone[3:], "+")
	}
	if strings.HasPrefix(digits, "84") {
		return "+" + digits
	}
	return "+84" + digits
}

func digitsOnly(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatCash(value float64) string {
	if value == math.Trunc(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func httpGetRaw(endpoint string, timeout time.Duration) (string, int, string) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", 0, err.Error()
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err.Error()
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err.Error()
	}
	return string(body), resp.StatusCode, ""
}

func parseJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func parseJSONList(raw string) []map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func parseDBTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02",
	}
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		location = time.Local
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, location); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func makeServiceURL(baseURL, token, country string) string {
	params := url.Values{}
	params.Set("token", token)
	if strings.TrimSpace(country) != "" {
		params.Set("country", country)
	}
	return strings.TrimRight(baseURL, "/") + "/service/getv2?" + params.Encode()
}

func makeSessionURL(baseURL, token, requestID string) string {
	params := url.Values{}
	params.Set("requestId", requestID)
	params.Set("token", token)
	return strings.TrimRight(baseURL, "/") + "/session/getv2?" + params.Encode()
}
