package profile

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	// Import 2 package quan trọng của bạn
	"hust_backend/main/api"      // Chứa headers.go và json_print.go
	"hust_backend/main/database" // Chứa config_1.go
)

func GetStatusHandler(c *gin.Context) {
	// 1. Set Headers chuẩn (Dùng hàm trong headers.go)
	api.SetHeaders(c)

	// Xử lý Preflight request
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	// 2. Lấy API Key
	apiKey := strings.TrimSpace(c.Query("apikey"))
	if apiKey == "" {
		// Dùng api.Print_json để trả lỗi
		api.Print_json(c, "status", 0, "result", "Vui lòng nhập API Key")
		return
	}

	// 3. Kết nối DB (Dùng hàm Open trong config_1.go)
	db, err := database.Open()
	if err != nil {
		api.Print_json(c, "status", 0, "result", "Lỗi kết nối Database")
		return
	}
	// Không cần defer db.Close() vì config_1.go dùng kết nối chung (Singleton)

	// 4. Query dữ liệu
	// Lưu ý: Tìm trong bảng 'users', cột 'key' (phải có dấu huyền `key`)
	var username string
	var money int64

	query := "SELECT username, COALESCE(money, 0) FROM users WHERE `key` = ? LIMIT 1"

	err = db.QueryRow(query, apiKey).Scan(&username, &money)

	if err != nil {
		// Lỗi không tìm thấy hoặc lỗi SQL
		fmt.Println("Debug lỗi SQL:", err) // In ra terminal để debug nếu cần
		api.Print_json(c, "status", 0, "result", "API Key không tồn tại hoặc sai")
		return
	}

	// 5. Trả về kết quả thành công (Dùng api.Print_json cho gọn)
	// Hàm này sẽ tự đóng gói thành JSON: {"status": 1, "result": "...", ...}
	api.Print_json(c,
		"status", 1,
		"result", "Lấy thông tin thành công",
		"money", strconv.FormatInt(money, 10), // Chuyển số thành chuỗi
		"username", username,
	)
}
