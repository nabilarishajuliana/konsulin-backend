package route

import (
	"konsulin-backend/app/repository"
	"konsulin-backend/app/service"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
)

func registerSlotBookingRoutes(api fiber.Router, deps Dependencies) {
	slotRepo := repository.NewSlotRepository(deps.Pool)
	bookingRepo := repository.NewBookingRepository(deps.Pool)

	slotSvc := service.NewSlotService(slotRepo, bookingRepo, deps.Tx)
	bookingSvc := service.NewBookingService(bookingRepo, slotRepo, deps.Tx)

	auth := middleware.RequireAuth(deps.JWT)
	perm := func(p string) fiber.Handler {
		return middleware.RequirePermission(deps.Permissions, p)
	}

	slots := api.Group("/slots", auth)
	slots.Get("", perm("slot:list"), slotSvc.ListAvailable)
	slots.Post("", perm("slot:create"), slotSvc.Create)
	slots.Get("/:id", perm("slot:read"), slotSvc.GetByID)
	slots.Get("/:id/bookings", perm("slot:read_bookers"), slotSvc.GetSlotBookings)
	slots.Patch("/:id/capacity", perm("slot:update"), slotSvc.UpdateCapacity)
	slots.Delete("/:id", perm("slot:delete"), slotSvc.Close)
	slots.Post("/:id/book", perm("booking:create"), bookingSvc.Book)
	slots.Delete("/:id/book", perm("booking:cancel"), bookingSvc.Cancel)

	bookings := api.Group("/bookings", auth)
	bookings.Get("/mine", perm("booking:read_own"), bookingSvc.GetMine)
	bookings.Post("/swap", perm("booking:swap"), bookingSvc.Swap)
	bookings.Patch("/:id/confirm", perm("booking:confirm"), bookingSvc.Confirm)
	bookings.Patch("/:id/status", perm("booking:attendance"), bookingSvc.UpdateStatus)
}
