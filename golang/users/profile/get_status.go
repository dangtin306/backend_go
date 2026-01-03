package profile

import (
	"net/http"

	"hust_backend/main/api"

	"github.com/gin-gonic/gin"
)

func GetStatusHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	api.Print_json(c, 123)
}
