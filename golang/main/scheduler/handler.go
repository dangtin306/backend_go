package scheduler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func GetDataHandler(c *gin.Context) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}