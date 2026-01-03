package p2p_link

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"hust_backend/main/api"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

type getlinkRequest struct {
	national_market string
	has_national    bool
	apikey          string
	service_id      string
	tools           any
	code_item       string
	crypto          getlinkCrypto
	ip_address_v4   string
}

type getlinkCrypto struct {
	data string
	key  string
	iv   string
}

var (
	translations     map[string]map[string]string
	translationsOnce sync.Once
)

func GetlinkRunHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	print_json := api.MakePrintJSON(c)
	payload := parseGetlinkPayload(c)
	national_market := strings.TrimSpace(payload.national_market)
	if !payload.has_national {
		national_market = "vi"
	}
	if national_market == "" {
		national_market = "en"
	}
	lang := getLang(national_market)

	apikey := strings.TrimSpace(payload.apikey)
	service_id := strings.TrimSpace(payload.service_id)
	code_item := strings.TrimSpace(payload.code_item)
	ip_address_v4 := strings.TrimSpace(payload.ip_address_v4)
	tools := isTruthy(payload.tools)
	chedo := strings.TrimSpace(os.Getenv("CHEDO"))

	web3_service := ""
	web3_apikey := ""
	if chedo != "okluonhe" && !tools {
		decrypted, err := decryptCrypto(payload.crypto.data, payload.crypto.key, payload.crypto.iv)
		if err == nil && decrypted != "" {
			parts := strings.Split(decrypted, "|")
			if len(parts) > 0 {
				web3_service = parts[0]
			}
			if len(parts) > 1 {
				web3_apikey = parts[1]
			}
		}
	}

	id_users := toString(play_sql.Query("SELECT `id_users` FROM `users_key` WHERE `users_apikey` = '" + apikey + "' ").Fetch_array()["id_users"])
	my_username := toString(play_sql.Query("SELECT `username` FROM `users` WHERE `id` = '" + id_users + "'  ").Fetch_array()["username"])
	checkphone := toString(play_sql.Query("SELECT `checkgiaodich` FROM `check_phone_1` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["checkgiaodich"])
	phone := toString(play_sql.Query("SELECT `phone` FROM `check_phone_1` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["phone"])
	device_id := toString(play_sql.Query("SELECT `device_id` FROM `users_device_list` WHERE `iduser` = '" + id_users + "' ").Fetch_array()["device_id"])
	money := toFloat(toString(play_sql.Query("SELECT `money` FROM `users` WHERE `id` = '" + id_users + "'  ").Fetch_array()["money"]))

	if id_users == "" || apikey == "" || my_username == "" {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "login_required"),
		))
		return
	}

	if phone == "" || checkphone != "YES" || id_users == "" {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "tele_config_missing"),
		))
		return
	}

	if money < 0 {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "money_mission_minus"),
		))
		return
	}

	partner_main_row := play_sql.Query("SELECT * FROM `mission_partner_main` WHERE `id` = '" + service_id + "' ")
	partner_main_data := partner_main_row.Fetch_array()
	id_misson_main := toString(partner_main_data["id"])
	type_api := toString(partner_main_data["type_api"])
	api_category := toString(partner_main_data["api_category"])
	maxorder := toString(partner_main_data["maxorder"])
	minorder := toString(partner_main_data["minorder"])
	category := toString(partner_main_data["category"])
	partner_info := fetchPartnerInfo(id_misson_main)
	api_url := partner_info.api_url
	api_key := partner_info.api_key
	api_categorymini := partner_info.api_categorymini
	time := partner_info.time
	level_mission := partner_info.level

	if chedo == "okluonhe" {
		category = "urlshorten"
	}

	last_mission_row := play_sql.Query(
		"SELECT `id`, `mission_updatedate` FROM `misson_shorten_main` WHERE  `api_category` = '" + api_category + "' AND DATE(mission_updatedate) = CURRENT_DATE() AND `iduser` =  '" + id_users + "' AND `status` =  'Completed' ORDER BY id desc limit 1  ",
	).Fetch_array()
	last_mission_id := toString(last_mission_row["id"])
	createdate := toString(last_mission_row["mission_updatedate"])
	time_delay, ok := computeTimeDelaySeconds(createdate, toInt(time))
	if ok {
		if last_mission_id != "" {
			old_link := toString(play_sql.Query("SELECT `link` FROM `misson_shorten_link` WHERE `id_misson` =  '" + last_mission_id + "' ORDER BY id desc limit 1  ").Fetch_array()["link"])
			if old_link != "" {
				print_json(api.J(
					"status", "1",
					"message", langValue(lang, "try_now"),
					"link", old_link,
				))
				return
			}
		}
		print_json(api.J(
			"status", "0",
			"message", fmt.Sprintf(langValue(lang, "wait_time"), strconv.Itoa(time_delay)),
		))
		return
	}

	if api_url == "" || type_api == "" {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "service_invalid"),
		))
		return
	}

	if !tools && chedo != "okluonhe" && (web3_service != service_id || web3_apikey != apikey) {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "crypto_code_invalid"),
		))
		return
	}

	maxorder_int := toInt(maxorder)
	minorder_int := toInt(minorder)
	maxorder_query := strings.TrimSpace(maxorder)
	if maxorder_query == "" {
		maxorder_query = strconv.Itoa(maxorder_int)
	}
	var views_count int
	if minorder_int == 0 {
		views_count = toInt(toString(play_sql.Query("SELECT COUNT(*) FROM `misson_shorten_main` WHERE `api_category` = '" + api_category + "' AND DATE(mission_updatedate) >= DATE_SUB(CURRENT_DATE(), INTERVAL " + maxorder_query + " DAY) AND `iduser` =  '" + id_users + "'  AND `status` = 'Completed'").Fetch_array()["COUNT(*)"]))
	} else {
		views_count = toInt(toString(play_sql.Query("SELECT COUNT(*) FROM `misson_shorten_main` WHERE  `iduser` = '" + id_users + "' AND `status` = 'Completed' AND `api_category` = '" + api_category + "'    AND DATE(mission_updatedate) = CURRENT_DATE()   ").Fetch_array()["COUNT(*)"]))
	}

	level_users := toInt(toString(play_sql.Query("SELECT `level` FROM `users2` WHERE `id_users` = '" + id_users + "' ").Fetch_array()["level"]))
	level_mission_int := toInt(level_mission)
	namelevel := toString(play_sql.Query("SELECT `namelevel` FROM `capdotaikhoan` WHERE `id` =  '" + level_mission + "'  ").Fetch_array()["namelevel"])

	if level_users < level_mission_int {
		print_json(api.J(
			"status", "0",
			"message", fmt.Sprintf(langValue(lang, "level_insufficient"), namelevel),
		))
		return
	}

	if chedo != "okluonhe" && !tools && strings.Contains(type_api, "only_app") && device_id == "" {
		print_json(api.J(
			"status", "0",
			"message", langValue(lang, "system_error"),
		))
		return
	}

	if chedo != "okluonhe" && !tools && views_count >= maxorder_int && minorder_int != 0 {
		print_json(api.J(
			"status", "0",
			"message", fmt.Sprintf(langValue(lang, "daily_limit_reached"), strconv.Itoa(maxorder_int)),
		))
		return
	}

	if chedo != "okluonhe" && !tools && views_count > 0 && minorder_int == 0 {
		print_json(api.J(
			"status", "0",
			"message", fmt.Sprintf(langValue(lang, "periodic_limit_reached"), strconv.Itoa(maxorder_int)),
		))
		return
	}

	id_misson_xuly := toString(play_sql.Query("SELECT `id` FROM `misson_shorten_main` WHERE  `api_category` = '" + api_category + "' AND DATE(mission_updatedate) = CURRENT_DATE() AND `category_code` =  '" + category + "' AND `iduser` =  '" + id_users + "' AND `status` = 'Processing'  ORDER BY id desc limit 1  ").Fetch_array()["id"])
	xuly_misson_shorten_link := ""
	if id_misson_xuly != "" {
		xuly_misson_shorten_link = toString(play_sql.Query("SELECT `link` FROM `misson_shorten_link` WHERE  DATE(mission_createdate) = CURRENT_DATE() AND `id_misson` =  '" + id_misson_xuly + "' ORDER BY id desc limit 1  ").Fetch_array()["link"])
	}
	if xuly_misson_shorten_link != "" {
		print_json(api.J(
			"status", "1",
			"message", langValue(lang, "try_now"),
			"link", xuly_misson_shorten_link,
		))
		return
	}

	ip_address_php := getClientIP(c)
	result := getlinkProcess(getlinkProcessInput{
		lang:             lang,
		api_url:          api_url,
		api_key:          api_key,
		type_api:         type_api,
		api_categorymini: api_categorymini,
		api_category:     api_category,
		category:         category,
		id_users:         id_users,
		id_misson_main:   id_misson_main,
		service_id:       service_id,
		code_item:        code_item,
		phone:            phone,
		device_id:        device_id,
		ip_address_php:   ip_address_php,
		ip_address_v4:    ip_address_v4,
		chedo:            chedo,
	})

	print_json(result)
}

func parseGetlinkPayload(c *gin.Context) getlinkRequest {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		return getlinkRequest{}
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return getlinkRequest{}
	}

	req := getlinkRequest{
		national_market: toString(raw["national_market"]),
		has_national:    hasKey(raw, "national_market"),
		apikey:          toString(raw["apikey"]),
		service_id:      toString(raw["service"]),
		tools:           raw["tools"],
		code_item:       toString(raw["code_item"]),
		ip_address_v4:   toString(raw["ip_address_v4"]),
	}
	if cryptoRaw, ok := raw["crypto"].(map[string]any); ok {
		req.crypto.data = toString(cryptoRaw["data"])
		req.crypto.key = toString(cryptoRaw["key"])
		req.crypto.iv = toString(cryptoRaw["iv"])
	}
	return req
}

func hasKey(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	_, ok := values[key]
	return ok
}

func loadTranslations() map[string]map[string]string {
	translationsOnce.Do(func() {
		baseDir, err := os.Getwd()
		if err != nil {
			baseDir = "."
		}
		path := filepath.Join(baseDir, "p2p", "mission", "p2p_link", "mision_main.json")
		content, err := os.ReadFile(path)
		if err != nil {
			translations = map[string]map[string]string{}
			return
		}
		var data map[string]map[string]string
		if err := json.Unmarshal(content, &data); err != nil {
			translations = map[string]map[string]string{}
			return
		}
		translations = data
	})
	return translations
}

func getLang(market string) map[string]string {
	data := loadTranslations()
	if lang, ok := data[market]; ok {
		return lang
	}
	if lang, ok := data["en"]; ok {
		return lang
	}
	return map[string]string{}
}

func langValue(lang map[string]string, key string) string {
	if lang == nil {
		return ""
	}
	if value, ok := lang[key]; ok {
		return value
	}
	return ""
}

func getClientIP(c *gin.Context) string {
	headers := []string{
		"HTTP_CLIENT_IP",
		"HTTP_X_FORWARDED_FOR",
		"HTTP_X_FORWARDED",
		"HTTP_FORWARDED_FOR",
		"HTTP_FORWARDED",
		"X-Forwarded-For",
		"X-Real-IP",
	}
	for _, header := range headers {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			if strings.Contains(value, ",") {
				value = strings.TrimSpace(strings.Split(value, ",")[0])
			}
			return value
		}
	}
	remoteAddr := strings.TrimSpace(c.Request.RemoteAddr)
	if remoteAddr == "" {
		return "UNKNOWN"
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	return remoteAddr
}

func isTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		val := strings.TrimSpace(strings.ToLower(v))
		return val != "" && val != "0" && val != "false"
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	default:
		return true
	}
}
