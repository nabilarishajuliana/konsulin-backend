package middleware

import (
	"strings"

	"konsulin-backend/helper"

	"github.com/gofiber/fiber/v2"
)

const ctxUserKey = "currentUser"

func RequireAuth(jwt *helper.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authz := c.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak ditemukan")
		}
		user, err := jwt.Parse(strings.TrimPrefix(authz, "Bearer "))
		if err != nil {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid")
		}
		c.Locals(ctxUserKey, user)
		return c.Next()
	}
}

func CurrentUser(c *fiber.Ctx) (helper.AuthUser, bool) {
	u, ok := c.Locals(ctxUserKey).(helper.AuthUser)
	return u, ok
}
