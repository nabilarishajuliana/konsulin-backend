package middleware

import (
	"strings"

	"konsulin-backend/helper"

	"github.com/gofiber/fiber/v2"
)

func RequireJSON(c *fiber.Ctx) error {
	m := c.Method()
	if m == fiber.MethodPost || m == fiber.MethodPut || m == fiber.MethodPatch {
		if !strings.Contains(c.Get("Content-Type"), "application/json") {
			return helper.Fail(c, fiber.StatusUnsupportedMediaType, helper.CodeValidation, "Content-Type harus application/json")
		}
	}
	return c.Next()
}