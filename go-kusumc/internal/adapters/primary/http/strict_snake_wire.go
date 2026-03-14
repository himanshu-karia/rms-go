package http

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const strictSnakeWireEnv = "STRICT_SNAKE_WIRE"

func StrictSnakeWireEnabled() bool {
	val := strings.TrimSpace(os.Getenv(strictSnakeWireEnv))
	if val == "" {
		return false
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}

func StrictSnakeWireMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !StrictSnakeWireEnabled() {
			return c.Next()
		}

		// Only enforce on our API surface; other routes may carry third-party payloads.
		path := c.Path()
		if !(strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/api/v1")) {
			return c.Next()
		}
		// Third-party webhook payloads may legitimately include camelCase keys.
		if strings.HasPrefix(path, "/api/northbound/") {
			return c.Next()
		}

		allowReactFlowOpaque := strings.Contains(path, "/config/automation")

		// 1) Query params: reject any camelCase keys.
		var badQueryKey string
		c.Context().QueryArgs().VisitAll(func(k, _ []byte) {
			if badQueryKey != "" {
				return
			}
			key := string(k)
			if containsUpperASCII(key) {
				badQueryKey = key
			}
		})
		if badQueryKey != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "camelCase query keys are not allowed; use snake_case",
				"issues":  []fiber.Map{{"field": badQueryKey, "message": "use snake_case"}},
			})
		}

		// 2) JSON body keys (top-level and nested) for write requests.
		method := strings.ToUpper(c.Method())
		switch method {
		case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch:
			// continue
		default:
			return c.Next()
		}

		if !strings.Contains(strings.ToLower(c.Get(fiber.HeaderContentType)), "application/json") {
			return c.Next()
		}

		body := c.Body()
		if len(body) == 0 {
			return c.Next()
		}

		var decoded any
		if err := json.Unmarshal(body, &decoded); err != nil {
			// Leave JSON validation to the handler.
			return c.Next()
		}

		if keyPath := firstCamelKeyPath(decoded, allowReactFlowOpaque); keyPath != "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "camelCase JSON keys are not allowed; use snake_case",
				"issues":  []fiber.Map{{"field": keyPath, "message": "use snake_case"}},
			})
		}

		return c.Next()
	}
}

func containsUpperASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'A' && b <= 'Z' {
			return true
		}
	}
	return false
}

func firstCamelKeyPath(value any, allowReactFlowOpaque bool) string {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if containsUpperASCII(key) {
				return key
			}
			if allowReactFlowOpaque && (key == "nodes" || key == "edges") {
				continue
			}
			if childPath := firstCamelKeyPath(child, allowReactFlowOpaque); childPath != "" {
				return key + "." + childPath
			}
		}
	case []any:
		for i := 0; i < len(v); i++ {
			if childPath := firstCamelKeyPath(v[i], allowReactFlowOpaque); childPath != "" {
				return "[" + strconvItoa(i) + "]." + childPath
			}
		}
	}
	return ""
}

// tiny int->string helper to avoid fmt import in hot path
func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
