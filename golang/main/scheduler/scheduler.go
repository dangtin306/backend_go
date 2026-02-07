package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

// --- STRUCT ---

type schedulerConfig struct {
	Categories []categoryConfig `json:"categories_cron"`
}

type categoryConfig struct {
	Name     string      `json:"name_category"`
	Services []jobConfig `json:"services_cron"`
}

type jobConfig struct {
	Name            string  `json:"name_cron"`
	Task            string  `json:"task_cron"`
	IntervalSeconds float64 `json:"interval_seconds"`
	Status          bool    `json:"status_cron"`
	AtTime          string  `json:"at_time_cron"`
	Url             string  `json:"url_cron"` // [MỚI] Link cần ping
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
			fullName := fmt.Sprintf("%s > %s", category.Name, job.Name)

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

			// Truyền Url vào hàm taskFor
			task, err := taskFor(taskName, fullName, job.Url)
			if err != nil {
				return err
			}

			options := []gocron.JobOption{
				gocron.WithName(fullName),
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
func taskFor(taskType string, fullName string, url string) (func(), error) {

	// 1. Kiểm tra các task Logic Đặc Biệt (không dùng URL hoặc logic nội bộ)
	if taskType == "increment_counter" {
		return func() { incrementCounter(fullName) }, nil
	}

	// 2. LOGIC TỰ ĐỘNG:
	// Nếu có URL -> Tự động hiểu là task Ping URL.
	// Bất kể tên task là "cc_test", "ping_google", "check_ip"... đều chạy tuốt.
	if url != "" {
		return func() { executeUrlTask(fullName, taskType, url) }, nil
	}

	// 3. Nếu không có URL mà tên task lạ hoắc -> Lỗi
	return nil, fmt.Errorf("task %q lạ quá (không có URL để chạy)", taskType)
}

// Hàm chạy URL tổng quát (Đổi tên từ pingUrl thành executeUrlTask cho chuẩn)
func executeUrlTask(fullName string, taskType string, url string) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)

	if err != nil {
		log.Printf("[%s] [%s] ❌ Fail: %v", fullName, taskType, err)
		return
	}
	defer resp.Body.Close()

	// In ra cả tên loại task (taskType) để bạn biết nó đang chạy kiểu gì
	log.Printf("[%s] [%s] ✅ Status: %s (Link: %s)", fullName, taskType, resp.Status, url)
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

func loadConfig() (schedulerConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return schedulerConfig{}, err
	}
	var config schedulerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return schedulerConfig{}, err
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
