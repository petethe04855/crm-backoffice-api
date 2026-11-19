package route

import "github.com/gofiber/fiber/v2"

// registerHealth mounts the unauthenticated liveness probe.
//
//	GET /api/v1/health
func registerHealth(api fiber.Router, h Handlers) {
	api.Get("/health", h.Health.Health)
}
