package p2p_link

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"hust_backend/main/play_sql"
)

const requestTimeout time.Duration = 0

type getlinkProcessInput struct {
	lang             map[string]string
	api_url          string
	api_key          string
	type_api         string
	api_categorymini string
	api_category     string
	category         string
	id_users         string
	id_misson_main   string
	service_id       string
	code_item        string
	phone            string
	device_id        string
	ip_address_php   string
	ip_address_v4    string
	chedo            string
}

func getlinkProcess(input getlinkProcessInput) map[string]any {
	lang := input.lang
	if strings.Contains(input.type_api, "offerwall") {
		return map[string]any{
			"status":  "1",
			"message": langValue(lang, "try_now"),
			"link":    input.api_url,
		}
	}

	play_sql.Query("INSERT INTO `misson_shorten_main` SET `api_category` = '" + input.api_category + "', `iduser` = '" + input.id_users + "', `category_code` = '" + input.category + "', `reload` = '0', `status` = 'Processing', `ipaddress` = '" + input.ip_address_php + "', `ip_address_v4` = '" + input.ip_address_v4 + "', `mission_updatedate` = now() ")
	id_misson := toString(play_sql.Query("SELECT `id` FROM `misson_shorten_main` ORDER BY id desc limit 1 ").Fetch_array()["id"])

	textToEncrypt := input.id_users + "|" + input.api_category + "|" + id_misson
	if input.chedo == "okluonhe" {
		textToEncrypt = input.id_users + "|" + input.api_category + "|" + id_misson + "|" + input.code_item
	}
	encryptedMessage := encryptLinkCode(textToEncrypt)
	link_done := "https://tecom.pro/jop/mission/thankskiu.php?code=" + encryptedMessage

	alias := randomStringAlias()
	api_get_link := strings.NewReplacer(
		"{api_key}", input.api_key,
		"{url}", link_done,
		"{alias}", alias,
		"{id_misson_main}", input.id_misson_main,
		"{service_id}", input.service_id,
	).Replace(input.api_url)

	if strings.Contains(input.type_api, "total_count") {
		re := regexp.MustCompile(`\d+`)
		min_orders := toInt(re.FindString(input.type_api))
		total_count := 0
		if strings.Contains(input.api_category, "text_speech") {
			total_count = toInt(toString(play_sql.Query("SELECT SUM(amount) AS total_amount FROM `orders_ai` WHERE DATE(createdate) = CURRENT_DATE() AND `category` = 'text_speech' AND `id_users` = '" + input.id_users + "'").Fetch_array()["total_amount"]))
		} else if strings.Contains(input.api_category, "dice_love") {
			total_count = toInt(toString(play_sql.Query("SELECT COUNT(*) AS total_count FROM `misson_shorten_main` WHERE DATE(mission_updatedate) = CURRENT_DATE() AND `status` = 'Completed' AND `iduser` = '" + input.id_users + "'").Fetch_array()["total_count"]))
		} else if strings.Contains(input.api_category, "tiktok_pro_max") {
			follow_count := toInt(toString(play_sql.Query("SELECT COUNT(*) AS total_count FROM `tiktok_follow_users` WHERE DATE(createdate) = CURRENT_DATE() AND `iduser` = '" + input.id_users + "'").Fetch_array()["total_count"]))
			heart_count := toInt(toString(play_sql.Query("SELECT COUNT(*) AS total_count FROM `tiktok_video_users` WHERE DATE(createdate) = CURRENT_DATE()  AND `iduser` = '" + input.id_users + "'").Fetch_array()["total_count"]))
			total_count = follow_count + heart_count
		} else if strings.Contains(input.api_category, "hust.media/media") {
			total_count = toInt(toString(play_sql.Query("SELECT COUNT(*) AS total_count FROM `cash_approval` WHERE DATE(createdate) = CURRENT_DATE() AND `category_code` = 'videos_maker' AND `iduser` = '" + input.id_users + "'").Fetch_array()["total_count"]))
		}
		if total_count < min_orders {
			return map[string]any{
				"status":  "0",
				"message": replaceFirst(langValue(lang, "insufficient_condition"), "%s", strconv.Itoa(total_count)),
			}
		}
	}

	var link string
	var message string

	if strings.Contains(input.type_api, "rewarded_ads") {
		link = "https://tecom.pro/jop/mission/thankskiu.php" + "?webappmode=open_rewarded_ads" + "&redirect_hust=" + link_done
		if input.device_id == "" {
			message = langValue(lang, "app_required")
		}
	} else if strings.Contains(input.type_api, "standard") {
		response := ""
		redirect_url := ""
		if strings.Contains(input.api_category, "trafficuser") {
			response, _ = postJSON("http://vip.tecom.pro:8787/trafficuser", map[string]any{"value": api_get_link}, requestTimeout)
		} else if strings.Contains(input.api_category, "exalink") {
			response, _ = postJSON("http://vip.tecom.pro:2999/exalink", map[string]any{"url": api_get_link}, requestTimeout)
		} else {
			response, redirect_url, _ = getWithRedirect(api_get_link, requestTimeout)
		}

		result := parseJSON(response)

		if strings.Contains(input.api_category, "traffic-user") {
			msg := toString(result["message"])
			if strings.Contains(msg, "https://") {
				link = msg
			} else {
				message = msg
			}
		} else if strings.Contains(input.api_category, "8link") {
			if toString(result["shortened_key"]) == "" || toString(result["shortened_url"]) == "" {
				message = langValue(lang, "link_unavailable")
			} else {
				link = toString(result["shortened_url"])
			}
		} else if strings.Contains(input.api_category, "earn-money") || strings.Contains(input.api_category, "shortlink24h.click") {
			if toString(result["errors"]) != "" {
				message = toString(result["message"])
			} else {
				link = toString(result["shortlink"])
			}
		} else if strings.Contains(input.api_category, "bbmkts") {
			if toString(result["status"]) == "error" {
				message = toString(result["message"])
			} else {
				link = redirect_url
			}
		} else if strings.Contains(input.api_category, "teckurl") || strings.Contains(input.api_category, "filesmoney") {
			if toString(result["errors"]) != "" {
				message = toString(result["errors"])
			} else {
				link = toString(result["url"])
			}
		} else if strings.Contains(input.api_category, "trafficuserr.com") {
			msg := toString(result["message"])
			if strings.Contains(msg, "https://") {
				link = msg
			} else {
				message = msg
			}
		} else if strings.Contains(input.api_category, "dilink") {
			if toString(result["status"]) != "" {
				message = toString(result["status"])
			} else {
				link = toString(result["url"])
			}
		} else if strings.Contains(input.api_category, "hust.media/ads_max") {
			if toString(result["status"]) == "error" {
				message = toString(result["message"])
			} else {
				link = toString(result["shortenedUrl"])
				api_get_link = "https://api.1short.io/public/links?token=8pvfPpn6tMlCyPcNkpuudWpkQnVFQOdk&url=" + link + "&alias=" + alias + "&method_level=level_1_plus"
				response, _, _ = getWithRedirect(api_get_link, requestTimeout)
				result = parseJSON(response)
				if toString(result["status"]) == "error" {
					message = toString(result["message"])
				} else {
					link = toString(result["shortenedUrl"])
				}
			}
		} else {
			if toString(result["status"]) == "error" {
				message = toString(result["message"])
			} else {
				link = toString(result["shortenedUrl"])
			}
		}
	} else if strings.Contains(input.type_api, "redirect") {
		_, redirect_url, err := getWithRedirect(api_get_link, requestTimeout)
		if err != nil {
			message = langValue(lang, "curl_error")
		} else if redirect_url != "" {
			link = redirect_url
		} else {
			message = langValue(lang, "no_redirect")
		}
	}

	if message != "" {
		return map[string]any{
			"status":  "0",
			"message": message,
		}
	}
	if link == "" {
		return map[string]any{
			"status":  "0",
			"message": langValue(lang, "partner_error"),
		}
	}

	device_id := toString(play_sql.Query("SELECT `device_id` FROM `users_device_list` WHERE `iduser` = '" + input.id_users + "' ").Fetch_array()["device_id"])
	play_sql.Query("INSERT INTO `misson_shorten_link` SET `id_misson` = '" + id_misson + "', `link` = '" + link + "', `iduser` = '" + input.id_users + "', `api_category` = '" + input.api_category + "', `mission_createdate` = now(), `status` = 'Processing', `ip_address` = '" + input.ip_address_php + "', `device_id` = '" + device_id + "'")

	if input.api_categorymini == "2-step" {
		tele_code := randomString(29)
		play_sql.Query("INSERT INTO `misson_shorten_code` SET `tele_code` = '" + tele_code + "', `status` = 'Processing', `phone` = '" + input.phone + "', `id_misson` = '" + id_misson + "', `api_category` = '" + input.api_category + "', `iduser` = '" + input.id_users + "', `createdate` = now() ")
	}

	return map[string]any{
		"status":  "1",
		"message": langValue(lang, "try_now"),
		"link":    link,
	}
}

func decryptCrypto(dataHex, keyHex, ivHex string) (string, error) {
	data, err := hex.DecodeString(strings.TrimSpace(dataHex))
	if err != nil {
		return "", err
	}
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(strings.TrimSpace(ivHex))
	if err != nil {
		return "", err
	}

	key = normalizeKey(key, 32)
	iv = normalizeKey(iv, aes.BlockSize)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(data)%aes.BlockSize != 0 {
		return "", errors.New("invalid block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(data))
	mode.CryptBlocks(plaintext, data)
	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func encryptLinkCode(text string) string {
	secret := "NAuNyvtSmaADALVKMorI3yVHKpowxr29"
	key := normalizeKey([]byte(secret), 16)
	iv := make([]byte, aes.BlockSize)

	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	plain := pkcs7Pad([]byte(text), aes.BlockSize)
	encrypted := make([]byte, len(plain))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted, plain)

	first := base64.StdEncoding.EncodeToString(encrypted)
	return base64.StdEncoding.EncodeToString([]byte(first))
}

func normalizeKey(value []byte, size int) []byte {
	if len(value) == size {
		return value
	}
	if len(value) > size {
		return value[:size]
	}
	out := make([]byte, size)
	copy(out, value)
	return out
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	if padding == 0 {
		padding = blockSize
	}
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, pad...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < padding; i++ {
		if data[len(data)-1-i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

func randomString(length int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return randomFromCharset(chars, length)
}

func randomStringAlias() string {
	return randomString(14)
}

func randomFromCharset(chars string, length int) string {
	if length <= 0 {
		return ""
	}
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return builder.String()
		}
		builder.WriteByte(chars[n.Int64()])
	}
	return builder.String()
}

func postJSON(url string, payload any, timeout time.Duration) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

func getWithRedirect(url string, timeout time.Duration) (string, string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return string(respBody), finalURL, nil
}

func parseJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func replaceFirst(value, old, new string) string {
	if old == "" {
		return value
	}
	index := strings.Index(value, old)
	if index == -1 {
		return value
	}
	return value[:index] + new + value[index+len(old):]
}
