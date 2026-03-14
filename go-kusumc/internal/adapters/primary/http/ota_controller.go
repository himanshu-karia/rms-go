package http

import (
	"io"

	"github.com/gofiber/fiber/v2"
)

type otaService interface {
	UploadFirmware(filename string, content []byte) (string, error)
	StartCampaign(name, version, s3URL, checksum, projectType string) error
}

type OtaController struct {
	ota otaService
}

func NewOtaController(ota otaService) *OtaController {
	return &OtaController{ota: ota}
}

func (c *OtaController) UploadFirmware(ctx *fiber.Ctx) error {
	file, err := ctx.FormFile("firmware")
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "No file uploaded"})
	}

	f, _ := file.Open()
	defer f.Close()
	buf, _ := io.ReadAll(f)

	url, err := c.ota.UploadFirmware(file.Filename, buf)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"url": url})
}

func (c *OtaController) StartCampaign(ctx *fiber.Ctx) error {
	var body struct {
		Name             string `json:"name"`
		Version          string `json:"version"`
		S3URL            string `json:"s3_url"`
		S3URLCamel       string `json:"s3Url"`
		Checksum         string `json:"checksum"`
		ProjectType      string `json:"project_type"`
		ProjectTypeCamel string `json:"projectType"`
	}

	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}

	if body.S3URL == "" {
		body.S3URL = body.S3URLCamel
	}
	if body.ProjectType == "" {
		body.ProjectType = body.ProjectTypeCamel
	}
	err := c.ota.StartCampaign(body.Name, body.Version, body.S3URL, body.Checksum, body.ProjectType)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"status": "campaign_started"})
}
