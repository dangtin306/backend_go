package mission

import (
	"hust_backend/main/api"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

func GetlinkTest2Handler(c *gin.Context) {
	var value = play_sql.Query("SELECT * FROM `users` WHERE `id` = 207638 ORDER BY `id` DESC").Fetch_array()["money"]

	api.Print_json(c, "value", value)
}
