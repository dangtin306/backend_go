package service

import (
	"database/sql"
	"net/http"
	"strings"

	"hust_backend/main/api"
	"hust_backend/main/database"

	"github.com/gin-gonic/gin"
)

type moneyListItem struct {
	ID            int64  `json:"id"`
	Website       string `json:"website"`
	KeyCheckMoney int64  `json:"key_check_money"`
	Money         string `json:"money"`
	KeyMoneyOK    string `json:"key_money_ok,omitempty"`
	CreateDate    string `json:"createdate"`
}

func hasColumn(conn *sql.DB, tableName string, columnName string) (bool, error) {
	cfg := database.LoadConfig()
	var count int
	err := conn.QueryRow(
		`SELECT COUNT(*)
		 FROM INFORMATION_SCHEMA.COLUMNS
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		cfg.Database, tableName, columnName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MoneyListHandler returns rows in apikey with key_check_money = 1 and their money fields.
func MoneyListHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	conn, err := database.Open()
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}

	hasID, err := hasColumn(conn, "apikey", "id")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	hasWebsite, err := hasColumn(conn, "apikey", "website")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	hasKeyCheckMoney, err := hasColumn(conn, "apikey", "key_check_money")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	if !hasID || !hasWebsite || !hasKeyCheckMoney {
		api.Print_json(c, "status", "0", "message", "apikey table missing required columns: id/website/key_check_money")
		return
	}

	hasMoney, err := hasColumn(conn, "apikey", "money")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	hasKeyMoneyOK, err := hasColumn(conn, "apikey", "key_money_ok")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	hasCreateDate, err := hasColumn(conn, "apikey", "createdate")
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}

	selectCols := []string{
		"`id`",
		"`website`",
		"`key_check_money`",
	}
	if hasMoney {
		selectCols = append(selectCols, "`money`")
	} else {
		selectCols = append(selectCols, "'' AS `money`")
	}
	if hasCreateDate {
		selectCols = append(selectCols, "CAST(`createdate` AS CHAR) AS `createdate`")
	} else {
		selectCols = append(selectCols, "'' AS `createdate`")
	}
	if hasKeyMoneyOK {
		selectCols = append(selectCols, "`key_money_ok`")
	} else {
		selectCols = append(selectCols, "'' AS `key_money_ok`")
	}

	query := "SELECT " + strings.Join(selectCols, ", ") + " FROM `apikey` WHERE `key_check_money` = 1 ORDER BY `id` ASC"

	rows, err := conn.Query(query)
	if err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}
	defer rows.Close()

	list := make([]moneyListItem, 0, 32)
	for rows.Next() {
		var id sql.NullInt64
		var website sql.NullString
		var keyCheck sql.NullInt64
		var money sql.NullString
		var createDate sql.NullString
		var keyMoneyOK sql.NullString

		if err := rows.Scan(&id, &website, &keyCheck, &money, &createDate, &keyMoneyOK); err != nil {
			api.Print_json(c, "status", "0", "message", err.Error())
			return
		}

		list = append(list, moneyListItem{
			ID:            id.Int64,
			Website:       website.String,
			KeyCheckMoney: keyCheck.Int64,
			Money:         money.String,
			KeyMoneyOK:    keyMoneyOK.String,
			CreateDate:    createDate.String,
		})
	}
	if err := rows.Err(); err != nil {
		api.Print_json(c, "status", "0", "message", err.Error())
		return
	}

	api.Print_json(
		c,
		"status", "1",
		"message", "ok",
		"count", len(list),
		"has_money", hasMoney,
		"has_key_money_ok", hasKeyMoneyOK,
		"has_createdate", hasCreateDate,
		"list", list,
	)
}
