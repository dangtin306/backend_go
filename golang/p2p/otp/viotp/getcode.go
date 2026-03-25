package otp_viotp

import (
	"math"
	"net/http"
	"strings"
	"time"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

func GetCodeHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	cfg := LoadConfig()
	country := normalizeCountry(c.Query("country"), cfg.DefaultCountry)
	if strings.TrimSpace(cfg.Token) == "" {
		api.Print_json(c, "status", 0, "message", "Missing viotp token")
		return
	}

	endpoint := makeServiceURL(cfg.BaseURL, cfg.Token, country)
	raw, httpCode, curlError := httpGetRaw(endpoint, 30*time.Second)
	debug := c.Query("debug") == "1"
	if debug {
		api.Print_json(c,
			"status", 0,
			"debug", true,
			"http_code", httpCode,
			"curl_error", curlError,
			"raw", raw,
		)
		return
	}

	if strings.TrimSpace(raw) == "" {
		api.Print_json(c,
			"status", 0,
			"message", "Empty response from viotp",
			"http_code", httpCode,
			"curl_error", curlError,
		)
		return
	}

	response := parseJSONMap(raw)
	if toInt(response["status_code"]) != 200 || !toBool(response["success"]) {
		message := toString(response["message"])
		if strings.TrimSpace(message) == "" {
			message = "Viotp error"
		}
		api.Print_json(c,
			"status", 0,
			"message", message,
			"http_code", httpCode,
			"curl_error", curlError,
			"raw", response,
		)
		return
	}

	servicesRaw := response["data"]
	services := []map[string]any{}
	switch value := servicesRaw.(type) {
	case []any:
		for _, item := range value {
			services = append(services, toMap(item))
		}
	case []map[string]any:
		services = value
	case string:
		services = parseJSONList(value)
	default:
		services = nil
	}

	if len(services) == 0 {
		api.Print_json(c,
			"status", 0,
			"message", "Empty service list",
			"raw", response,
		)
		return
	}

	conn, err := database.Open()
	if err != nil {
		api.Print_json(c, "status", 0, "message", "Loi ket noi Database")
		return
	}

	_, _ = conn.Exec(
		"DELETE FROM `service3` WHERE `category` = 'viotp' AND `api_categorymini` = ?",
		country,
	)

	inserted := 0
	for _, service := range services {
		name := strings.TrimSpace(toString(service["name"]))
		if name == "" {
			continue
		}

		serviceID := toString(service["id"])
		stt := toInt(serviceID)
		priceRaw := toFloat(service["price"])
		price := math.Ceil(priceRaw * cfg.PriceMultiplier)

		_, err := conn.Exec(
			"INSERT INTO `service3` SET `status` = 'show', `stt` = ?, `name` = ?, `minorder` = '0', `maxorder` = '10', `content` = ?, `money` = ?, `servicecode` = ?, `category` = 'viotp', `api_category` = ?, `api_categorymini` = ?, `time` = now()",
			stt,
			name,
			name,
			price,
			serviceID,
			cfg.BaseURL,
			country,
		)
		if err == nil {
			inserted++
		}
	}

	api.Print_json(c,
		"status", 1,
		"message", "updated",
		"country", country,
		"total", len(services),
		"inserted", inserted,
	)
}
