package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type JsonEncode map[string]any

func J(pairs ...any) JsonEncode {
	out := JsonEncode{}
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = pairs[i+1]
	}
	return out
}

func MakePrintJSON(c *gin.Context) func(payload any, status ...int) {
	return func(payload any, status ...int) {
		if c == nil {
			return
		}
		code := http.StatusOK
		if len(status) > 0 {
			code = status[0]
		}
		c.JSON(code, payload)
	}
}
