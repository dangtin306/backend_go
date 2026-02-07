package main

import (
	"net/http"

	"hust_backend/main/database"
	"hust_backend/main/scheduler"
	"hust_backend/p2p/media/social/telegram"
	"hust_backend/p2p/mission"
	"hust_backend/p2p/mission/p2p_link"
	"hust_backend/p2p/scam_check"
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
	r.GET("/social/telegram/auto_reply", telegram.AutoReplyHandler)


	r.StaticFile("/site_map/check_healing.xml", "main/site_map/check_healing.xml")
	r.Any("/profile/plan/plan_orders", profile.PlanOrdersHandler)
	r.Any("/profile/plan/list_plan", profile.ListPlanHandler)
	r.Any("/profile/setting/get_status", profile.GetStatusHandler)
	r.Any("/profile/setting/get_balance", profile.GetStatusHandler)
	r.Any("/database/export_data", database.ExportDataHandler)
	r.GET("/servers/scheduler/get_data", scheduler.GetDataHandler)
}


