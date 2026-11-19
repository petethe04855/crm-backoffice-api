package handler

import (
	"context"
	"log"
	"time"

	"backoffice-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Pinger checks a dependency (the CRM database).
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthHandler serves GET /api/v1/health.
type HealthHandler struct {
	db Pinger
}

// NewHealthHandler builds the health handler.
func NewHealthHandler(db Pinger) *HealthHandler { return &HealthHandler{db: db} }

// Health reports service and database status.
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		log.Printf("[WARN] health: database ping failed: %v", err)
		return response.Error(c, fiber.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Database is not reachable")
	}
	return response.OK(c, fiber.Map{"service": "backoffice-api", "status": "ok"})
}
