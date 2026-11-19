package handler

import (
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	usecasestaffrole "crm-core/usecase/staffrole"

	"github.com/gofiber/fiber/v2"
)

// StaffRoleHandler serves CRM role grants (owner only).
type StaffRoleHandler struct {
	roles usecasestaffrole.Usecase
}

// NewStaffRoleHandler builds the handler.
func NewStaffRoleHandler(roles usecasestaffrole.Usecase) *StaffRoleHandler {
	return &StaffRoleHandler{roles: roles}
}

// List serves GET /staff-roles.
func (h *StaffRoleHandler) List(c *fiber.Ctx) error {
	grants, err := h.roles.List(c.UserContext())
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.StaffRoleResponse, 0, len(grants))
	for _, g := range grants {
		out = append(out, dto.ToStaffRoleResponse(g))
	}
	return response.OK(c, out)
}

// Grant serves PUT /staff-roles/:erpUserId.
func (h *StaffRoleHandler) Grant(c *fiber.Ctx) error {
	id, err := parseID(c.Params("erpUserId"), "erpUserId")
	if err != nil {
		return writeError(c, err)
	}
	var req dto.GrantStaffRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	actor, _ := middleware.ActorFrom(c)
	grant, err := h.roles.Grant(c.UserContext(), actor, id, req.Email, req.Role)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToStaffRoleResponse(*grant), "Role granted")
}

// Revoke serves DELETE /staff-roles/:erpUserId.
func (h *StaffRoleHandler) Revoke(c *fiber.Ctx) error {
	id, err := parseID(c.Params("erpUserId"), "erpUserId")
	if err != nil {
		return writeError(c, err)
	}
	actor, _ := middleware.ActorFrom(c)
	if err := h.roles.Revoke(c.UserContext(), actor, id); err != nil {
		return writeError(c, err)
	}
	return response.OK(c, fiber.Map{"erpUserId": id}, "Role revoked")
}
