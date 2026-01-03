package mission

import (
	"hust_backend/main/api"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

func GetlinkTest3Handler(c *gin.Context) {
	var row = play_sql.Query(
		"SELECT COUNT(*) FROM `misson_shorten` WHERE `api_category` = 'https://ez4short.com' " +
			"AND DATE(mission_updatedate) >= DATE_SUB(CURRENT_DATE(), INTERVAL 3 DAY) " +
			"AND `iduser` = '74568' AND `status` = 'Completed'",
	)
	var views_count = row.Fetch_array()["COUNT(*)"]
	api.Print_json(c, "views_count", views_count)
}
