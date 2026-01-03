package profile

import (
	"net/http"

	"hust_backend/main/api"
	"hust_backend/main/database"
	"hust_backend/main/play_sql"

	"github.com/gin-gonic/gin"
)

func ListPlanHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusOK)
		return
	}

	print_json := api.MakePrintJSON(c)
	rows, err := queryPlanRows()
	if err != nil {
		print_json([]api.JsonEncode{})
		return
	}

	data := make([]api.JsonEncode, 0, len(rows))
	for _, row := range rows {
		data = append(data, api.J(
			"id", row["id"],
			"level", row["namelevel"],
			"soxunap", row["soxunap"],
			"thucnhan", row["thucnhan"],
			"tongxunhan", row["tongxunhan"],
			"chietkhau", row["chietkhaugiam"],
			"stt", row["id"],
		))
	}

	print_json(data)
}

func queryPlanRows() ([]map[string]string, error) {
	conn, err := database.Open()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query("SELECT * FROM `capdotaikhoan`")
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
			row[col] = play_sql.ToString(values[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
