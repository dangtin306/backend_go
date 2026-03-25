package otp_process

import (
	"net/http"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

func HistoryHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	conn, err := database.Open()
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}

	apikey := c.Query("apikey")
	idUsers := ""
	_ = conn.QueryRow("SELECT `id_users` FROM `users_key` WHERE `users_apikey` = ? LIMIT 1", apikey).Scan(&idUsers)

	username := ""
	if idUsers != "" {
		_ = conn.QueryRow("SELECT `username` FROM `users` WHERE `id` = ? LIMIT 1", idUsers).Scan(&username)
	}

	rows, err := conn.Query(
		"SELECT `service_id`, `code`, `username`, `service_name`, `money`, `orders_order`, `createdate`, `updatedate`, `note`, `status`, `url` "+
			"FROM `orders` WHERE `username` = ? AND `category_code` = 'viotp' ORDER BY `id` DESC",
		username,
	)
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	cols, err := rows.Columns()
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			continue
		}
		row := map[string]string{}
		for i, col := range cols {
			row[col] = toString(values[i])
		}
		results = append(results, map[string]any{
			"service_id":   row["service_id"],
			"code":         row["code"],
			"username":     row["username"],
			"service_name": row["service_name"],
			"money":        row["money"],
			"order":        row["orders_order"],
			"createdate":   row["createdate"],
			"updatedate":   row["updatedate"],
			"note":         row["note"],
			"status":       row["status"],
			"url":          row["url"],
		})
	}

	api.Print_json(c, results)
}
