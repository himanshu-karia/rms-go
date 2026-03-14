package http

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func queryAlias(ctx *fiber.Ctx, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(ctx.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func queryAliasDefault(ctx *fiber.Ctx, defaultValue string, keys ...string) string {
	if value := queryAlias(ctx, keys...); value != "" {
		return value
	}
	return defaultValue
}

func queryAliasInt(ctx *fiber.Ctx, defaultValue int, keys ...string) int {
	for _, key := range keys {
		if raw := strings.TrimSpace(ctx.Query(key)); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				return parsed
			}
			return defaultValue
		}
	}
	return defaultValue
}

func queryAliasBool(ctx *fiber.Ctx, defaultValue bool, keys ...string) bool {
	for _, key := range keys {
		if raw := strings.TrimSpace(ctx.Query(key)); raw != "" {
			return strings.EqualFold(raw, "true") || raw == "1"
		}
	}
	return defaultValue
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseInt64(raw string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}
