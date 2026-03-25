package otp_process

import (
	"net/http"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

func GetListHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	conn, err := database.Open()
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}

	rows, err := conn.Query(
		"SELECT `id`, `stt`, `money`, `name`, `content` FROM `service3` "+
			"WHERE `category` = 'viotp' AND `status` = 'show' AND `api_categorymini` = ? "+
			"ORDER BY CASE WHEN `name` LIKE '%other%' THEN 0 ELSE 1 END, `stt`",
		"vn",
	)
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}
	defer rows.Close()

	services := make([]map[string]any, 0)
	for rows.Next() {
		var id, stt, money, name, content string
		if err := rows.Scan(&id, &stt, &money, &name, &content); err != nil {
			continue
		}
		services = append(services, map[string]any{
			"id":      id,
			"stt":     stt,
			"money":   money,
			"name":    name,
			"content": content,
		})
	}

	api.Print_json(c, services)
}
