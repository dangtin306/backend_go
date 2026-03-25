package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"hust_backend/main/database"

	"golang.org/x/text/unicode/norm"
)

type accountRow struct {
	ServiceID   int64
	Caption     string
	Alias       string
	Description string
}

type accountJSON struct {
	AccountsAlias   string            `json:"accounts_alias"`
	AccountsCaption map[string]string `json:"accounts_caption"`
	IDAccountsMain  int64             `json:"id_accounts_main"`
	Description     map[string]string `json:"description"`
}

func main() {
	conn, err := database.Open()
	if err != nil {
		fmt.Printf("db_open_error: %v\n", err)
		return
	}

	rows, err := conn.Query(`
SELECT
	m.id,
	m.accounts_caption,
	COALESCE((
		SELECT s.accounts_alias
		FROM accounts_lists_seo s
		WHERE s.id_accounts_main = m.id
		ORDER BY s.id DESC
		LIMIT 1
	), '') AS alias,
	COALESCE((
		SELECT s.accounts_description
		FROM accounts_lists_seo s
		WHERE s.id_accounts_main = m.id
		ORDER BY s.id DESC
		LIMIT 1
	), '') AS description
FROM accounts_lists_main m
WHERE m.accounts_status = 'Completed'
ORDER BY m.id DESC`)
	if err != nil {
		fmt.Printf("query_error: %v\n", err)
		return
	}
	defer rows.Close()

	records := make([]accountRow, 0, 1024)
	for rows.Next() {
		var item accountRow
		if err := rows.Scan(&item.ServiceID, &item.Caption, &item.Alias, &item.Description); err != nil {
			continue
		}
		records = append(records, item)
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("rows_error: %v\n", err)
		return
	}

	outFile := outputPath()
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		fmt.Printf("mkdir_error: %v\n", err)
		return
	}

	file, err := os.Create(outFile)
	if err != nil {
		fmt.Printf("create_file_error: %v\n", err)
		return
	}
	defer file.Close()

	output := make([]accountJSON, 0, len(records))
	for _, item := range records {
		output = append(output, accountJSON{
			AccountsAlias: pickFileName(item.Alias, item.Caption, item.ServiceID),
			AccountsCaption: map[string]string{
				"vi": cleanText(item.Caption),
			},
			IDAccountsMain: item.ServiceID,
			Description: map[string]string{
				"vi": cleanText(item.Description),
			},
		})
	}

	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("json_marshal_error: %v\n", err)
		return
	}
	if _, err := file.Write(encoded); err != nil {
		fmt.Printf("write_file_error: %v\n", err)
		return
	}
	_, _ = file.WriteString("\n")

	fmt.Printf("written=%d file=%s\n", len(records), outFile)
}

func pickFileName(alias string, caption string, serviceID int64) string {
	alias = strings.TrimSpace(alias)
	if alias != "" && !strings.EqualFold(alias, "canceled") {
		return alias
	}

	slug := slugify(caption)
	if slug == "" {
		return fmt.Sprintf("service-%d", serviceID)
	}
	return slug
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	normalized := norm.NFD.String(value)
	var b strings.Builder
	lastDash := false

	for _, r := range normalized {
		if unicode.Is(unicode.Mn, r) {
			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}

		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	return out
}

func cleanText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.Join(strings.Fields(value), " ")
}

func outputPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "accounts_name.json"
	}
	return filepath.Join(filepath.Dir(file), "accounts_name.json")
}
