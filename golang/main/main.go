package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

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

	if shouldStartScheduler() {
		if err := scheduler.StartCounter(); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("scheduler disabled")
	}

	// 3. Chạy server ở cổng 8795
	// (Nginx sẽ hứng từ cổng 8888 rồi đẩy vào cổng 8795 này)
	r.Run(":8795")
}

func shouldStartScheduler() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GOLANG_SCHEDULER")))
	switch mode {
	case "1", "true", "on", "enable", "enabled":
		return true
	case "0", "false", "off", "disable", "disabled":
		return false
	}

	// auto mode (default): disable scheduler on machines without OpenClaw files.
	openClawCheckPath := filepath.Join("service", "openclaw", "check_openclaw.py")
	if _, err := os.Stat(openClawCheckPath); err != nil {
		return false
	}
	return true
}
