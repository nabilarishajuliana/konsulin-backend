package route

import (
	"konsulin-backend/app/repository"
	"konsulin-backend/app/service"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
)

func registerNoteRoutes(api fiber.Router, deps Dependencies) {
	noteRepo := repository.NewNoteRepository(deps.Mongo)

	noteService := service.NewNoteService(
		noteRepo,
		deps.Pool,
	)

	auth := middleware.RequireAuth(deps.JWT)

	api.Get(
		"/bookings/:bookingID/notes",
		auth,
		middleware.RequirePermission(deps.Permissions, "note:read"),
		noteService.Get,
	)

	api.Put(
		"/bookings/:bookingID/notes",
		auth,
		middleware.RequirePermission(deps.Permissions, "note:write"),
		noteService.Save,
	)
}
