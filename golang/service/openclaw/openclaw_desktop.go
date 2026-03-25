package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hust_backend/main/api"

	"github.com/gin-gonic/gin"
)

const defaultOpenClawDesktopScriptDir = `c:\hustmedia6\backend\golang\service\openclaw`

func OpenClawDesktopStatusHandler(c *gin.Context) {
	handleOpenClawDesktopScript(c, "desktop_status.ps1", nil)
}

func OpenClawDesktopListWindowsHandler(c *gin.Context) {
	title := openClawReadString(c, "title")
	limit := openClawReadInt(c, 50, "limit")
	if limit <= 0 {
		limit = 50
	}
	handleOpenClawDesktopScript(c, "desktop_list_windows.ps1", []string{
		"-Title", title,
		"-Limit", strconv.Itoa(limit),
	})
}

func OpenClawDesktopActivateWindowHandler(c *gin.Context) {
	title := openClawReadString(c, "title")
	processName := openClawReadString(c, "process_name", "processName")
	exact := openClawReadBool(c, false, "exact")
	if title == "" && processName == "" {
		api.Print_json(
			c,
			"status", "0",
			"message", "missing title or process_name",
			http.StatusBadRequest,
		)
		return
	}

	handleOpenClawDesktopScript(c, "desktop_activate_window.ps1", []string{
		"-Title", title,
		"-ProcessName", processName,
		"-Exact", openClawPowerShellBool(exact),
	})
}

func OpenClawDesktopClickHandler(c *gin.Context) {
	x, okX := openClawReadRequiredInt(c, "x")
	y, okY := openClawReadRequiredInt(c, "y")
	if !okX || !okY {
		api.Print_json(
			c,
			"status", "0",
			"message", "missing x or y",
			http.StatusBadRequest,
		)
		return
	}

	button := openClawReadString(c, "button")
	if button == "" {
		button = "left"
	}
	doubleClick := openClawReadBool(c, false, "double", "double_click", "doubleClick")

	handleOpenClawDesktopScript(c, "desktop_click.ps1", []string{
		"-X", strconv.Itoa(x),
		"-Y", strconv.Itoa(y),
		"-Button", button,
		"-DoubleClick", openClawPowerShellBool(doubleClick),
	})
}

func OpenClawDesktopSendKeysHandler(c *gin.Context) {
	keys := openClawReadString(c, "keys")
	if keys == "" {
		api.Print_json(
			c,
			"status", "0",
			"message", "missing keys",
			http.StatusBadRequest,
		)
		return
	}

	title := openClawReadString(c, "title")
	processName := openClawReadString(c, "process_name", "processName")
	exact := openClawReadBool(c, false, "exact")
	delayMs := openClawReadInt(c, 250, "delay_ms", "delayMs")
	if delayMs < 0 {
		delayMs = 0
	}

	handleOpenClawDesktopScript(c, "desktop_send_keys.ps1", []string{
		"-Keys", keys,
		"-Title", title,
		"-ProcessName", processName,
		"-Exact", openClawPowerShellBool(exact),
		"-DelayMs", strconv.Itoa(delayMs),
	})
}

func OpenClawDesktopRunAhkHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	remoteHost, loopback := extractLoopbackRemoteHost(c.Request.RemoteAddr)
	if !loopback {
		api.Print_json(
			c,
			"status", "0",
			"message", "forbidden: loopback only",
			"remote_addr", c.Request.RemoteAddr,
			http.StatusForbidden,
		)
		return
	}

	ahkPath := findAutoHotkeyExecutable()
	if ahkPath == "" {
		api.Print_json(
			c,
			"status", "0",
			"message", "AutoHotkey not found",
			"paths_checked", []string{
				`C:\Program Files\AutoHotkey\AutoHotkey.exe`,
				`C:\Program Files\AutoHotkey\v2\AutoHotkey64.exe`,
				`C:\Program Files\AutoHotkey\v2\AutoHotkey.exe`,
				`C:\Program Files\AutoHotkey\UX\AutoHotkeyUX.exe`,
			},
			http.StatusNotFound,
		)
		return
	}

	timeoutSec := openClawReadInt(c, 15, "timeout_sec", "timeoutSec")
	if timeoutSec <= 0 {
		timeoutSec = 15
	}

	scriptPath := strings.TrimSpace(openClawReadString(c, "script_path", "scriptPath"))
	script := openClawReadString(c, "script")
	if scriptPath == "" && script == "" {
		api.Print_json(
			c,
			"status", "0",
			"message", "missing script_path or script",
			http.StatusBadRequest,
		)
		return
	}

	tempScript := ""
	if scriptPath == "" {
		tmpFile, err := os.CreateTemp("", "openclaw-desktop-*.ahk")
		if err != nil {
			api.Print_json(c, "status", "0", "message", "create temp ahk failed", "error", err.Error(), http.StatusInternalServerError)
			return
		}
		tempScript = tmpFile.Name()
		if _, err := tmpFile.WriteString(script); err != nil {
			tmpFile.Close()
			api.Print_json(c, "status", "0", "message", "write temp ahk failed", "error", err.Error(), http.StatusInternalServerError)
			return
		}
		tmpFile.Close()
		scriptPath = tempScript
		defer os.Remove(tempScript)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ahkPath, scriptPath)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		api.Print_json(c, "status", "0", "message", "autohotkey timeout", "remote_host", remoteHost, "autohotkey", ahkPath, "output", result, http.StatusGatewayTimeout)
		return
	}
	if err != nil {
		api.Print_json(c, "status", "0", "message", "autohotkey failed", "remote_host", remoteHost, "autohotkey", ahkPath, "script_path", scriptPath, "output", result, "error", err.Error(), http.StatusInternalServerError)
		return
	}

	api.Print_json(c,
		"status", "1",
		"message", "ok",
		"remote_host", remoteHost,
		"autohotkey", ahkPath,
		"script_path", scriptPath,
		"output", result,
	)
}

func handleOpenClawDesktopScript(c *gin.Context, scriptName string, scriptArgs []string) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	remoteHost, loopback := extractLoopbackRemoteHost(c.Request.RemoteAddr)
	if !loopback {
		api.Print_json(
			c,
			"status", "0",
			"message", "forbidden: loopback only",
			"remote_addr", c.Request.RemoteAddr,
			http.StatusForbidden,
		)
		return
	}

	scriptPath, err := resolveOpenClawDesktopScript(scriptName)
	if err != nil {
		api.Print_json(c, "status", "0", "message", "script not found", "script", scriptName, "error", err.Error(), http.StatusInternalServerError)
		return
	}

	timeoutSec := openClawReadInt(c, 15, "timeout_sec", "timeoutSec")
	if timeoutSec <= 0 {
		timeoutSec = 15
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := append([]string{
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	}, scriptArgs...)

	cmd := exec.CommandContext(ctx, "powershell.exe", args...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		api.Print_json(c, "status", "0", "message", "powershell timeout", "script", scriptName, "remote_host", remoteHost, "output", result, http.StatusGatewayTimeout)
		return
	}
	if err != nil {
		api.Print_json(c, "status", "0", "message", "powershell failed", "script", scriptName, "remote_host", remoteHost, "output", result, "error", err.Error(), http.StatusInternalServerError)
		return
	}

	if openClawLooksLikeJSON(result) {
		var payload any
		if err := json.Unmarshal([]byte(result), &payload); err == nil {
			api.Print_json(c, payload)
			return
		}
	}

	api.Print_json(c,
		"status", "1",
		"message", "ok",
		"script", scriptName,
		"remote_host", remoteHost,
		"output", result,
	)
}

func resolveOpenClawDesktopScript(scriptName string) (string, error) {
	envDir := strings.TrimSpace(os.Getenv("OPENCLAW_DESKTOP_SCRIPT_DIR"))
	candidates := []string{}
	if envDir != "" {
		candidates = append(candidates, filepath.Join(envDir, scriptName))
	}

	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates,
			filepath.Join(cwd, "service", "openclaw", scriptName),
			filepath.Join(cwd, "backend", "golang", "service", "openclaw", scriptName),
		)
	}

	candidates = append(candidates, filepath.Join(defaultOpenClawDesktopScriptDir, scriptName))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("script %s not found", scriptName)
}

func openClawReadString(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
		if value := strings.TrimSpace(c.PostForm(key)); value != "" {
			return value
		}
		if bodyValue, ok := openClawBodyValue(c, key); ok {
			if value := strings.TrimSpace(fmt.Sprint(bodyValue)); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func openClawReadInt(c *gin.Context, fallback int, keys ...string) int {
	for _, key := range keys {
		raw := openClawReadString(c, key)
		if raw == "" {
			continue
		}
		if value, err := strconv.Atoi(raw); err == nil {
			return value
		}
	}
	return fallback
}

func openClawReadRequiredInt(c *gin.Context, keys ...string) (int, bool) {
	for _, key := range keys {
		raw := openClawReadString(c, key)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err == nil {
			return value, true
		}
	}
	return 0, false
}

func openClawReadBool(c *gin.Context, fallback bool, keys ...string) bool {
	for _, key := range keys {
		raw := strings.ToLower(strings.TrimSpace(openClawReadString(c, key)))
		switch raw {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func openClawPowerShellBool(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func openClawBodyValue(c *gin.Context, key string) (any, bool) {
	bodyMap := openClawReadBodyMap(c)
	value, ok := bodyMap[key]
	return value, ok
}

func openClawReadBodyMap(c *gin.Context) map[string]any {
	if cached, ok := c.Get("_openclaw_body_map"); ok {
		if payload, ok := cached.(map[string]any); ok {
			return payload
		}
	}

	payload := map[string]any{}
	if c.Request == nil || c.Request.Body == nil {
		c.Set("_openclaw_body_map", payload)
		return payload
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &payload)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	} else {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	}
	c.Set("_openclaw_body_map", payload)
	return payload
}

func openClawLooksLikeJSON(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[")
}

func findAutoHotkeyExecutable() string {
	candidates := []string{
		`C:\Program Files\AutoHotkey\AutoHotkey.exe`,
		`C:\Program Files\AutoHotkey\v2\AutoHotkey64.exe`,
		`C:\Program Files\AutoHotkey\v2\AutoHotkey.exe`,
		`C:\Program Files\AutoHotkey\UX\AutoHotkeyUX.exe`,
	}
	if path, err := exec.LookPath("AutoHotkey.exe"); err == nil {
		return path
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
