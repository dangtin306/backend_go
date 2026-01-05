package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
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
	Jobs []jobConfig `json:"jobs"`
}

type jobConfig struct {
	Name            string `json:"name_cron"`
	Task            string `json:"task_cron"`
	IntervalSeconds int    `json:"interval_seconds"`
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
		taskName := job.Task
		if taskName == "" {
			taskName = "increment_counter"
		}

		task, err := taskFor(taskName)
		if err != nil {
			return err
		}
		if job.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid interval_seconds for job %q", job.Name)
		}

		options := []gocron.JobOption{}
		if job.Name != "" {
			options = append(options, gocron.WithName(job.Name))
		}

		_, err = scheduler.NewJob(
			gocron.DurationJob(time.Duration(job.IntervalSeconds)*time.Second),
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

func taskFor(name string) (func(), error) {
	switch name {
	case "increment_counter":
		return incrementCounter, nil
	default:
		return nil, fmt.Errorf("unknown task %q", name)
	}
}

func incrementCounter() {
	counterMu.Lock()
	defer counterMu.Unlock()

	count, err := readCounter()
	if err != nil {
		log.Printf("read counter: %v", err)
		return
	}

	count++
	if err := writeCounter(count); err != nil {
		log.Printf("write counter: %v", err)
		return
	}

	log.Printf("counter: %d", count)
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
		return 0, fmt.Errorf("empty counter file")
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
