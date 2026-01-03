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

func PrintJSON(c *gin.Context, payload any, status ...int) {
	if c == nil {
		return
	}
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	c.JSON(code, payload)
}

func Print_json(c *gin.Context, args ...any) {
	if c == nil {
		return
	}
	if len(args) == 0 {
		PrintJSON(c, J())
		return
	}
	if len(args) == 1 {
		PrintJSON(c, args[0])
		return
	}
	if _, ok := args[0].(string); ok {
		status := 0
		if len(args)%2 == 1 {
			if code, ok := args[len(args)-1].(int); ok {
				status = code
				args = args[:len(args)-1]
			}
		}
		if status != 0 {
			PrintJSON(c, J(args...), status)
			return
		}
		PrintJSON(c, J(args...))
		return
	}
	payload := args[0]
	if len(args) > 1 {
		if code, ok := args[1].(int); ok {
			PrintJSON(c, payload, code)
			return
		}
	}
	PrintJSON(c, payload)
}

func Print(c *gin.Context, pairs ...any) {
	Print_json(c, pairs...)
}

func MakePrint(c *gin.Context) func(pairs ...any) {
	return func(pairs ...any) {
		Print_json(c, pairs...)
	}
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
