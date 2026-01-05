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

type schedulerConfig struct {
	Jobs []jobConfig `json:"jobs_cron"`
}

type jobConfig struct {
	Name            string  `json:"name_cron"`
	Task            string  `json:"task_cron"`
	IntervalSeconds float64 `json:"interval_seconds"`
	Status          bool    `json:"status_cron"`
	AtTime          string  `json:"at_time_cron"`
}

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

	if len(config.Jobs) == 0 {
		return fmt.Errorf("scheduler config has no jobs")
	}

	for _, job := range config.Jobs {
		// 1. Kiểm tra Status
		if !job.Status {
			log.Printf("⚠️  [Job: %s] Đang TẮT -> Bỏ qua", job.Name)
			continue
		}

		// 2. Kiểm tra Interval
		if job.IntervalSeconds <= 0 {
			log.Printf("❌ [Job: %s] Lỗi: interval_seconds phải > 0", job.Name)
			continue
		}

		taskName := job.Task
		if taskName == "" {
			taskName = "increment_counter"
		}

		// [THAY ĐỔI] Truyền job.Name vào hàm taskFor để nó biết tên job
		task, err := taskFor(taskName, job.Name)
		if err != nil {
			return err
		}

		options := []gocron.JobOption{
			gocron.WithName(job.Name),
		}

		// 3. Xử lý AtTime
		if job.AtTime != "" {
			startTime, err := parseTimeToday(job.AtTime)
			if err != nil {
				log.Printf("❌ [Job: %s] Lỗi giờ: %v", job.Name, err)
			} else {
				now := time.Now()
				if startTime.After(now) {
					log.Printf("⏳ [Job: %s] Hẹn giờ chạy lúc %s", job.Name, startTime.Format("15:04:05"))
					options = append(options, gocron.WithStartAt(
						gocron.WithStartDateTime(startTime),
					))
				} else {
					log.Printf("▶️  [Job: %s] Đã qua giờ hẹn (%s) -> Chạy ngay", job.Name, job.AtTime)
				}
			}
		}

		// 4. Tạo Job Loop
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

	scheduler.Start()
	schedulerInstance = scheduler
	return nil
}

// Hàm parse giờ
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
		return time.Time{}, fmt.Errorf("sai định dạng (dùng HH:MM)")
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

// [THAY ĐỔI] Hàm taskFor nhận thêm jobName và trả về hàm con (Closure)
func taskFor(taskType string, jobName string) (func(), error) {
	switch taskType {
	case "increment_counter":
		// Trả về hàm nặc danh đã "gói" jobName vào bên trong
		return func() { incrementCounter(jobName) }, nil
	case "ping_google":
		return func() { pingGoogle(jobName) }, nil
	default:
		return nil, fmt.Errorf("unknown task %q", taskType)
	}
}

// [THAY ĐỔI] Nhận jobName để in ra log
func pingGoogle(jobName string) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		log.Printf("[Job: %s] ❌ Ping Google Fail: %v", jobName, err)
		return
	}
	defer resp.Body.Close()
	// [HIỂN THỊ TÊN JOB]
	log.Printf("[Job: %s] ✅ Ping Google Status: %s", jobName, resp.Status)
}

// [THAY ĐỔI] Nhận jobName để in ra log
func incrementCounter(jobName string) {
	counterMu.Lock()
	defer counterMu.Unlock()

	count, _ := readCounter()
	count++
	writeCounter(count)

	// [HIỂN THỊ TÊN JOB]
	log.Printf("[Job: %s] 🔢 Counter tăng lên: %d", jobName, count)
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
