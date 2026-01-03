package telegram

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hust_backend/main/api"

	"github.com/gin-gonic/gin"
)

func AutoReplyHandler(c *gin.Context) {
	text := strings.TrimSpace(c.Query("text"))
	username := strings.TrimSpace(c.Query("username"))
	if text == "" || username == "" {
		api.Print_json(c, "error", "Missing text or username", http.StatusBadRequest)
		return
	}

	pyExe := strings.TrimSpace(os.Getenv("PYTHON_EXE"))
	if pyExe == "" {
		pyExe = "python"
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	scriptPath := filepath.Join(cwd, "p2p", "media", "social", "telegram", "auto_reply.py")
	cmd := exec.Command(pyExe, scriptPath, "--username", username, "--text", text)
	cmd.Dir = cwd

	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		api.Print_json(c, "error", err.Error(), "result", result, http.StatusInternalServerError)
		return
	}

	api.Print_json(c, "status", "ok", "result", result)
}
