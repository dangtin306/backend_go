package openclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"hust_backend/main/api"

	"github.com/gin-gonic/gin"
)

const defaultOpenClawSourceDir = `D:\hustmedia\application\openclaw-src`

type openClawDevicesList struct {
	Pending []openClawPendingDevice `json:"pending"`
}

type openClawPendingDevice struct {
	RequestID string `json:"requestId"`
	DeviceID  string `json:"deviceId"`
	RemoteIP  string `json:"remoteIp"`
	Platform  string `json:"platform"`
	Role      string `json:"role"`
	TS        int64  `json:"ts"`
}

func OpenClawApprovePairingHandler(c *gin.Context) {
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

	sourceDir := strings.TrimSpace(os.Getenv("OPENCLAW_SRC_DIR"))
	if sourceDir == "" {
		sourceDir = defaultOpenClawSourceDir
	}

	requestID := strings.TrimSpace(c.Query("request_id"))
	deviceID := strings.TrimSpace(c.Query("device_id"))
	remoteIP := normalizeRemoteLookupValue(firstNonEmpty(
		c.Query("remote_ip"),
		c.GetHeader("X-OpenClaw-Remote-IP"),
		c.GetHeader("CF-Connecting-IP"),
		c.GetHeader("X-Real-IP"),
		c.GetHeader("X-Forwarded-For"),
	))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	pending, rawList, err := waitForOpenClawPendingPairing(ctx, sourceDir, requestID, deviceID, remoteIP)
	if err != nil {
		api.Print_json(
			c,
			"status", "0",
			"message", "list pending failed",
			"error", err.Error(),
			"openclaw_dir", sourceDir,
			http.StatusInternalServerError,
		)
		return
	}

	selected := selectOpenClawPendingRequest(pending, requestID, deviceID, remoteIP)
	if selected == nil {
		api.Print_json(
			c,
			"status", "1",
			"message", "no matching pending pairing request",
			"filter_request_id", requestID,
			"filter_device_id", deviceID,
			"filter_remote_ip", remoteIP,
			"pending_count", len(pending),
			"pending", pending,
			"raw_list", rawList,
		)
		return
	}

	approveOutput, err := approveOpenClawPairing(ctx, sourceDir, selected.RequestID)
	if err != nil {
		api.Print_json(
			c,
			"status", "0",
			"message", "approve pairing failed",
			"error", err.Error(),
			"approved_request", selected,
			"openclaw_dir", sourceDir,
			http.StatusInternalServerError,
		)
		return
	}

	log.Printf(
		"[openclaw-approve] remote=%s selected_request=%s device=%s pending_ip=%s",
		remoteHost, selected.RequestID, selected.DeviceID, selected.RemoteIP,
	)

	api.Print_json(
		c,
		"status", "1",
		"message", "approved",
		"remote_host", remoteHost,
		"filter_request_id", requestID,
		"filter_device_id", deviceID,
		"filter_remote_ip", remoteIP,
		"approved_request", selected,
		"approve_output", approveOutput,
		"openclaw_dir", sourceDir,
	)
}

func listOpenClawPendingPairing(ctx context.Context, sourceDir string) ([]openClawPendingDevice, string, error) {
	output, err := runOpenClawNodeScript(
		ctx,
		sourceDir,
		`import { i as listDevicePairing } from "./dist/device-pairing-De3rgrNp.js";
const list = await listDevicePairing();
console.log(JSON.stringify(list));`,
	)
	if err != nil {
		return nil, output, err
	}

	var payload openClawDevicesList
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, output, fmt.Errorf("parse devices list json: %w", err)
	}
	return payload.Pending, output, nil
}

func waitForOpenClawPendingPairing(
	ctx context.Context,
	sourceDir string,
	requestID string,
	deviceID string,
	remoteIP string,
) ([]openClawPendingDevice, string, error) {
	var lastPending []openClawPendingDevice
	var lastRaw string
	for {
		pending, raw, err := listOpenClawPendingPairing(ctx, sourceDir)
		if err != nil {
			return nil, raw, err
		}
		lastPending = pending
		lastRaw = raw
		if selectOpenClawPendingRequest(pending, requestID, deviceID, remoteIP) != nil {
			return pending, raw, nil
		}
		if requestID == "" && deviceID == "" && remoteIP == "" {
			return pending, raw, nil
		}

		select {
		case <-ctx.Done():
			return lastPending, lastRaw, nil
		case <-time.After(350 * time.Millisecond):
		}
	}
}

func approveOpenClawPairing(ctx context.Context, sourceDir string, requestID string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return "", fmt.Errorf("request id empty")
	}
	return runOpenClawNodeScript(
		ctx,
		sourceDir,
		fmt.Sprintf(`import { t as approveDevicePairing } from "./dist/device-pairing-De3rgrNp.js";
const approved = await approveDevicePairing(%q);
if (!approved) {
  console.log(JSON.stringify({ ok: false, requestId: %q }));
  process.exit(2);
}
console.log(JSON.stringify({ ok: true, requestId: %q, device: approved.device }));`, requestID, requestID, requestID),
	)
}

func runOpenClawNodeScript(ctx context.Context, sourceDir string, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	cmd.Dir = sourceDir
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("node script timeout")
	}
	if err != nil {
		if result == "" {
			return "", err
		}
		return result, fmt.Errorf("%w: %s", err, result)
	}
	return result, nil
}

func selectOpenClawPendingRequest(
	pending []openClawPendingDevice,
	requestID string,
	deviceID string,
	remoteIP string,
) *openClawPendingDevice {
	requestID = strings.TrimSpace(requestID)
	deviceID = strings.TrimSpace(deviceID)
	remoteIP = normalizeRemoteLookupValue(remoteIP)

	var selected *openClawPendingDevice
	for i := range pending {
		item := &pending[i]
		if requestID != "" && item.RequestID != requestID {
			continue
		}
		if deviceID != "" && item.DeviceID != deviceID {
			continue
		}
		if remoteIP != "" && normalizeRemoteLookupValue(item.RemoteIP) != remoteIP {
			continue
		}
		if selected == nil || item.TS > selected.TS {
			selected = item
		}
	}
	if selected != nil {
		return selected
	}
	if requestID != "" || deviceID != "" || remoteIP != "" {
		return nil
	}
	for i := range pending {
		item := &pending[i]
		if selected == nil || item.TS > selected.TS {
			selected = item
		}
	}
	return selected
}

func extractLoopbackRemoteHost(remoteAddr string) (string, bool) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host, false
	}
	return host, ip.IsLoopback()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeRemoteLookupValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ",") {
		value = strings.TrimSpace(strings.Split(value, ",")[0])
	}
	return value
}
