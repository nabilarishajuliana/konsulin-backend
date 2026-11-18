package helper

import "github.com/gofiber/fiber/v2"

func Success(c *fiber.Ctx, status int, message string, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func Fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"code":    code,
		"message": message,
	})
}

func ErrorResponse(c *fiber.Ctx, status int, code, message string, errs map[string]string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"code":    code,
		"message": message,
		"errors":  errs,
	})
}
