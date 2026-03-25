package main

import (
	"net/http"

	"hust_backend/main/database"
	"hust_backend/main/scheduler"
	"hust_backend/p2p/media"
	"hust_backend/p2p/media/social/telegram"
	"hust_backend/p2p/mission"
	"hust_backend/p2p/mission/p2p_link"
	otp_process "hust_backend/p2p/otp/process"
	otp_viotp "hust_backend/p2p/otp/viotp"
	"hust_backend/p2p/scam_check"
	"hust_backend/service"
	openclawservice "hust_backend/service/openclaw"
	"hust_backend/users/profile"

	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine) {
	// Trang chá»§ check backend
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"project": "Hust Media",
			"status":  "Backend Golang is Running!",
			"port":    8795,
		})
	})

	// API test dá»¯ liá»‡u
	r.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Ket noi thanh cong voi Gin Framework",
			"server":  "Windows Server 2025",
		})
	})

	r.GET("/mission/getlink/test_1", mission.GetlinkTest1Handler)
	r.GET("/mission/getlink/test_2", mission.GetlinkTest2Handler)
	r.GET("/mission/getlink/test_3", mission.GetlinkTest3Handler)
	r.Any("/mission/p2p_link/getlink", p2p_link.GetlinkRunHandler)
	r.Any("/mission/p2p_link/checklink", p2p_link.ChecklinkHandler)

	r.GET("/check/xml_save_data", scam_check.XmlSaveDataHandler)
	r.GET("/media/xml_save_hust_media", media.HustMediaXmlSaveHandler)
	r.GET("/social/telegram/auto_reply", telegram.AutoReplyHandler)
	r.Any("/otp/process/history", otp_process.HistoryHandler)
	r.Any("/otp/process/getlist", otp_process.GetListHandler)
	r.Any("/otp/process/orders", otp_process.OrdersHandler)
	r.Any("/otp/viotp/getcode", otp_viotp.GetCodeHandler)
	r.Any("/otp/viotp/update", otp_viotp.UpdateHandler)
	r.Any("/otp/viotp/orders", otp_viotp.OrdersHandler)

	r.StaticFile("/site_map/check_healing.xml", "main/site_map/check_healing.xml")
	r.StaticFile("/site_map/hust_media.xml", "main/site_map/hust_media.xml")
	r.Any("/profile/plan/plan_orders", profile.PlanOrdersHandler)
	r.Any("/profile/plan/list_plan", profile.ListPlanHandler)
	r.Any("/profile/setting/get_status", profile.GetStatusHandler)
	r.Any("/profile/setting/get_balance", profile.GetStatusHandler)
	r.Any("/database/export_data", database.ExportDataHandler)
	r.Any("/service/money_update", service.UpdateMoneyHandler)
	r.Any("/service/money_list", service.MoneyListHandler)
	r.Any("/service/openclaw/approve_pairing", openclawservice.OpenClawApprovePairingHandler)
	r.Any("/service/openclaw/desktop/status", openclawservice.OpenClawDesktopStatusHandler)
	r.Any("/service/openclaw/desktop/list_windows", openclawservice.OpenClawDesktopListWindowsHandler)
	r.Any("/service/openclaw/desktop/activate_window", openclawservice.OpenClawDesktopActivateWindowHandler)
	r.Any("/service/openclaw/desktop/click", openclawservice.OpenClawDesktopClickHandler)
	r.Any("/service/openclaw/desktop/send_keys", openclawservice.OpenClawDesktopSendKeysHandler)
	r.Any("/service/openclaw/desktop/run_ahk", openclawservice.OpenClawDesktopRunAhkHandler)
	r.GET("/servers/scheduler/get_data", scheduler.GetDataHandler)
}
