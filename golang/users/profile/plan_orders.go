package profile

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hust_backend/main/api"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

func PlanOrdersHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	print_json := api.MakePrintJSON(c)
	payload := parsePlanPayload(c)
	my_apikey, has_apikey := payload["key"]
	mucnhan := strings.TrimSpace(payload["mucnhan"])
	my_apikey = strings.TrimSpace(my_apikey)

	id_users := play_sql.ToString(play_sql.Query("SELECT `id_users` FROM `users_key` WHERE `users_apikey` = '" + my_apikey + "' ").Fetch_array()["id_users"])
	my_username := play_sql.ToString(play_sql.Query("SELECT `username` FROM `users` WHERE `id` = '" + id_users + "' ").Fetch_array()["username"])
	my_total_nap_raw := play_sql.ToString(play_sql.Query("SELECT `total_nap` FROM `users` WHERE `id` = '" + id_users + "' ").Fetch_array()["total_nap"])
	moneyusername_raw := play_sql.ToString(play_sql.Query("SELECT `money` FROM `users` WHERE `id` = '" + id_users + "' ").Fetch_array()["money"])

	level_row := play_sql.Query("SELECT * FROM `capdotaikhoan` WHERE `id` = '" + mucnhan + "' ").Fetch_array()
	namelevel := play_sql.ToString(level_row["namelevel"])
	soxunap_raw := play_sql.ToString(level_row["soxunap"])
	thucnhan_raw := play_sql.ToString(level_row["thucnhan"])
	tongxunhan_raw := play_sql.ToString(level_row["tongxunhan"])
	chietkhaugiam := strings.TrimSpace(play_sql.ToString(level_row["chietkhaugiam"]))

	play_sql.Query("UPDATE `users` SET `money` = `money` - 1 WHERE `id` = '" + id_users + "' ")
	checkdatdon := play_sql.ToString(play_sql.Query("SELECT `checkdatdon` FROM `users2` WHERE `id_users` = '" + id_users + "'  ").Fetch_array()["checkdatdon"])

	defer func() {
		time.Sleep(time.Second)
		play_sql.Query("UPDATE `users2`  SET `checkdatdon` =  0 WHERE `id_users` = '" + id_users + "' ")
	}()

	if !has_apikey {
		print_json(api.J(
			"error", "Vui lòng đăng nhập để tiếp tục.",
		))
		return
	}
	if checkdatdon == "1" {
		print_json(api.J(
			"error", "Thao tác quá nhanh vui lòng đợi một xíu rồi đặt lại",
		))
		return
	}
	if my_username == "" {
		print_json(api.J(
			"error", "Vui lòng đăng nhập để tiếp tục.",
		))
		return
	}

	user2_id := play_sql.ToString(play_sql.Query("SELECT `id` FROM `users2` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["id"])
	if user2_id == "" {
		play_sql.Query("INSERT INTO `users2` SET `id_users` = '" + id_users + "', `checkdatdon` = '0'  ")
	} else {
		play_sql.Query("UPDATE `users2` SET `checkdatdon` =  1 WHERE `id_users` = '" + id_users + "' ")
	}

	id_users = play_sql.ToString(play_sql.Query("SELECT `id` FROM `users` WHERE `id` = '" + id_users + "' ").Fetch_array()["id"])
	user4_username := play_sql.ToString(play_sql.Query("SELECT `username` FROM `users4` WHERE `iduser` = '" + id_users + "' ").Fetch_array()["username"])
	if user4_username == "" {
		play_sql.Query("INSERT INTO `users4` SET `username` = '" + my_username + "', `checkspin` = now() , `iduser` = '" + id_users + "'  ")
	}

	my_tongxunhan_raw := play_sql.ToString(play_sql.Query("SELECT `tongxunhan` FROM `users2` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["tongxunhan"])
	my_tongxunhan := play_sql.ToFloat(my_tongxunhan_raw)
	if strings.TrimSpace(my_tongxunhan_raw) == "" {
		my_tongxunhan = 0
		play_sql.Query("UPDATE `users2` SET `tongxunhan` = " + formatFloat(my_tongxunhan) + "  WHERE `id_users` = '" + id_users + "' ")
	}

	level := play_sql.ToInt(play_sql.ToString(play_sql.Query("SELECT `level` FROM `users2` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["level"]))
	mucnhan_int := play_sql.ToInt(mucnhan)

	soxunap := play_sql.ToFloat(soxunap_raw)
	thucnhan := play_sql.ToFloat(thucnhan_raw)
	tongxunhan := play_sql.ToFloat(tongxunhan_raw)
	thucnhanx2 := thucnhan * 2
	thucnhanx3 := thucnhan * 3

	total_moneyok := play_sql.ToInt(play_sql.ToString(play_sql.Query("SELECT COUNT(*) FROM `orders` WHERE `category_code` = 'updateacount' AND DATE(createdate) = CURRENT_DATE() AND `api_url` = '" + my_apikey + "' ").Fetch_array()["COUNT(*)"]))
	moneyusername := play_sql.ToFloat(moneyusername_raw)
	my_total_nap := play_sql.ToFloat(my_total_nap_raw)

	if moneyusername < thucnhanx3 && soxunap > 1500000 {
		print_json(api.J(
			"error", "Số dư ko được nhỏ hơn 3 lần coins thưởng là "+formatFloat(thucnhanx2)+" coins.",
		))
		return
	} else if total_moneyok >= 1 {
		print_json(api.J(
			"error", "Mỗi ngày nâng cấp tối đa "+strconv.Itoa(total_moneyok)+" lần!",
		))
		return
	} else if moneyusername < thucnhanx2 && soxunap > 200000 {
		print_json(api.J(
			"error", "Số dư ko được nhỏ hơn 2 lần coins thưởng là "+formatFloat(thucnhanx2)+" coins.",
		))
		return
	} else if moneyusername < thucnhan {
		print_json(api.J(
			"error", "Số dư ko được nhỏ hơn số coins thưởng "+formatFloat(thucnhan)+" coins.",
		))
		return
	} else if my_total_nap < soxunap {
		print_json(api.J(
			"error", "Tổng coins kiếm được "+formatFloat(my_total_nap)+"  ko được nhỏ hơn mức coin "+formatFloat(soxunap)+" coins.",
		))
		return
	} else if mucnhan_int-level > 1 {
		print_json(api.J(
			"error", "Bạn phải thăng cấp từ từ kiểu từ level 1 đến 2",
		))
		return
	} else if level >= mucnhan_int {
		print_json(api.J(
			"error", "Bạn đã thăng cấp level này rồi",
		))
		return
	} else if my_tongxunhan > tongxunhan {
		print_json(api.J(
			"error", "Lỗi tổng coins nhận vui lòng liên hệ admin",
		))
		return
	}

	play_sql.Query("INSERT INTO `orders` SET `service_name` = 'Thăng cấp tài khoản ', `api_url` = '" + my_apikey + "' , `status` = 'Completed' , `username` = '" + my_username + "' , `category_code` = 'updateacount' , `createdate` = now()  ")
	checkok2 := tongxunhan - my_tongxunhan
	checkok3 := math.Round(checkok2 / 10)
	success := "Thăng cấp level " + namelevel + " thành công Nhận " + formatFloat(checkok2) + " coins"
	print_json(api.J(
		"order", success,
	))

	play_sql.Query("UPDATE `users` SET `money` = `money` + '" + formatFloat(checkok2) + "' ,  `ck` = " + chietkhaugiam + " WHERE `id` = '" + id_users + "' ")
	play_sql.Query("UPDATE users4 SET `experience_user` = `experience_user` + " + formatFloat(checkok3) + "  WHERE `iduser` = '" + id_users + "'  ")
	play_sql.Query("UPDATE `users2` SET `level` =  '" + mucnhan + "' , `tongxunhan` = '" + formatFloat(tongxunhan) + "' WHERE `id_users` = '" + id_users + "' ")
}

func parsePlanPayload(c *gin.Context) map[string]string {
	if c == nil || c.Request == nil {
		return map[string]string{}
	}

	body, err := io.ReadAll(c.Request.Body)
	if err == nil && len(body) > 0 {
		raw := strings.TrimSpace(string(body))
		if raw != "" {
			var obj map[string]any
			if json.Unmarshal([]byte(raw), &obj) == nil && len(obj) > 0 {
				return mapFromAny(obj)
			}

			var rawString string
			if json.Unmarshal([]byte(raw), &rawString) == nil && strings.TrimSpace(rawString) != "" {
				return parseQueryString(rawString)
			}

			return parseQueryString(raw)
		}
	}

	if c.Request.URL != nil && c.Request.URL.RawQuery != "" {
		return parseQueryString(c.Request.URL.RawQuery)
	}

	return map[string]string{}
}

func parseQueryString(raw string) map[string]string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			out[key] = value[0]
		} else {
			out[key] = ""
		}
	}
	return out
}

func mapFromAny(values map[string]any) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = play_sql.ToString(value)
	}
	return out
}

func formatFloat(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
