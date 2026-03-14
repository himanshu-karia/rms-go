package http

import (
	"ingestion-go/internal/core/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type AnalyticsController struct {
	service *services.AnalyticsService
}

func NewAnalyticsController(service *services.AnalyticsService) *AnalyticsController {
	return &AnalyticsController{service: service}
}

func (c *AnalyticsController) GetHistory(ctx *fiber.Ctx) error {
	device := analyticsDeviceQuery(ctx)
	if device == "" {
		return ctx.Status(400).SendString("Missing device or imei")
	}

	packetType := analyticsPacketTypeQuery(ctx)
	startStr := analyticsStartQuery(ctx)
	endStr := analyticsEndQuery(ctx)

	// Defaults
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if startStr != "" {
		if t, err := parseFlexibleTime(startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := parseFlexibleTime(endStr); err == nil {
			end = t
		}
	}

	page, _ := strconv.Atoi(analyticsPageQuery(ctx))
	limit, _ := strconv.Atoi(analyticsLimitQuery(ctx))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	offset := (page - 1) * limit

	data, err := c.service.GetHistory(device, packetType, start, end, limit, offset)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *AnalyticsController) GetLatest(ctx *fiber.Ctx) error {
	device := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if device == "" {
		device = ctx.Params("imei")
	}
	if device == "" {
		device = analyticsDeviceQuery(ctx)
	}
	if device == "" {
		return ctx.Status(400).SendString("Missing device or imei")
	}
	packetType := analyticsPacketTypeQuery(ctx)
	item, err := c.service.GetLatest(device, packetType)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if item == nil {
		return ctx.SendStatus(204)
	}
	return ctx.JSON(normalizeToSnakeKeys(item))
}

func (c *AnalyticsController) GetProjectHistory(ctx *fiber.Ctx) error {
	projectId := analyticsProjectQuery(ctx)
	if projectId == "" {
		return ctx.Status(400).SendString("Missing project_id")
	}

	packetType := analyticsPacketTypeQuery(ctx)
	quality := analyticsQualityQuery(ctx)
	excludeQuality := analyticsExcludeQualityQuery(ctx)
	startStr := analyticsStartQuery(ctx)
	endStr := analyticsEndQuery(ctx)

	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if startStr != "" {
		if t, err := parseFlexibleTime(startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := parseFlexibleTime(endStr); err == nil {
			end = t
		}
	}

	page, _ := strconv.Atoi(analyticsPageQuery(ctx))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(analyticsLimitQuery(ctx))
	if limit <= 0 {
		limit = 50
	}
	offset := (page - 1) * limit

	rows, total, err := c.service.GetHistoryByProject(projectId, packetType, quality, excludeQuality, start, end, limit, offset)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	pages := 1
	if total > 0 {
		pages = (total + limit - 1) / limit
	}

	return ctx.JSON(fiber.Map{
		"data":  normalizeToSnakeKeys(rows),
		"total": total,
		"page":  page,
		"pages": pages,
	})
}

func parseFlexibleTime(val string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04", val)
}

func (c *AnalyticsController) GetAnomalies(ctx *fiber.Ctx) error {
	list, err := c.service.GetAnomalies(50) // Need to add to service first!
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(list))
}

func analyticsDeviceQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "device", "device_id", "deviceId", "device_uuid", "deviceUuid", "imei")
}

func analyticsPacketTypeQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "packet_type", "packetType")
}

func analyticsProjectQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func analyticsQualityQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "quality", "data_quality")
}

func analyticsExcludeQualityQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "exclude_quality", "excludeQuality")
}

func analyticsStartQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "from", "start")
}

func analyticsEndQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "to", "end")
}

func analyticsPageQuery(ctx *fiber.Ctx) string {
	return queryAliasDefault(ctx, "1", "page", "pageNumber")
}

func analyticsLimitQuery(ctx *fiber.Ctx) string {
	return queryAliasDefault(ctx, "50", "limit", "pageSize")
}
