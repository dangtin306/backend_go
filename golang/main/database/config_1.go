package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   string
}

type DB struct{}

type Row struct {
	data map[string]any
	err  error
}

var (
	db      *sql.DB
	lock    sync.Mutex
	PlaySQL DB
)

func Open() (*sql.DB, error) {
	lock.Lock()
	defer lock.Unlock()

	if db != nil {
		return db, nil
	}

	cfg := LoadConfig()
	if cfg.User == "" || cfg.Database == "" {
		return nil, fmt.Errorf("missing DB_USER/DB_NAME (or MYSQL_USER/MYSQL_DB)")
	}

	dsn := BuildDSN(cfg)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(readEnvInt("DB_MAX_OPEN", 1000))
	conn.SetMaxIdleConns(readEnvInt("DB_MAX_IDLE", 100))
	conn.SetConnMaxLifetime(time.Duration(readEnvInt("DB_MAX_LIFETIME", 300)) * time.Second)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	db = conn
	return db, nil
}

func Close() error {
	lock.Lock()
	defer lock.Unlock()

	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

// Query returns the first row as a map-like helper for PHP-style access.
func (db DB) Query(query string, args ...any) Row {
	return db.QueryCtx(context.Background(), query, args...)
}

// QueryCtx does the same as Query but allows custom context.
func (DB) QueryCtx(ctx context.Context, query string, args ...any) Row {
	conn, err := Open()
	if err != nil {
		return Row{err: err}
	}

	if !isSelectQuery(query) {
		_, err := conn.ExecContext(ctx, query, args...)
		if err != nil {
			return Row{err: err}
		}
		return Row{data: map[string]any{}}
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return Row{err: err}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return Row{err: err}
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		colTypes = nil
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Row{err: err}
		}
		return Row{err: sql.ErrNoRows}
	}

	values := make([]any, len(cols))
	pointers := make([]any, len(cols))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return Row{err: err}
	}

	data := make(map[string]any, len(cols))
	for i, col := range cols {
		if b, ok := values[i].([]byte); ok {
			data[col] = castByColumnType(b, colTypes, i)
		} else {
			data[col] = values[i]
		}
	}

	return Row{data: data}
}

// Query is a package-level helper (useful with alias import).
func Query(query string, args ...any) Row {
	return PlaySQL.Query(query, args...)
}

func (r Row) Err() error {
	return r.err
}

// FetchArray returns the raw map for PHP-style access.
func (r Row) FetchArray() map[string]any {
	if r.err != nil {
		return nil
	}
	return r.data
}

// Fetch_array is PHP-style alias (must be exported to use across packages).
func (r Row) Fetch_array() map[string]any {
	return r.FetchArray()
}

func castByColumnType(raw []byte, colTypes []*sql.ColumnType, index int) any {
	if raw == nil {
		return nil
	}
	value := string(raw)
	if colTypes == nil || index < 0 || index >= len(colTypes) || colTypes[index] == nil {
		return value
	}
	typeName := strings.ToUpper(colTypes[index].DatabaseTypeName())
	switch typeName {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT":
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	case "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL":
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	case "BIT", "BOOL", "BOOLEAN":
		if value == "1" {
			return true
		}
		if value == "0" {
			return false
		}
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return value
}

func isSelectQuery(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "select", "with", "show", "describe", "desc", "explain":
		return true
	default:
		return false
	}
}

func LoadConfig() Config {
	fileCfg, _ := loadConfigFile()
	host := firstEnv("DB_HOST", "MYSQL_HOST")
	if host == "" {
		host = fileCfg.Host
	}
	if host == "" {
		host = "vip.tecom.pro"
	}

	port := 0
	if envPort, ok := firstEnvInt("DB_PORT", "MYSQL_PORT"); ok {
		port = envPort
	}
	if port == 0 {
		port = fileCfg.Port
	}
	if port == 0 {
		port = 3306
	}
	user := firstEnv("DB_USER", "MYSQL_USER")
	if user == "" {
		user = fileCfg.User
	}
	if user == "" {
		user = "hust_media_vip"
	}
	pass := firstEnv("DB_PASS", "MYSQL_PASSWORD")
	if pass == "" {
		pass = fileCfg.Password
	}
	if pass == "" {
		pass = "hust_media_vip"
	}
	name := firstEnv("DB_NAME", "MYSQL_DB")
	if name == "" {
		name = fileCfg.Database
	}
	if name == "" {
		name = "hustmedi_777"
	}
	params := firstEnv("DB_PARAMS", "MYSQL_PARAMS")
	if params == "" {
		params = fileCfg.Params
	}
	if params == "" {
		params = "charset=utf8mb4&parseTime=true&loc=Local"
	}

	return Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: pass,
		Database: name,
		Params:   params,
	}
}

func BuildDSN(cfg Config) string {
	auth := cfg.User
	if cfg.Password != "" {
		auth = auth + ":" + cfg.Password
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return fmt.Sprintf("%s@tcp(%s)/%s?%s", auth, addr, cfg.Database, cfg.Params)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func firstEnvInt(keys ...string) (int, bool) {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		return parsed, true
	}
	return 0, false
}

func readEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func loadConfigFile() (Config, bool) {
	baseDir, err := os.Getwd()
	if err != nil {
		baseDir = "."
	}
	path := filepath.Join(baseDir, "main", "database", "config_1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, false
	}

	cfg := Config{
		Host:     readString(raw, "host"),
		User:     readString(raw, "user"),
		Password: readString(raw, "password"),
		Database: readString(raw, "database"),
		Params:   readString(raw, "params"),
	}
	if port, ok := readInt(raw, "port"); ok {
		cfg.Port = port
	}

	return cfg, true
}

func readString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func readInt(values map[string]any, key string) (int, bool) {
	if values == nil {
		return 0, false
	}
	value, ok := values[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
