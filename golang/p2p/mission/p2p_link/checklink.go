package p2p_link

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hust_backend/main/api"
	"hust_backend/main/database"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

var (
	dataTimeOnce sync.Once
	dataTimeMap  map[string]map[string]any
)

func ChecklinkHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	apikey := strings.TrimSpace(c.Query("apikey"))
	category := strings.TrimSpace(c.Query("category"))
	service_id := strings.TrimSpace(c.Query("service"))

	id_users := toString(play_sql.Query("SELECT `id_users` FROM `users_key` WHERE `users_apikey` = '"+apikey+"' ").Fetch_array()["id_users"])
	email := toString(play_sql.Query("SELECT `email_users6` FROM `users_device_list` WHERE `iduser` = '"+id_users+"'   ").Fetch_array()["email_users6"])

	level_users := toString(play_sql.Query("SELECT `level` FROM `users2` WHERE `id_users` = '"+id_users+"' ").Fetch_array()["level"])
	user2_id := toString(play_sql.Query("SELECT `id` FROM `users2` WHERE `id_users` = '"+id_users+"' ").Fetch_array()["id"])
	if user2_id == "" {
		play_sql.Query("INSERT INTO `users2` SET `id_users` = '" + id_users + "', `checkdatdon` = '0'  ")
	}
	level_users = toString(play_sql.Query("SELECT `level` FROM `users2` WHERE `id_users` = '"+id_users+"' ").Fetch_array()["level"])

	if category == "rollup_123456" && email == "" {
		api.Print_json(c,
			"status", "0",
			"message", "Vui lòng liên kết google để làm nhiệm vụ này",
			"redirect", "/shop/profiles?webappmode=showadview",
		)
		return
	}

	query := buildMissionQuery(service_id, id_users, category)
	rows, err := queryRows(query)
	if err != nil {
		api.Print_json(c, []map[string]any{})
		return
	}

	dataTime := loadDataTime()
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		mergePartnerInfo(row)
		api_category := row["api_category"]
		minorder := row["minorder"]
		maxorder := row["maxorder"]
		level_mission := row["level"]
		time_value := row["time"]

		hientai := 0
		if minorder == "0" {
			hientai = toInt(toString(play_sql.Query("SELECT COUNT(*) FROM `misson_shorten` WHERE `api_category` = '"+api_category+"' AND DATE(mission_updatedate) >= DATE_SUB(CURRENT_DATE(), INTERVAL "+maxorder+" DAY) AND `iduser` =  '"+id_users+"'  AND `status` = 'Completed'").Fetch_array()["COUNT(*)"]))
		}

		createdate := toString(play_sql.Query("SELECT `mission_updatedate` FROM `misson_shorten` WHERE  `api_category` = '"+api_category+"' AND DATE(mission_updatedate) = CURRENT_DATE() AND `iduser` =  '"+id_users+"' AND `status` =  'Completed' ORDER BY id desc limit 1  ").Fetch_array()["mission_updatedate"])
		timeDelay := any("")
		if createdate != "" {
			if minorder != "0" {
				totalOrders := toInt(toString(play_sql.Query("SELECT COUNT(*) FROM `misson_shorten` WHERE `api_category` = '"+api_category+"' AND DATE(mission_updatedate) = CURRENT_DATE() AND `iduser` =  '"+id_users+"' AND `status` = 'Completed'  ").Fetch_array()["COUNT(*)"]))
				hientai = toInt(maxorder) - totalOrders
			}
			if hientai != 0 {
				if delaySeconds, ok := computeTimeDelaySeconds(createdate, toInt(time_value)); ok {
					timeDelay = delaySeconds
				}
			}
		} else {
			hientai = toInt(maxorder)
		}

		item := buildItem(row, category, hientai, timeDelay, dataTime[api_category])
		if shouldSkipLevel(level_users, level_mission) {
			continue
		}
		results = append(results, item)
	}

	api.Print_json(c, results)
}

func buildMissionQuery(service_id, id_users, category string) string {
	if service_id != "" {
		return "SELECT * FROM `mission_partner_main` WHERE `status` = 'show' AND `id` = '" + service_id + "' "
	}
	if id_users == "" {
		return "SELECT * FROM `mission_partner_main` WHERE `category` = 'newer' AND `status` = 'show' ORDER BY `stt` ASC LIMIT 100 "
	}
	if category == "rollup" || category == "option_viewer" || category == "rewarded_ads" || category == "offerwall" || category == "p2p_mmo" || category == "videos_check" {
		return "SELECT * FROM `mission_partner_main` WHERE `category` LIKE '%" + category + "%' AND `status` = 'show' ORDER BY `stt` ASC LIMIT 100 "
	}
	if category == "hustadmin" {
		return "SELECT * FROM `mission_partner_main` WHERE (`category` = 'option_viewer' OR `category` = 'rollup') AND `status` = 'show' ORDER BY `stt` ASC LIMIT 100 "
	}
	return "SELECT * FROM `mission_partner_main` WHERE `category` = 'option_viewer' AND `status` = 'show' ORDER BY `stt` ASC LIMIT 100 "
}

func buildItem(row map[string]string, category string, views int, timeDelay any, dataTime any) map[string]any {
	if dataTime == nil {
		dataTime = false
	}
	if category == "offerwall" {
		return map[string]any{
			"id":          row["id"],
			"stt":         row["stt"],
			"name":        row["mission_name"],
			"level":       row["level"],
			"description": row["mission_note"],
			"price":       row["money"],
			"limit":       row["servicecode"],
			"maxorder":    row["maxorder"],
			"type_api":    row["type_api"],
			"data_time":   dataTime,
			"views":       views,
			"url":         row["api_url"],
			"time":        row["time"],
			"minorder":    row["minorder"],
		}
	}
	if category == "hustadmin" {
		return map[string]any{
			"id":               row["id"],
			"stt":              row["stt"],
			"name":             row["mission_name"],
			"level":            row["level"],
			"time_delay":       timeDelay,
			"description":      row["mission_note"],
			"price":            row["money"],
			"limit":            row["servicecode"],
			"maxorder":         row["maxorder"],
			"type_api":         row["type_api"],
			"api_category":     row["api_category"],
			"api_categorymini": row["api_categorymini"],
			"data_time":        dataTime,
			"views":            views,
			"time":             row["time"],
			"minorder":         row["minorder"],
		}
	}
	return map[string]any{
		"id":               row["id"],
		"stt":              row["stt"],
		"name":             row["mission_name"],
		"level":            row["level"],
		"time_delay":       timeDelay,
		"description":      row["mission_note"],
		"price":            row["money"],
		"limit":            row["servicecode"],
		"maxorder":         row["maxorder"],
		"type_api":         row["type_api"],
		"api_categorymini": row["api_categorymini"],
		"data_time":        dataTime,
		"views":            views,
		"time":             row["time"],
		"minorder":         row["minorder"],
	}
}

func shouldSkipLevel(level_users, level_mission string) bool {
	if level_users == "" || level_mission == "" {
		return false
	}
	return toInt(level_users) < toInt(level_mission)
}

func loadDataTime() map[string]map[string]any {
	dataTimeOnce.Do(func() {
		dataTimeMap = map[string]map[string]any{}
		baseDir, err := os.Getwd()
		if err != nil {
			baseDir = "."
		}
		path := filepath.Join(baseDir, "p2p", "mission", "p2p_link", "time", "data_time_done.json")
		content, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var raw []map[string]any
		if err := json.Unmarshal(content, &raw); err != nil {
			return
		}
		for _, entry := range raw {
			value, ok := entry["api_category"]
			if !ok {
				continue
			}
			key, ok := value.(string)
			if !ok || key == "" {
				continue
			}
			delete(entry, "api_category")
			dataTimeMap[key] = entry
		}
	})
	return dataTimeMap
}

func queryRows(query string) ([]map[string]string, error) {
	conn, err := database.Open()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]string, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			case nil:
				row[col] = ""
			default:
				row[col] = toString(v)
			}
		}
		results = append(results, row)
	}
	return results, nil
}

