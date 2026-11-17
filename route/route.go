package route

import (
	"konsulin-backend/helper"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type Dependencies struct {
	App           *fiber.App
	Pool          *pgxpool.Pool
	Mongo         *mongo.Database
	JWT           *helper.JWTManager
	Permissions   *helper.PermissionSet
	Tx            *helper.TxManager
	EnableSwagger bool
}

func Register(deps Dependencies) {
	deps.App.Use(middleware.RateLimiter())
	deps.App.Get("/api/v1/health", healthCheck)

	api := deps.App.Group("/api/v1", middleware.RequireJSON)

	// Diaktifkan setelah modul masing-masing siap:
	// registerAuthRoutes(api, deps)
	registerSlotBookingRoutes(api, deps)
	// registerNoteRoutes(api, deps)
}

func healthCheck(c *fiber.Ctx) error {
	return helper.Success(c, fiber.StatusOK, "server berjalan", nil)
}