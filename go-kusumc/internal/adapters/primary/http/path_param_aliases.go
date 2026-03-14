package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func pathParamAlias(ctx *fiber.Ctx, keys ...string) string {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		val := strings.TrimSpace(ctx.Params(key))
		if val != "" {
			return val
		}
	}
	return ""
}
