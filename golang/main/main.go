package main

import (
	"log"

	"hust_backend/main/api"
	"hust_backend/main/scheduler"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Khởi tạo router
	r := gin.Default()
	r.RemoveExtraSlash = true
	r.Use(func(c *gin.Context) {
		api.SetHeaders(c)
		c.Next()
	})

	// 2. Định nghĩa các đường dẫn (API)
	registerRoutes(r)

	if err := scheduler.StartCounter(); err != nil {
		log.Fatal(err)
	}

	// 3. Chạy server ở cổng 8795
	// (Nginx sẽ hứng từ cổng 8888 rồi đẩy vào cổng 8795 này)
	r.Run(":8795")
}
