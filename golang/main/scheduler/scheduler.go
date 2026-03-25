package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
)

const counterFileName = "test.txt"
const configFileName = "scheduler.json"

var schedulerInstance gocron.Scheduler
var counterMu sync.Mutex
var jobRunState sync.Map

// --- STRUCT ---

type schedulerConfig struct {
	Categories []categoryConfig `json:"categories_cron"`
}

type categoryConfig struct {
	Key      string      `json:"category_key"`
	Services []jobConfig `json:"services_cron"`
}

func (c *categoryConfig) UnmarshalJSON(data []byte) error {
	type rawCategoryConfig struct {
		CategoryKey   string      `json:"category_key"`
		CategoryValue string      `json:"category_value"`
		Services      []jobConfig `json:"services_cron"`
	}

	var raw rawCategoryConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	c.Key = firstNonEmpty(raw.CategoryKey, raw.CategoryValue)
	c.Services = raw.Services
	return nil
}

type jobConfig struct {
	Name            string  `json:"name_cron"`
	Task            string  `json:"task_cron"`
	Type            string  `json:"cron_tyoe"`
	IntervalSeconds float64 `json:"interval_seconds"`
	Status          bool    `json:"status_cron"`
	CronShow        bool    `json:"cron_show"`
	CronSingleton   bool    `json:"cron_single"`
	RunOnStart      bool    `json:"run_on_start_cron"`
	TimeoutSeconds  int     `json:"cron_timeout_seconds"`
	AtTime          string  `json:"at_time_cron"`
	Command         string  `json:"command_cron"`
	Url             string  `json:"url_cron"` // [M?I] Link c?n ping
}

func (j *jobConfig) UnmarshalJSON(data []byte) error {
	type rawJobConfig struct {
		Name            string  `json:"name_cron"`
		Task            string  `json:"task_cron"`
		CronType        string  `json:"cron_tyoe"`
		LegacyType      string  `json:"type_cron"`
		IntervalSeconds float64 `json:"interval_seconds"`
		Status          bool    `json:"status_cron"`
		CronShow        bool    `json:"cron_show"`
		CronSingle      bool    `json:"cron_single"`
		LegacySingleton bool    `json:"cron_singleton"`
		RunOnStart      bool    `json:"run_on_start_cron"`
		TimeoutSeconds  int     `json:"cron_timeout_seconds"`
		AtTime          string  `json:"at_time_cron"`
		Command         string  `json:"command_cron"`
		URL             string  `json:"url_cron"`
	}

	var raw rawJobConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	j.Name = raw.Name
	j.Task = raw.Task
	j.Type = firstNonEmpty(raw.CronType, raw.LegacyType)
	j.IntervalSeconds = raw.IntervalSeconds
	j.Status = raw.Status
	j.CronShow = raw.CronShow
	j.CronSingleton = raw.CronSingle || raw.LegacySingleton
	j.RunOnStart = raw.RunOnStart
	j.TimeoutSeconds = raw.TimeoutSeconds
	j.AtTime = raw.AtTime
	j.Command = raw.Command
	j.Url = raw.URL
	return nil
}

// -----------------------------------------------------

func StartCounter() error {
	if schedulerInstance != nil {
		return nil
	}
	if err := ensureCounterFile(); err != nil {
		return err
	}

	config, err := loadConfig()
	if err != nil {
		return err
	}

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return err
	}

	if len(config.Categories) == 0 {
		return fmt.Errorf("config empty")
	}

	for _, category := range config.Categories {
		for _, job := range category.Services {
			fullName := fmt.Sprintf("%s > %s", category.Key, job.Name)

			if !job.Status {
				log.Printf("⚠️  [%s] OFF -> Skip", fullName)
				continue
			}

			if job.IntervalSeconds <= 0 {
				log.Printf("❌ [%s] Lỗi: interval_seconds <= 0", fullName)
				continue
			}

			taskName := job.Task
			if taskName == "" {
				taskName = "increment_counter"
			}

			// Truyền Url/Command vào hàm taskFor
			task, err := taskFor(job.Type, taskName, fullName, job.Url, job.Command, job.CronShow, job.CronSingleton, job.TimeoutSeconds)
			if err != nil {
				return err
			}

			options := []gocron.JobOption{
				gocron.WithName(fullName),
			}

			if job.RunOnStart {
				options = append(options, gocron.WithStartAt(gocron.WithStartImmediately()))
			}

			if job.AtTime != "" {
				startTime, err := parseTimeToday(job.AtTime)
				if err != nil {
					log.Printf("❌ [%s] Lỗi giờ (at_time_cron): %v", fullName, err)
				} else {
					now := time.Now()
					if startTime.After(now) {
						log.Printf("⏳ [%s] Hẹn giờ lúc %s", fullName, startTime.Format("15:04:05"))
						options = append(options, gocron.WithStartAt(
							gocron.WithStartDateTime(startTime),
						))
					} else {
						log.Printf("▶️  [%s] Quá giờ (%s) -> Chạy ngay", fullName, job.AtTime)
					}
				}
			}

			duration := time.Duration(job.IntervalSeconds * float64(time.Second))
			_, err = scheduler.NewJob(
				gocron.DurationJob(duration),
				gocron.NewTask(task),
				options...,
			)
			if err != nil {
				return err
			}
		}
	}

	scheduler.Start()
	schedulerInstance = scheduler
	return nil
}

// --- LOGIC TASK ---
func taskFor(jobType string, taskType string, fullName string, url string, command string, cronShow bool, cronSingleton bool, timeoutSeconds int) (func(), error) {
	normalizedType := strings.ToLower(strings.TrimSpace(jobType))
	commandToRun := strings.TrimSpace(command)
	urlToRun := strings.TrimSpace(url)

	if normalizedType == "" {
		switch {
		case commandToRun != "":
			normalizedType = "powershell"
		case urlToRun != "":
			normalizedType = "api_get"
		case taskType == "increment_counter" || strings.HasPrefix(taskType, "increment_counter_"):
			normalizedType = "internal"
		}
	}

	switch normalizedType {
	case "internal":
		switch {
		case taskType == "increment_counter" || strings.HasPrefix(taskType, "increment_counter_"):
			return wrapJobExecution(fullName, normalizedType, cronSingleton, func() { incrementCounter(fullName) }), nil
		default:
			return nil, fmt.Errorf("task internal %q chua duoc ho tro", taskType)
		}
	case "powershell":
		if commandToRun == "" {
			return nil, fmt.Errorf("task powershell %q thieu command_cron", taskType)
		}
		return wrapJobExecution(fullName, normalizedType, cronSingleton, func() { executePowerShellTask(fullName, taskType, commandToRun, cronShow, timeoutSeconds) }), nil
	case "api_get":
		if urlToRun == "" {
			return nil, fmt.Errorf("task api_get %q thieu url_cron", taskType)
		}
		return wrapJobExecution(fullName, normalizedType, cronSingleton, func() { executeUrlTask(fullName, taskType, urlToRun, timeoutSeconds) }), nil
	default:
		return nil, fmt.Errorf("cron_tyoe %q khong hop le cho task %q", normalizedType, taskType)
	}
}

// Hàm chạy URL tổng quát (Đổi tên từ pingUrl thành executeUrlTask cho chuẩn)
func wrapJobExecution(fullName string, jobType string, cronSingleton bool, runner func()) func() {
	return func() {
		if cronSingleton {
			if _, loaded := jobRunState.LoadOrStore(fullName, true); loaded {
				log.Printf("[%s] [%s] Skip: previous run still active (cron_single=true)", fullName, jobType)
				return
			}
			defer jobRunState.Delete(fullName)
		}

		runner()
	}
}

func executeUrlTask(fullName string, taskType string, url string, timeoutSeconds int) {
	client := http.Client{Timeout: durationFromSeconds(timeoutSeconds, 10*time.Second)}
	resp, err := client.Get(url)

	if err != nil {
		log.Printf("[%s] [%s] ❌ Fail: %v", fullName, taskType, err)
		return
	}
	defer resp.Body.Close()

	// In ra cả tên loại task (taskType) để bạn biết nó đang chạy kiểu gì
	log.Printf("[%s] [%s] ✅ Status: %s (Link: %s)", fullName, taskType, resp.Status, url)
}

func executePowerShellTask(fullName string, taskType string, command string, cronShow bool, timeoutSeconds int) {
	command = strings.TrimSpace(command)
	if command == "" {
		log.Printf("[%s] [%s] PowerShell skip: command_cron empty", fullName, taskType)
		return
	}

	if runtime.GOOS == "windows" {
		if cronShow {
			psCommand := fmt.Sprintf("$ErrorActionPreference='Stop'; try { %s } finally { Start-Sleep -Seconds 3 }", command)
			cmd := exec.Command(
				"cmd", "/C", "start", "",
				"powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand,
			)
			output, err := cmd.CombinedOutput()
			result := strings.TrimSpace(string(output))
			if err != nil {
				log.Printf("[%s] [%s] PowerShell launch fail: %v | Output: %s", fullName, taskType, err, result)
				return
			}
			log.Printf("[%s] [%s] Opened PowerShell window (auto close after 3s)", fullName, taskType)
			return
		}
	}

	timeout := durationFromSeconds(timeoutSeconds, 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[%s] [%s] PowerShell timeout after %s | Command: %s", fullName, taskType, timeout, command)
		return
	}

	if err != nil {
		log.Printf("[%s] [%s] PowerShell fail: %v | Output: %s", fullName, taskType, err, result)
		return
	}

	log.Printf("[%s] [%s] PowerShell output: %s", fullName, taskType, result)
}

func durationFromSeconds(timeoutSeconds int, fallback time.Duration) time.Duration {
	if timeoutSeconds <= 0 {
		return fallback
	}
	return time.Duration(timeoutSeconds) * time.Second
}

// func taskFor(taskType string, fullName string, url string) (func(), error) {
// 	switch taskType {
// 	case "increment_counter":
// 		return func() { incrementCounter(fullName) }, nil

// 	case "ping_url": // Task dùng link từ JSON
// 		return func() { pingUrl(fullName, url) }, nil

// 	case "ping_google": // Task cũ (giữ lại cho tương thích)
// 		return func() { pingUrl(fullName, "https://www.google.com") }, nil

// 	default:
// 		return nil, fmt.Errorf("unknown task: %q", taskType)
// 	}
// }

// func pingUrl(fullName string, url string) {
// 	if url == "" {
// 		log.Printf("[%s] ❌ Lỗi: Chưa điền url_cron trong JSON!", fullName)
// 		return
// 	}

// 	client := http.Client{Timeout: 10 * time.Second}
// 	resp, err := client.Get(url)
// 	if err != nil {
// 		log.Printf("[%s] ❌ Ping [%s] Fail: %v", fullName, url, err)
// 		return
// 	}
// 	defer resp.Body.Close()
// 	log.Printf("[%s] ✅ Ping [%s] -> Status: %s", fullName, url, resp.Status)
// }

func incrementCounter(fullName string) {
	counterMu.Lock()
	defer counterMu.Unlock()
	count, _ := readCounter()
	count++
	writeCounter(count)
	log.Printf("[%s] 🔢 Counter: %d", fullName, count)
}

// --- TIỆN ÍCH (ĐÃ SỬA LỖI BIẾN ERR) ---

func parseTimeToday(timeStr string) (time.Time, error) {
	now := time.Now()
	parts := strings.Split(timeStr, ":")

	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("sai định dạng HH:MM")
	}

	// Sửa lỗi: Khai báo và check lỗi đàng hoàng
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}

	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}

	s := 0
	if len(parts) == 3 {
		s, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, err
		}
	}

	return time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, now.Location()), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func loadConfig() (schedulerConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return schedulerConfig{}, err
	}

	// Support UTF-8 BOM files saved by some Windows editors.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var config schedulerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return schedulerConfig{}, fmt.Errorf("parse %s: %w", configPath(), err)
	}
	return config, nil
}

func ensureCounterFile() error {
	_, err := os.Stat(counterPath())
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeCounter(0)
}

func readCounter() (int, error) {
	data, err := os.ReadFile(counterPath())
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, nil
	}
	return strconv.Atoi(text)
}

func writeCounter(value int) error {
	return os.WriteFile(counterPath(), []byte(strconv.Itoa(value)), 0o644)
}

func counterPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return counterFileName
	}
	return filepath.Join(filepath.Dir(file), counterFileName)
}

func configPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return configFileName
	}
	return filepath.Join(filepath.Dir(file), configFileName)
}
