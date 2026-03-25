package otp_process

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var nonLettersRegex = regexp.MustCompile(`[^a-z]`)
var digitsRegex = regexp.MustCompile(`\D`)

type viotpConfig struct {
	Token           string
	BaseURL         string
	DefaultCountry  string
	PriceMultiplier float64
	TimeoutMinutes  int
	SpamPenalty     float64
	AddLeadingZero  bool
}

func loadVIOTPConfig() viotpConfig {
	return viotpConfig{
		Token:           getEnv("VIOTP_TOKEN", "8c7f75c450ea4fa2aa967ed973390e07"),
		BaseURL:         getEnv("VIOTP_BASE_URL", "https://api.viotp.com"),
		DefaultCountry:  getEnv("VIOTP_DEFAULT_COUNTRY", "vn"),
		PriceMultiplier: getEnvFloat("VIOTP_PRICE_MULTIPLIER", 1.3),
		TimeoutMinutes:  getEnvInt("VIOTP_TIMEOUT_MINUTES", 12),
		SpamPenalty:     getEnvFloat("VIOTP_SPAM_PENALTY", 10),
		AddLeadingZero:  getEnvBool("VIOTP_ADD_LEADING_ZERO", false),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseBodyObject(c *gin.Context) map[string]any {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return map[string]any{}
	}
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		return map[string]any{}
	}
	c.Request.Body = ioNopCloser(body)

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func ioNopCloser(raw []byte) *readCloser {
	return &readCloser{reader: bytes.NewReader(raw)}
}

type readCloser struct {
	reader *bytes.Reader
}

func (r *readCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *readCloser) Close() error {
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func str(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return toString(values[key])
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
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		value := float64(v)
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(v)
	}
}

func toFloat(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return number
}

func toIntAny(value any) int {
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
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		return text == "1" || text == "true" || text == "yes" || text == "on"
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
	if value == nil {
		return map[string]any{}
	}
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func randomCode(length int) string {
	if length <= 0 {
		return ""
	}
	const chars = "QWERTYUIOPASDFGHJKLZXCVBNM1234567890"
	max := big.NewInt(int64(len(chars)))
	var builder strings.Builder
	builder.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return builder.String()
		}
		builder.WriteByte(chars[n.Int64()])
	}
	return builder.String()
}

func formatCash(value float64) string {
	if value == math.Trunc(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func normalizeCountry(raw string, fallback string) string {
	country := strings.ToLower(strings.TrimSpace(raw))
	country = nonLettersRegex.ReplaceAllString(country, "")
	if country == "" {
		country = strings.ToLower(strings.TrimSpace(fallback))
		country = nonLettersRegex.ReplaceAllString(country, "")
	}
	return country
}

func normalizeNetwork(raw string) string {
	networkRaw := strings.TrimSpace(raw)
	networkLower := strings.ToLower(networkRaw)

	mapping := map[string]string{
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
	if mapped, ok := mapping[networkLower]; ok {
		return mapped
	}
	return strings.ToUpper(networkRaw)
}

func normalizeVNPhone(raw string) string {
	phone := strings.TrimSpace(raw)
	digits := digitsRegex.ReplaceAllString(phone, "")
	if strings.HasPrefix(phone, "+84") {
		return "+84" + strings.TrimLeft(phone[3:], "+")
	}
	if strings.HasPrefix(digits, "84") {
		return "+" + digits
	}
	return "+84" + digits
}

func buildVIOTPRequestURL(baseURL string, params url.Values) string {
	baseURL = strings.TrimSpace(baseURL)
	if strings.HasSuffix(baseURL, "/") {
		baseURL = strings.TrimSuffix(baseURL, "/")
	}
	return baseURL + "/request/getv2?" + params.Encode()
}
