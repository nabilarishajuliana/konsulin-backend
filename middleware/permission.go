package middleware

import (
	"konsulin-backend/helper"

	"github.com/gofiber/fiber/v2"
)

func RequirePermission(perms *helper.PermissionSet, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := CurrentUser(c)
		if !ok {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "belum login")
		}
		if !perms.Has(user.Role, permission) {
			return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "akses ditolak")
		}
		return c.Next()
	}
}
