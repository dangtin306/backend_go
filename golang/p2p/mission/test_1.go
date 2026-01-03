package mission

import (
	"hust_backend/main/api"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

func GetlinkTest1Handler(c *gin.Context) {
	var create = play_sql.Query("INSERT INTO `misson_shorten_code` SET `tele_code` = '1', `createdate` = now()")
	api.Print_json(c, "status", create)
}
