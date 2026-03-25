package otp_process

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

var ordersLogMu sync.Mutex

func OrdersHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	payload := parseBodyObject(c)
	serviceID := firstNonEmpty(str(payload, "service"), c.Query("service"))
	apikey := firstNonEmpty(str(payload, "apikey"), c.Query("apikey"))
	chedo := strings.ToLower(firstNonEmpty(str(payload, "chedo"), c.Query("chedo")))
	network := firstNonEmpty(
		str(payload, "network"),
		str(payload, "social"),
		c.Query("network"),
		c.Query("social"),
	)
	prefix := firstNonEmpty(str(payload, "prefix"), c.Query("prefix"))
	exceptPrefix := firstNonEmpty(
		str(payload, "exceptPrefix"),
		str(payload, "except_prefix"),
		c.Query("exceptPrefix"),
		c.Query("except_prefix"),
	)
	requestNumberRaw := firstNonEmpty(
		str(payload, "url"),
		c.Query("url"),
		str(payload, "number"),
		c.Query("number"),
	)
	countryRaw := firstNonEmpty(str(payload, "country"), c.Query("country"))
	quantityRaw := firstNonEmpty(
		str(payload, "quantity"),
		c.Query("quantity"),
		str(payload, "amount"),
		c.Query("amount"),
	)

	cfg := loadVIOTPConfig()
	appendOrdersNote(fmt.Sprintf("INCOMING chedo=%s service=%s country=%s raw_number=%s quantity=%s", chedo, serviceID, countryRaw, requestNumberRaw, quantityRaw))

	conn, err := database.Open()
	if err != nil {
		appendOrdersNote("REJECT db_open_failed")
		api.Print_json(c, "status", "0", "message", "Loi may chu")
		return
	}

	quantity := toFloat(quantityRaw)
	if quantity <= 0 {
		quantity = 1
	}
	code := randomCode(6)

	idUsers := querySingleString(conn, "SELECT `id_users` FROM `users_key` WHERE `users_apikey` = ? LIMIT 1", apikey)
	moneyService := toFloat(querySingleString(conn, "SELECT `money` FROM `service3` WHERE `id` = ? LIMIT 1", serviceID))
	nameService := querySingleString(conn, "SELECT `name` FROM `service3` WHERE `id` = ? LIMIT 1", serviceID)
	myUsername := querySingleString(conn, "SELECT `username` FROM `users` WHERE `id` = ? LIMIT 1", idUsers)
	myVND := toFloat(querySingleString(conn, "SELECT `money` FROM `users` WHERE `id` = ? LIMIT 1", idUsers))

	totalMoneyService1 := moneyService * quantity
	totalMoneyService := totalMoneyService1 - (totalMoneyService1 / 100)

	if strings.TrimSpace(idUsers) == "" || strings.TrimSpace(apikey) == "" {
		appendOrdersNote("REJECT missing_login_or_apikey")
		api.Print_json(c, "status", "0", "message", "VUI LONG DANG NHAP!")
		return
	}
	if strings.TrimSpace(serviceID) == "" {
		appendOrdersNote("REJECT missing_service")
		api.Print_json(c, "status", "0", "message", "Vui long chon dich vu")
		return
	}
	if myVND < totalMoneyService {
		appendOrdersNote("REJECT insufficient_balance")
		ifVndBuy := totalMoneyService - myVND
		api.Print_json(c, "status", "0", "message", "Vui long nap them: "+formatCash(ifVndBuy)+"d de thanh toan!")
		return
	}
	if myVND < 50 {
		appendOrdersNote("REJECT balance_lt_50")
		api.Print_json(c, "status", "0", "message", "Toi thieu 50 coins de su dung dich vu nay")
		return
	}
	if strings.TrimSpace(cfg.Token) == "" {
		appendOrdersNote("REJECT missing_viotp_token")
		api.Print_json(c, "status", "0", "message", "Missing viotp token")
		return
	}

	serviceCode := querySingleString(conn, "SELECT `servicecode` FROM `service3` WHERE `id` = ? LIMIT 1", serviceID)
	if strings.TrimSpace(serviceCode) == "" {
		appendOrdersNote("REJECT service_not_found_in_service3")
		api.Print_json(c, "status", "0", "message", "Dich vu khong ton tai")
		return
	}

	country := normalizeCountry(countryRaw, cfg.DefaultCountry)
	networkNormalized := normalizeNetwork(network)
	if chedo == "" {
		chedo = "buycard"
	}

	params := url.Values{}
	params.Set("token", cfg.Token)
	params.Set("serviceId", serviceCode)
	if country != "" {
		params.Set("country", country)
	}
	if networkNormalized != "" {
		params.Set("network", networkNormalized)
	}
	if strings.TrimSpace(prefix) != "" {
		params.Set("prefix", prefix)
	}
	if strings.TrimSpace(exceptPrefix) != "" {
		params.Set("exceptPrefix", exceptPrefix)
	}

	requestNumber := strings.TrimSpace(requestNumberRaw)
	if chedo == "reset" {
		requestNumber = normalizeResetNumber(requestNumber, country)
		if requestNumber == "" {
			appendOrdersNote("REJECT reset_missing_number")
			api.Print_json(c, "status", "0", "message", "Reset can truyen so thue lai qua field url/number")
			return
		}
		params.Set("number", requestNumber)
	} else if strings.TrimSpace(requestNumber) != "" {
		params.Set("number", normalizeResetNumber(requestNumber, country))
	}

	endpoint := buildVIOTPRequestURL(cfg.BaseURL, params)
	response, rawResponse, requestErr := requestVIOTP(endpoint)
	appendOrdersLog(endpoint, rawResponse, requestErr, "")
	statusCode := toIntAny(response["status_code"])
	if statusCode == 200 && toBool(response["success"]) {
		data := toMap(response["data"])
		requestID := toString(data["request_id"])
		phoneNumber := toString(data["phone_number"])
		rePhoneNumber := toString(data["re_phone_number"])

		if country == "vn" {
			phoneNumber = normalizeVNPhone(phoneNumber)
		} else if cfg.AddLeadingZero && len(phoneNumber) == 9 {
			phoneNumber = "0" + phoneNumber
		}

		_, err := conn.Exec(
			"INSERT INTO `orders` SET `service_name` = ?, `category_code` = 'viotp', `service_id` = ?, `id_users` = ?, `username` = ?, `amount` = ?, `money` = ?, `note` = 'Vui long doi de lay ma otp', `url` = ?, `status` = 'Processing', `start_count` = ?, `code` = ?, `identifier` = ?, `api_url` = ?, `api_key` = ?, `createdate` = now()",
			nameService,
			serviceID,
			idUsers,
			myUsername,
			quantity,
			totalMoneyService,
			phoneNumber,
			0,
			code,
			requestID,
			nameService,
			rePhoneNumber,
		)
		if err != nil {
			appendOrdersNote("REJECT db_insert_order_failed")
			api.Print_json(c, "status", "0", "message", "Loi may chu")
			return
		}

		logContent := formatCash(myVND) + " - " + formatCash(totalMoneyService) + " = " + formatCash(myVND-totalMoneyService) + "  ly do: Thanh Toan Don " + nameService + " #" + code
		_, _ = conn.Exec(
			"INSERT INTO `log` SET `content` = ?, `createdate` = now(), `username` = ?",
			logContent,
			myUsername,
		)
		_, _ = conn.Exec(
			"UPDATE `users` SET `money` = `money` - ? WHERE `id` = ?",
			totalMoneyService,
			idUsers,
		)

		api.Print_json(c, "status", "1", "message", "Vui long vao lich su de lay so thue otp")
		return
	}

	errorMap := map[int]string{
		401: "Loi xac thuc",
		429: "Limit exceeded",
		-1:  "Co loi",
		-2:  "So du API khong du",
		-3:  "Kho so tam het",
		-4:  "Ứng dụng không tồn tại hoặc đang tạm ngưng",
	}

	message := ""
	if value, ok := errorMap[statusCode]; ok {
		message = value
	}
	if message == "" {
		message = firstNonEmpty(toString(response["message"]), "Viotp error")
	}

	_, _ = conn.Exec(
		"UPDATE `users` SET `money` = `money` - 1 WHERE `id` = ?",
		idUsers,
	)
	appendOrdersNote("VIOTP_FAIL status_code=" + toString(response["status_code"]) + " message=" + message)
	api.Print_json(c, "status", "0", "message", message)
}

func querySingleString(conn *sql.DB, query string, args ...any) string {
	var raw any
	if err := conn.QueryRow(query, args...).Scan(&raw); err != nil {
		return ""
	}
	return toString(raw)
}

func requestVIOTP(endpoint string) (map[string]any, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return map[string]any{}, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{}, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return map[string]any{}, string(body), err
	}
	raw := string(body)

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return map[string]any{}, raw, err
	}
	return data, raw, nil
}

func appendOrdersLog(endpoint string, rawResponse string, requestErr error, note string) {
	ordersLogMu.Lock()
	defer ordersLogMu.Unlock()

	var builder strings.Builder
	builder.WriteString("[")
	builder.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	builder.WriteString("]\n")

	if strings.TrimSpace(note) != "" {
		builder.WriteString("NOTE: ")
		builder.WriteString(note)
		builder.WriteString("\n")
	}

	if strings.TrimSpace(endpoint) != "" {
		builder.WriteString("URL: ")
		builder.WriteString(endpoint)
		builder.WriteString("\n")
	}

	if requestErr != nil {
		builder.WriteString("ERROR: ")
		builder.WriteString(requestErr.Error())
		builder.WriteString("\n")
	}

	builder.WriteString("RESPONSE: ")
	if strings.TrimSpace(rawResponse) == "" {
		builder.WriteString("(empty)")
	} else {
		builder.WriteString(rawResponse)
	}
	builder.WriteString("\n----\n")

	logPath := ordersLogPath()
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer logFile.Close()

	_, _ = logFile.WriteString(builder.String())
	_ = logFile.Sync()
}

func appendOrdersNote(note string) {
	appendOrdersLog("", "", nil, note)
}

func ordersLogPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "orders_log.txt"
	}
	return filepath.Join(filepath.Dir(file), "orders_log.txt")
}

func normalizeResetNumber(raw string, country string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var digitsBuilder strings.Builder
	digitsBuilder.Grow(len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digitsBuilder.WriteRune(r)
		}
	}
	digits := digitsBuilder.String()
	if digits == "" {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(country)) {
	case "vn":
		if strings.HasPrefix(digits, "0") {
			return digits
		}
		if strings.HasPrefix(digits, "84") {
			local := strings.TrimPrefix(digits, "84")
			if local == "" {
				return ""
			}
			return "0" + local
		}
		if len(digits) == 9 {
			return "0" + digits
		}
		return digits
	case "la":
		if strings.HasPrefix(digits, "856") {
			return digits
		}
		if strings.HasPrefix(digits, "0") {
			return "856" + strings.TrimLeft(digits, "0")
		}
		return digits
	default:
		return digits
	}
}
