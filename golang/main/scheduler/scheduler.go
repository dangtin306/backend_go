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

// --- Tên file cấu hình và file lưu số đếm ---
const counterFileName = "test.txt"
const configFileName = "scheduler.json"

var schedulerInstance gocron.Scheduler
var counterMu sync.Mutex

// --- [CẤU TRÚC STRUCT KHỚP VỚI JSON CỦA BẠN] ---

type schedulerConfig struct {
	Categories []categoryConfig `json:"categories_cron"` // Khớp với JSON: categories_cron
}

type categoryConfig struct {
	Name     string      `json:"name_category"` // Khớp với JSON: name_category
	Services []jobConfig `json:"services_cron"` // Khớp với JSON: services_cron
}

type jobConfig struct {
	Name            string  `json:"name_cron"`        // Khớp: name_cron
	Task            string  `json:"task_cron"`        // Khớp: task_cron
	IntervalSeconds float64 `json:"interval_seconds"` // Khớp: interval_seconds
	Status          bool    `json:"status_cron"`      // Khớp: status_cron
	AtTime          string  `json:"at_time_cron"`     // Khớp: at_time_cron
}

// -----------------------------------------------------

func StartCounter() error {
	if schedulerInstance != nil {
		return nil
	}
	// Tạo file test.txt nếu chưa có
	if err := ensureCounterFile(); err != nil {
		return err
	}

	// Load file JSON
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Khởi tạo Scheduler
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return err
	}

	if len(config.Categories) == 0 {
		return fmt.Errorf("file json không có categories_cron nào")
	}

	// [VÒNG LẶP 1] Duyệt qua từng Category
	for _, category := range config.Categories {

		// [VÒNG LẶP 2] Duyệt qua từng Job (Service) trong Category đó
		for _, job := range category.Services {

			// Tạo tên hiển thị log dạng: "System Tasks > Counter Job"
			fullName := fmt.Sprintf("%s > %s", category.Name, job.Name)

			// 1. Kiểm tra Status: Nếu false thì bỏ qua
			if !job.Status {
				log.Printf("⚠️  [%s] Đang TẮT (Status=false) -> Bỏ qua", fullName)
				continue
			}

			// 2. Kiểm tra Interval: Phải có thời gian lặp
			if job.IntervalSeconds <= 0 {
				log.Printf("❌ [%s] Lỗi: interval_seconds phải lớn hơn 0", fullName)
				continue
			}

			// Xác định loại Task (increment_counter hay ping_google)
			taskName := job.Task
			if taskName == "" {
				taskName = "increment_counter"
			}

			// Truyền fullName vào task để in log đẹp
			task, err := taskFor(taskName, fullName)
			if err != nil {
				return err
			}

			options := []gocron.JobOption{
				gocron.WithName(fullName),
			}

			// 3. Xử lý Hẹn giờ bắt đầu (at_time_cron)
			if job.AtTime != "" {
				startTime, err := parseTimeToday(job.AtTime)
				if err != nil {
					log.Printf("❌ [%s] Lỗi định dạng giờ (at_time_cron): %v", fullName, err)
				} else {
					now := time.Now()
					// Chỉ hẹn giờ nếu thời gian đó ở Tương Lai
					if startTime.After(now) {
						log.Printf("⏳ [%s] Hẹn giờ chạy lúc %s", fullName, startTime.Format("15:04:05"))
						options = append(options, gocron.WithStartAt(
							gocron.WithStartDateTime(startTime),
						))
					} else {
						// Nếu đã qua giờ hẹn thì chạy luôn
						log.Printf("▶️  [%s] Đã qua giờ hẹn (%s) -> Chạy ngay", fullName, job.AtTime)
					}
				}
			}

			// 4. Tạo Job chạy lặp lại
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

// --- LOGIC CÁC TASK ---

func taskFor(taskType string, fullName string) (func(), error) {
	switch taskType {
	case "increment_counter":
		return func() { incrementCounter(fullName) }, nil
	case "ping_google":
		return func() { pingGoogle(fullName) }, nil
	default:
		return nil, fmt.Errorf("không tìm thấy task: %q", taskType)
	}
}

func pingGoogle(fullName string) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		log.Printf("[%s] ❌ Ping Lỗi: %v", fullName, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[%s] ✅ Ping Google: %s", fullName, resp.Status)
}

func incrementCounter(fullName string) {
	counterMu.Lock()
	defer counterMu.Unlock()

	count, _ := readCounter()
	count++
	writeCounter(count)

	log.Printf("[%s] 🔢 Counter: %d", fullName, count)
}

// --- CÁC HÀM TIỆN ÍCH ---

// Hàm parse giờ: HH:MM hoặc HH:MM:SS của ngày hôm nay
func parseTimeToday(timeStr string) (time.Time, error) {
	now := time.Now()
	parts := strings.Split(timeStr, ":")
	h, m, s := 0, 0, 0
	var err error

	if len(parts) >= 2 {
		h, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, err
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, err
		}
	}
	if len(parts) == 3 {
		s, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, err
		}
	}
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("sai định dạng HH:MM")
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
