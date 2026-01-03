package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func SetHeaders(c *gin.Context) {
	if c == nil {
		return
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		origin = "*"
	}
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Vary", "Origin")

	requestHeaders := strings.TrimSpace(c.GetHeader("Access-Control-Request-Headers"))
	if requestHeaders == "" {
		requestHeaders = "Content-Type, Authorization, X-Requested-With"
	}
	c.Header("Access-Control-Allow-Headers", requestHeaders)
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS")
	c.Header("Access-Control-Allow-Credentials", "true")

	server := strings.TrimSpace(c.Request.Host)
	if server == "" {
		server = strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	}
	if server == "" {
		server = strings.TrimSpace(c.GetHeader("Host"))
	}
	if server == "" {
		server = "hust.media"
	}
	c.Header("server", server)
	c.Header("Content-Type", "application/json; charset=UTF-8")
	c.Header("x-hustmedia-region", "AWS - ap-southeast-1")
}
