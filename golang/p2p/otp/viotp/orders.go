package otp_viotp

import (
	otp_process "hust_backend/p2p/otp/process"

	"github.com/gin-gonic/gin"
)

// Reuse the /otp/process/orders logic and expose a dedicated /otp/viotp/orders route.
func OrdersHandler(c *gin.Context) {
	otp_process.OrdersHandler(c)
}
