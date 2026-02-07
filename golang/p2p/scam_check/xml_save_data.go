package scam_check

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

const (
	xmlFileName = "check_healing.xml"
	baseURL     = "https://nofake.wiki/next/check/"
)

func XmlSaveDataHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	rows, err := queryRows("SELECT `id`, `alias`, `updatedate` FROM `scam_check` WHERE `status` = 'Completed' ORDER BY `updatedate` DESC LIMIT 49998")
	if err != nil {
		c.String(http.StatusInternalServerError, "Lỗi query: %v", err)
		return
	}

	unique := make(map[string]string, len(rows))
	ordered := make([]string, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row["id"])
		alias := strings.TrimSpace(row["alias"])
		if id == "" || alias == "" {
			continue
		}
		url := baseURL + alias + "_" + id + "/"
		if _, exists := unique[url]; exists {
			continue
		}
		unique[url] = toISO8601(row["updatedate"])
		ordered = append(ordered, url)
	}

	xmlText := buildSitemapXML(ordered, unique)
	if err := os.WriteFile(outputPath(), []byte(xmlText), 0o644); err != nil {
		c.String(http.StatusInternalServerError, "Lỗi lưu XML: %v", err)
		return
	}

	c.String(http.StatusOK, "Tạo check_healing.xml thành công với %d URL", len(ordered))
}

func buildSitemapXML(urls []string, lastmods map[string]string) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, url := range urls {
		lastmod := lastmods[url]
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>")
		b.WriteString(escapeXML(url))
		b.WriteString("</loc>\n")
		if lastmod != "" {
			b.WriteString("    <lastmod>")
			b.WriteString(escapeXML(lastmod))
			b.WriteString("</lastmod>\n")
		} else {
			b.WriteString("    <lastmod></lastmod>\n")
		}
		b.WriteString("    <changefreq>weekly</changefreq>\n")
		b.WriteString("    <priority>0.8</priority>\n")
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.String()
}

func escapeXML(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func toISO8601(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format(time.RFC3339)
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed.Format(time.RFC3339)
	}
	return value
}

func outputPath() string {
	baseDir, err := os.Getwd()
	if err != nil || baseDir == "" {
		return xmlFileName
	}
	return filepath.Join(baseDir, "main", "site_map", xmlFileName)
}

func queryRows(query string) ([]map[string]string, error) {
	conn, err := database.Open()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]string, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			case nil:
				row[col] = ""
			case time.Time:
				row[col] = v.Format("2006-01-02 15:04:05")
			default:
				row[col] = fmt.Sprint(v)
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
