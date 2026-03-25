package media

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
	hustMediaXmlFileName = "hust_media.xml"
	listsCategoryURL     = "https://hust.media/shop/category/tips_vip"
	hustMediaHomeURL     = "https://hust.media/reactapp/"
	hustMediaAboutURL    = "https://hust.media/next/info/about_us"
	hustMediaTermsURL    = "https://hust.media/next/info/terms_service"
	hustMediaPolicyURL   = "https://hust.media/next/info/private_policy"
	hustMediaSupportURL  = "https://hust.media/next/support"
	hustMediaDocsURL     = "https://hust.media/next/docs/overview"
	hustMediaArchURL     = "https://hust.media/next/community/docs/architecture"
	hustMediaAlgoURL     = "https://hust.media/next/community/docs/algorithm"
	hustMediaApiRefURL   = "https://hust.media/next/community/docs/api-reference"
	hustMediaThreatURL   = "https://hust.media/next/community/docs/security-threat-detection"
	hustMediaGameURL     = "https://hust.media/next/community/docs/gamification"
	hustMediaShopURL     = "https://hust.media/shop/channel"
	hustMediaDevURL      = "https://hust.media/next/services/development"
	hustMediaTtsURL      = "https://hust.media/ai/orders_once/text_speech"
	hustMediaSttURL      = "https://hust.media/ai/orders_once/speech_text"
	hustMediaImgTextURL  = "https://hust.media/ai/orders_once/image_text"
	hustMediaExtraPath   = `C:\hustmedia2\api\sitemap\hust_media.xml`
	hustMediaTipsBaseURL = "https://hust.media/shop/posts"
)

func HustMediaXmlSaveHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	lastmod := time.Now().Format(time.RFC3339)
	urls := []string{
		hustMediaHomeURL,
		hustMediaAboutURL,
		hustMediaTermsURL,
		hustMediaPolicyURL,
		hustMediaSupportURL,
		hustMediaDocsURL,
		hustMediaArchURL,
		hustMediaAlgoURL,
		hustMediaApiRefURL,
		hustMediaThreatURL,
		hustMediaGameURL,
		hustMediaShopURL,
		// hustMediaDevURL,
		// hustMediaTtsURL,
		// hustMediaSttURL,
		// hustMediaImgTextURL,
		listsCategoryURL,
	}

	tipsUrls, err := fetchTipsVipUrls()
	if err != nil {
		c.String(http.StatusInternalServerError, "Loi query tips_vip: %v", err)
		return
	}
	if len(tipsUrls) > 0 {
		urls = append(urls, tipsUrls...)
	}

	xmlText := buildHustMediaXML(urls, lastmod)
	if err := os.WriteFile(hustMediaOutputPath(), []byte(xmlText), 0o644); err != nil {
		c.String(http.StatusInternalServerError, "Loi luu XML: %v", err)
		return
	}
	if err := writeExtraHustMediaXML(xmlText); err != nil {
		c.String(http.StatusInternalServerError, "Loi luu XML phu: %v", err)
		return
	}

	c.String(http.StatusOK, "Tao hust_media.xml thanh cong voi %d URL", len(urls))
}

func buildHustMediaXML(urls []string, lastmod string) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, url := range urls {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>")
		b.WriteString(escapeXML(url))
		b.WriteString("</loc>\n")
		if lastmod != "" {
			b.WriteString("    <lastmod>")
			b.WriteString(escapeXML(lastmod))
			b.WriteString("</lastmod>\n")
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

func hustMediaOutputPath() string {
	baseDir, err := os.Getwd()
	if err != nil || baseDir == "" {
		return hustMediaXmlFileName
	}
	return filepath.Join(baseDir, "main", "site_map", hustMediaXmlFileName)
}

func writeExtraHustMediaXML(xmlText string) error {
	dir := filepath.Dir(hustMediaExtraPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(hustMediaExtraPath, []byte(xmlText), 0o644)
}

func fetchTipsVipUrls() ([]string, error) {
	conn, err := database.Open()
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query("SELECT `uri` FROM `tips_news` WHERE `category` = 'tips_vip' ORDER BY `id` DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := make([]string, 0)
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			return nil, err
		}
		uri = strings.TrimSpace(uri)
		if uri == "" {
			continue
		}
		urls = append(urls, joinHustMediaURL(hustMediaTipsBaseURL, uri))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

func joinHustMediaURL(baseURL string, uri string) string {
	baseURL = strings.TrimSpace(baseURL)
	uri = strings.TrimSpace(uri)
	if baseURL == "" {
		return uri
	}
	if uri == "" {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/") && strings.HasPrefix(uri, "/") {
		return baseURL + strings.TrimPrefix(uri, "/")
	}
	if strings.HasSuffix(baseURL, "/") || strings.HasPrefix(uri, "/") {
		return baseURL + uri
	}
	return fmt.Sprintf("%s/%s", baseURL, uri)
}
