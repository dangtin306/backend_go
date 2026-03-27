//go:build openclaw

package main

import (
	openclawservice "hust_backend/service/openclaw"

	"github.com/gin-gonic/gin"
)

func registerOpenClawRoutes(r *gin.Engine) {
	r.Any("/service/openclaw/approve_pairing", openclawservice.OpenClawApprovePairingHandler)
	r.Any("/service/openclaw/desktop/status", openclawservice.OpenClawDesktopStatusHandler)
	r.Any("/service/openclaw/desktop/list_windows", openclawservice.OpenClawDesktopListWindowsHandler)
	r.Any("/service/openclaw/desktop/activate_window", openclawservice.OpenClawDesktopActivateWindowHandler)
	r.Any("/service/openclaw/desktop/click", openclawservice.OpenClawDesktopClickHandler)
	r.Any("/service/openclaw/desktop/send_keys", openclawservice.OpenClawDesktopSendKeysHandler)
	r.Any("/service/openclaw/desktop/run_ahk", openclawservice.OpenClawDesktopRunAhkHandler)
}

