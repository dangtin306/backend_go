package otp_viotp

import (
	"net/http"
	"strings"
	"time"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

func UpdateHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	cfg := LoadConfig()
	if strings.TrimSpace(cfg.Token) == "" {
		api.Print_json(c, "status", 0, "message", "Missing viotp token")
		return
	}

	conn, err := database.Open()
	if err != nil {
		api.Print_json(c, "status", 0, "message", "Loi ket noi Database")
		return
	}

	hasRow := ""
	_ = conn.QueryRow(
		"SELECT `id` FROM `orders` WHERE `category_code` = 'viotp' AND (`status` = 'Processing' OR `status` = 'In progress') ORDER BY `id` DESC LIMIT 1",
	).Scan(&hasRow)
	if strings.TrimSpace(hasRow) == "" {
		c.String(http.StatusOK, "chua ai thue")
		return
	}

	rows, err := conn.Query(
		"SELECT `id`, `createdate`, `status`, `identifier`, `username`, `money` FROM `orders` WHERE `category_code` = 'viotp' AND (`status` = 'Processing' OR `status` = 'In progress') ORDER BY `id` DESC LIMIT 3",
	)
	if err != nil {
		api.Print_json(c, "status", 0, "message", "Loi truy van orders")
		return
	}
	defer rows.Close()

	processed := 0
	completed := 0
	canceled := 0
	inProgress := 0

	for rows.Next() {
		var id, status, requestID, username string
		var created any
		var money any
		if err := rows.Scan(&id, &created, &status, &requestID, &username, &money); err != nil {
			continue
		}

		statusTrim := strings.TrimSpace(status)
		if statusTrim != "Processing" && statusTrim != "In progress" {
			continue
		}

		createdAt, ok := parseDBTime(toString(created))
		if !ok {
			createdAt = time.Now()
		}
		minutesPassed := int(time.Since(createdAt).Minutes())
		if minutesPassed < 0 {
			minutesPassed = -minutesPassed
		}

		totalMoney := toFloat(money)
		endpoint := makeSessionURL(cfg.BaseURL, cfg.Token, requestID)
		raw, _, _ := httpGetRaw(endpoint, 30*time.Second)
		response := parseJSONMap(raw)
		statusCode := toInt(response["status_code"])
		message := toString(response["message"])

		if statusCode != 200 || !toBool(response["success"]) {
			if minutesPassed > cfg.TimeoutMinutes {
				note := "Het han otp tru " + formatCash(cfg.SpamPenalty) + " coins tranh spam"
				_, _ = conn.Exec(
					"UPDATE `users` SET `money` = `money` + ? - ? WHERE `username` = ?",
					totalMoney,
					cfg.SpamPenalty,
					username,
				)
				_, _ = conn.Exec(
					"UPDATE `orders` SET `status` = 'Canceled', `updatedate` = now(), `orders_order` = '=))', `note` = ? WHERE `id` = ?",
					note,
					id,
				)
				canceled++
			} else if strings.TrimSpace(message) != "" {
				_, _ = conn.Exec(
					"UPDATE `orders` SET `updatedate` = now(), `orders_order` = ? WHERE `id` = ?",
					message,
					id,
				)
			}
			processed++
			continue
		}

		data := toMap(response["data"])
		otpStatus := toInt(data["Status"])
		code := toString(data["Code"])
		sms := toString(data["SmsContent"])
		isSound := 0
		if toBool(data["IsSound"]) {
			isSound = 1
		}

		if otpStatus == 1 {
			_, _ = conn.Exec(
				"UPDATE `orders` SET `status` = 'Completed', `updatedate` = now(), `note` = ?, `identifier` = ?, `orders_order` = ? WHERE `id` = ?",
				code,
				isSound,
				sms,
				id,
			)
			completed++
		} else if otpStatus == 0 && minutesPassed < cfg.TimeoutMinutes {
			_, _ = conn.Exec(
				"UPDATE `orders` SET `status` = 'In progress', `updatedate` = now(), `note` = 'Doi tin nhan', `orders_order` = ? WHERE `id` = ?",
				sms,
				id,
			)
			inProgress++
		} else if otpStatus == 2 || minutesPassed > cfg.TimeoutMinutes {
			note := "Het han otp tru " + formatCash(cfg.SpamPenalty) + " coins"
			_, _ = conn.Exec(
				"UPDATE `users` SET `money` = `money` + ? - ? WHERE `username` = ?",
				totalMoney,
				cfg.SpamPenalty,
				username,
			)
			_, _ = conn.Exec(
				"UPDATE `orders` SET `status` = 'Canceled', `updatedate` = now(), `orders_order` = '=))', `note` = ? WHERE `id` = ?",
				note,
				id,
			)
			canceled++
		}

		processed++
	}

	api.Print_json(c,
		"status", 1,
		"message", "updated",
		"processed", processed,
		"completed", completed,
		"in_progress", inProgress,
		"canceled", canceled,
	)
}
