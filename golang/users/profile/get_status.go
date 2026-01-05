package profile

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hust_backend/main/api"
	"hust_backend/main/database"
)

func GetStatusHandler(c *gin.Context) {
	api.SetHeaders(c)

	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	apiKey := strings.TrimSpace(c.Query("apikey"))
	if apiKey == "" {
		api.Print_json(c, "status", 0, "result", "Vui lòng nhập API Key")
		return
	}

	db, err := database.Open()
	if err != nil {
		api.Print_json(c, "status", 0, "result", "Lỗi kết nối Database")
		return
	}

	mode := c.Query("mode")

	// =================================================================
	// MODE: STATUS
	// =================================================================
	if mode == "status" {
		var statusDetail string
		var usersStatus string
		var idUsers string

		// Logic JOIN 3 bảng:
		// users_key (Lấy ID) -> users_status_lists (Lấy mã status) -> users_status_detail (Lấy tên status)
		// Lưu ý: usl là viết tắt của users_status_lists, usd là users_status_detail
		queryStatus := `
			SELECT usd.status_detail, usl.users_status, usl.id_users
			FROM users_key uk
			JOIN users_status_lists usl ON uk.id_users = usl.id_users
			JOIN users_status_detail usd ON usl.users_status = usd.users_status
			WHERE uk.users_apikey = ?
			LIMIT 1
		`

		err = db.QueryRow(queryStatus, apiKey).Scan(&statusDetail, &usersStatus, &idUsers)

		if err != nil {
			fmt.Println("Debug SQL Status Error:", err)
			// Nếu lỗi này xảy ra, khả năng cao là user có Key nhưng chưa được thêm vào bảng users_status_lists
			api.Print_json(c, "status", 0, "result", "User chưa có trạng thái hoặc sai Key")
			return
		}

		api.Print_json(c,
			"status", 1,
			"status_detail", statusDetail,
			"users_status", usersStatus,
			"id_users", idUsers,
		)
		return
	}

	// =================================================================
	// MODE: OVERVIEW (Mặc định)
	// =================================================================
	var username string
	var money int64

	queryUser := `
		SELECT u.username, COALESCE(u.money, 0)
		FROM users u
		JOIN users_key k ON u.id = k.id_users
		WHERE k.users_apikey = ?
		LIMIT 1
	`

	err = db.QueryRow(queryUser, apiKey).Scan(&username, &money)

	if err != nil {
		api.Print_json(c, "status", 0, "result", "API Key không tồn tại hoặc sai")
		return
	}

	api.Print_json(c,
		"status", 1,
		"result", "Lấy thông tin thành công",
		"money", strconv.FormatInt(money, 10),
		"username", username,
	)
}
