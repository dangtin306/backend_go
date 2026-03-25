package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const peakerrAPIURL = "https://peakerr.com/api/v2"
const bulkfollowsAPIURL = "https://bulkfollows.com/api/v2"
const baostarAPIURL = "https://api.baostar.pro/api/v2"
const viotpBalanceAPIURL = "https://api.viotp.com/users/balance"
const shopxulaohacProfileAPIURL = "https://shopxulaohac.vn/api/profile.php"
const tuongtaccheoPanelAPIURL = "https://tuongtaccheo.com/api/v2"
const traodoisubProfileAPIURL = "https://traodoisub.com/api/"
const mfbPanelAPIURL = "https://api.mfb.vn/v2"

// peakerr_money gets account balance from Peakerr using action=balance.
func peakerr_money(apiKey string) (string, error) {
	return panel_money(peakerrAPIURL, apiKey)
}

// bulkfollows_money gets account balance from BulkFollows using action=balance.
func bulkfollows_money(apiKey string) (string, error) {
	return panel_money(bulkfollowsAPIURL, apiKey)
}

// baostar_money gets account balance from Baostar using action=balance.
// apiURL comes from apikey.api_url; when empty it falls back to the default baostar URL.
func baostar_money(apiURL string, apiKey string) (string, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = baostarAPIURL
	}
	return panel_money(apiURL, apiKey)
}

// viotp_money gets account balance from ViOTP using:
// GET /users/balance?token=<token>
func viotp_money(apiURL string, token string) (string, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = viotpBalanceAPIURL
	} else {
		parsed, err := url.Parse(apiURL)
		if err == nil && !strings.Contains(strings.ToLower(parsed.Path), "/users/balance") {
			parsed.Path = "/users/balance"
			parsed.RawQuery = ""
			apiURL = parsed.String()
		}
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("token", strings.TrimSpace(token))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("viotp status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	statusCode := strings.TrimSpace(fmt.Sprint(payload["status_code"]))
	success := strings.TrimSpace(fmt.Sprint(payload["success"]))
	if statusCode != "" && statusCode != "200" {
		return "", fmt.Errorf("viotp status_code=%s message=%s", statusCode, strings.TrimSpace(fmt.Sprint(payload["message"])))
	}
	if success != "" && strings.ToLower(success) != "true" {
		return "", fmt.Errorf("viotp unsuccessful message=%s", strings.TrimSpace(fmt.Sprint(payload["message"])))
	}

	// ViOTP docs: data.balance
	if data, ok := payload["data"].(map[string]any); ok {
		money := strings.TrimSpace(fmt.Sprint(data["balance"]))
		if money != "" && money != "<nil>" {
			return money, nil
		}
	}

	// Fallback in case response format differs.
	money := strings.TrimSpace(fmt.Sprint(payload["balance"]))
	if money == "" || money == "<nil>" {
		return "", fmt.Errorf("viotp balance missing")
	}
	return money, nil
}

// shopxulaohac_money gets account balance from Shopxulaohac using:
// GET /api/profile.php?api_key=<api_key>
func shopxulaohac_money(apiURL string, apiKey string) (string, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = shopxulaohacProfileAPIURL
	} else {
		parsed, err := url.Parse(apiURL)
		if err == nil && !strings.Contains(strings.ToLower(parsed.Path), "/api/profile.php") {
			parsed.Path = "/api/profile.php"
			parsed.RawQuery = ""
			apiURL = parsed.String()
		}
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("api_key", strings.TrimSpace(apiKey))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("shopxulaohac status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	money := extractMoneyField(payload)
	if money == "" {
		return "", fmt.Errorf("shopxulaohac balance missing")
	}
	return money, nil
}

// tuongtaccheo_money gets account balance from TuongTacCheo.
// keyFromApiURL is read from apikey.api_url as requested.
// Uses POST /api/v2 with action=balance,key=<key>.
func tuongtaccheo_money(keyFromApiURL string) (string, error) {
	key := extractTokenValue(keyFromApiURL)
	if key == "" {
		return "", fmt.Errorf("tuongtaccheo key missing in api_url")
	}
	return panel_money(tuongtaccheoPanelAPIURL, key)
}

// traodoisub_money gets account xu from Traodoisub:
// GET /api/?fields=profile&access_token=<token>
// token is read from apikey.api_url.
func traodoisub_money(keyFromApiURL string) (string, error) {
	token := extractTokenValue(keyFromApiURL)
	if token == "" {
		return "", fmt.Errorf("traodoisub token missing in api_url")
	}

	u, err := url.Parse(traodoisubProfileAPIURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("fields", "profile")
	q.Set("access_token", strings.TrimSpace(token))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("traodoisub status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	success := strings.TrimSpace(fmt.Sprint(payload["success"]))
	if success != "" && success != "200" {
		return "", fmt.Errorf("traodoisub success=%s", success)
	}

	if data, ok := payload["data"].(map[string]any); ok {
		xu := strings.TrimSpace(fmt.Sprint(data["xu"]))
		if xu != "" && xu != "<nil>" {
			return xu, nil
		}
	}

	return "", fmt.Errorf("traodoisub xu missing")
}

// mfb_money gets account balance from MFB panel API using action=balance.
// API key is taken from key_apikey column.
func mfb_money(apiKey string) (string, error) {
	return panel_money(mfbPanelAPIURL, apiKey)
}

func extractTokenValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Token often appears as 32-char lowercase/hex-like string.
	re := regexp.MustCompile(`(?i)\b[a-z0-9]{32}\b`)
	if token := strings.TrimSpace(re.FindString(raw)); token != "" {
		return token
	}

	if !strings.Contains(raw, "://") {
		return raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for _, key := range []string{"access_token", "api_key", "key", "token"} {
		val := strings.TrimSpace(q.Get(key))
		if val != "" {
			return val
		}
	}
	return ""
}

func extractMoneyField(values map[string]any) string {
	if values == nil {
		return ""
	}

	// Common field names used by panel/account APIs.
	keys := []string{"money", "balance", "so_du", "sodu", "coin", "amount", "fund", "wallet"}
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(raw))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}

	// Many APIs return nested data object.
	if nested, ok := values["data"].(map[string]any); ok {
		if money := extractMoneyField(nested); money != "" {
			return money
		}
	}
	if nested, ok := values["user"].(map[string]any); ok {
		if money := extractMoneyField(nested); money != "" {
			return money
		}
	}

	return ""
}

func panel_money(apiURL string, apiKey string) (string, error) {
	form := url.Values{}
	form.Set("key", strings.TrimSpace(apiKey))
	form.Set("action", "balance")

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("panel status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	if rawErr, ok := payload["error"]; ok {
		errText := strings.TrimSpace(fmt.Sprint(rawErr))
		if errText != "" && errText != "<nil>" {
			return "", fmt.Errorf("panel error: %s", errText)
		}
	}

	money := strings.TrimSpace(fmt.Sprint(payload["balance"]))
	if money == "" || money == "<nil>" {
		return "", fmt.Errorf("panel balance missing")
	}

	return money, nil
}
