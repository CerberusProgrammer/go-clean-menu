package main

import "github.com/gofiber/fiber/v2"

// RenderPage renders a template with the main layout
func RenderPage(c *fiber.Ctx, template string, data fiber.Map) error {
	// Add common data for all pages
	if c.Locals("CurrentTime") != nil {
		data["CurrentTime"] = c.Locals("CurrentTime")
	}
	return c.Render(template, data)
}

// RenderPartial renders a template without layout (for HTMX updates)
func RenderPartial(c *fiber.Ctx, template string, data fiber.Map) error {
	return c.Render(template, data, "")
}

// Success sends success response with toast and optional modal closing
func Success(c *fiber.Ctx, message string, closeModal bool) error {
	trigger := `{"showToast": "` + message + `"`
	if closeModal {
		trigger += `, "closeModal": true`
	}
	trigger += `}`

	c.Set("HX-Trigger", trigger)
	return nil
}

// Error sends error response with toast
func Error(c *fiber.Ctx, message string, statusCode int) error {
	c.Set("HX-Trigger", `{"showToast": "`+message+`"}`)
	return c.Status(statusCode).SendString(message)
}
