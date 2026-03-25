package service

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

// GetApikeyIDsNeedMoneyCheck returns apikey ids with key_check_money = 1.
func GetApikeyIDsNeedMoneyCheck() ([]int64, error) {
	conn, err := database.Open()
	if err != nil {
		return nil, err
	}

	// Skip OpenAI rows for money_update flow.
	rows, err := conn.Query("SELECT `id` FROM `apikey` WHERE `key_check_money` = 1 AND LOWER(`website`) NOT LIKE '%openai%' ORDER BY `id` ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0, 16)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetApikeyWebsiteAndKey returns website, key_apikey and api_url for one apikey id.
func GetApikeyWebsiteAndKey(id int64) (string, string, string, error) {
	conn, err := database.Open()
	if err != nil {
		return "", "", "", err
	}

	var website sql.NullString
	var keyApikey sql.NullString
	var apiURL sql.NullString
	err = conn.QueryRow("SELECT `website`, `key_apikey`, `api_url` FROM `apikey` WHERE `id` = ? LIMIT 1", id).Scan(&website, &keyApikey, &apiURL)
	if err != nil {
		return "", "", "", err
	}

	return strings.TrimSpace(website.String), strings.TrimSpace(keyApikey.String), strings.TrimSpace(apiURL.String), nil
}

// UpdateApikeyMoney writes balance into key_money_ok; if column doesn't exist, fallback to money.
func UpdateApikeyMoney(id int64, balance string) (string, error) {
	conn, err := database.Open()
	if err != nil {
		return "", err
	}

	normalizedBalance, err := normalizeMoneyForSQL(balance)
	if err != nil {
		return "", err
	}

	_, err = conn.Exec("UPDATE `apikey` SET `key_money_ok` = ? WHERE `id` = ?", normalizedBalance, id)
	if err == nil {
		return "key_money_ok", nil
	}

	// Backward-compatible fallback for schema without key_money_ok column.
	if strings.Contains(strings.ToLower(err.Error()), "unknown column") && strings.Contains(strings.ToLower(err.Error()), "key_money_ok") {
		_, err2 := conn.Exec("UPDATE `apikey` SET `money` = ? WHERE `id` = ?", normalizedBalance, id)
		if err2 == nil {
			return "money", nil
		}
		return "", err2
	}

	return "", err
}

// normalizeMoneyForSQL rounds money to at most 3 decimal places before updating DB.
func normalizeMoneyForSQL(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("money empty")
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return "", fmt.Errorf("money invalid: %s", text)
	}

	rounded := math.Round(parsed*1000) / 1000
	out := strconv.FormatFloat(rounded, 'f', 3, 64)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	if out == "" || out == "-0" {
		out = "0"
	}
	return out, nil
}

// UpdateMoneyHandler returns ids in apikey table where key_check_money = 1.
func UpdateMoneyHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	ids, err := GetApikeyIDsNeedMoneyCheck()
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}

	peakerrIDs := make([]int64, 0, len(ids))
	bulkfollowsIDs := make([]int64, 0, len(ids))
	baostarIDs := make([]int64, 0, len(ids))
	viotpIDs := make([]int64, 0, len(ids))
	shopxulaohacIDs := make([]int64, 0, len(ids))
	mfbIDs := make([]int64, 0, len(ids))
	tuongtaccheoIDs := make([]int64, 0, len(ids))
	traodoisubIDs := make([]int64, 0, len(ids))
	vipigIDs := make([]int64, 0, len(ids))
	updatedIDs := make([]int64, 0, len(ids))
	failed := make([]map[string]any, 0)
	updatedColumns := map[string]int{}

	for _, id := range ids {
		website, keyApikey, apiURL, err := GetApikeyWebsiteAndKey(id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			failed = append(failed, map[string]any{
				"id":    id,
				"error": err.Error(),
			})
			continue
		}
		websiteLower := strings.ToLower(website)
		panelType := ""
		if strings.Contains(websiteLower, "peakerr") {
			panelType = "peakerr"
			peakerrIDs = append(peakerrIDs, id)
		} else if strings.Contains(websiteLower, "bulkfollows") {
			panelType = "bulkfollows"
			bulkfollowsIDs = append(bulkfollowsIDs, id)
		} else if strings.Contains(websiteLower, "baostar") {
			panelType = "baostar"
			baostarIDs = append(baostarIDs, id)
		} else if strings.Contains(websiteLower, "viotp") {
			panelType = "viotp"
			viotpIDs = append(viotpIDs, id)
		} else if strings.Contains(websiteLower, "shopxulaohac") {
			panelType = "shopxulaohac"
			shopxulaohacIDs = append(shopxulaohacIDs, id)
		} else if strings.Contains(websiteLower, "mfb.vn") || strings.Contains(websiteLower, "api.mfb.vn") {
			panelType = "mfb"
			mfbIDs = append(mfbIDs, id)
		} else if strings.Contains(websiteLower, "tuongtaccheo") {
			panelType = "tuongtaccheo"
			tuongtaccheoIDs = append(tuongtaccheoIDs, id)
		} else if strings.Contains(websiteLower, "traodoisub") {
			panelType = "traodoisub"
			traodoisubIDs = append(traodoisubIDs, id)
		} else if strings.Contains(websiteLower, "vipig") {
			panelType = "vipig"
			vipigIDs = append(vipigIDs, id)
		}
		if panelType == "" {
			continue
		}

		authValue := keyApikey
		if panelType == "tuongtaccheo" || panelType == "traodoisub" || panelType == "vipig" {
			authValue = apiURL // per requirement: key is stored in api_url column
			if strings.TrimSpace(extractTokenValue(authValue)) == "" && strings.TrimSpace(keyApikey) != "" {
				// Fallback only when api_url does not contain a usable token.
				authValue = keyApikey
			}
		}

		if strings.TrimSpace(authValue) == "" {
			failed = append(failed, map[string]any{
				"id":      id,
				"panel":   panelType,
				"error":   "empty_auth_value",
				"website": website,
			})
			continue
		}

		money := ""
		if panelType == "peakerr" {
			money, err = peakerr_money(authValue)
		} else if panelType == "bulkfollows" {
			money, err = bulkfollows_money(authValue)
		} else if panelType == "baostar" {
			money, err = baostar_money(apiURL, authValue)
		} else if panelType == "viotp" {
			money, err = viotp_money(apiURL, authValue)
		} else if panelType == "shopxulaohac" {
			money, err = shopxulaohac_money(apiURL, authValue)
		} else if panelType == "mfb" {
			money, err = mfb_money(authValue)
		} else if panelType == "tuongtaccheo" {
			money, err = tuongtaccheo_money(authValue)
		} else if panelType == "vipig" {
			money, err = vipig_money(authValue)
		} else {
			money, err = traodoisub_money(authValue)
		}
		if err != nil {
			failed = append(failed, map[string]any{
				"id":      id,
				"panel":   panelType,
				"website": website,
				"api_url": apiURL,
				"error":   err.Error(),
			})
			continue
		}
		if strings.TrimSpace(money) == "" {
			failed = append(failed, map[string]any{
				"id":      id,
				"panel":   panelType,
				"website": website,
				"api_url": apiURL,
				"error":   "money_empty",
			})
			continue
		}

		updatedCol, err := UpdateApikeyMoney(id, money)
		if err != nil {
			failed = append(failed, map[string]any{
				"id":      id,
				"panel":   panelType,
				"website": website,
				"api_url": apiURL,
				"error":   err.Error(),
				"money":   money,
			})
			continue
		}

		updatedColumns[updatedCol]++
		updatedIDs = append(updatedIDs, id)
	}

	api.Print_json(
		c,
		"status", "1",
		"message", "ok",
		"count", len(ids),
		"ids", ids,
		"peakerr_count", len(peakerrIDs),
		"peakerr_ids", peakerrIDs,
		"bulkfollows_count", len(bulkfollowsIDs),
		"bulkfollows_ids", bulkfollowsIDs,
		"baostar_count", len(baostarIDs),
		"baostar_ids", baostarIDs,
		"viotp_count", len(viotpIDs),
		"viotp_ids", viotpIDs,
		"shopxulaohac_count", len(shopxulaohacIDs),
		"shopxulaohac_ids", shopxulaohacIDs,
		"mfb_count", len(mfbIDs),
		"mfb_ids", mfbIDs,
		"tuongtaccheo_count", len(tuongtaccheoIDs),
		"tuongtaccheo_ids", tuongtaccheoIDs,
		"traodoisub_count", len(traodoisubIDs),
		"traodoisub_ids", traodoisubIDs,
		"vipig_count", len(vipigIDs),
		"vipig_ids", vipigIDs,
		"updated_count", len(updatedIDs),
		"updated_ids", updatedIDs,
		"updated_columns", updatedColumns,
		"failed_count", len(failed),
		"failed", failed,
	)
}
