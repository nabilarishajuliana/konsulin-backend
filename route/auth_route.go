
package route

import (
	"konsulin-backend/app/repository"
	"konsulin-backend/app/service"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
)

func registerAuthRoutes(api fiber.Router, deps Dependencies) {
	authRepo := repository.NewAuthRepository(deps.Pool)

	authService := service.NewAuthService(
		authRepo,
		deps.JWT,
		deps.Tx,
	)

	auth := api.Group("/auth")
	auth.Post("/register", authService.Register)
	auth.Post("/login", authService.Login)
	auth.Post("/refresh", authService.Refresh)
	auth.Get(
		"/me",
		middleware.RequireAuth(deps.JWT),
		authService.Me,
	)
	auth.Post("/logout", authService.Logout)
}
