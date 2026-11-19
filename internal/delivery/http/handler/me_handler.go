package handler

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// Me serves GET /api/v1/crm/me: the actor with effective roles and permissions,
// used by backoffice-web to build its menu.
func Me(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	return response.OK(c, dto.MeResponse{
		ERPUserID:   actor.ERPUserID,
		Email:       actor.Email,
		Roles:       actor.Roles,
		Permissions: authz.PermissionsFor(actor.Roles),
	})
}
