package handler

import (
	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"crm-core/domain"

	"github.com/gofiber/fiber/v2"
)

// Update serves PATCH /members/:id (suspend / reactivate).
func (h *MemberHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.UpdateMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	if req.Status == nil {
		return writeError(c, domain.NewValidationError("status", "is required"))
	}
	m, err := h.members.SetStatus(c.UserContext(), actor, id, *req.Status)
	if err != nil {
		return writeError(c, err)
	}
	d, err := h.members.Get(c.UserContext(), m.ID)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToMemberDetail(d), "Member updated")
}

// UnlinkLine serves DELETE /members/:id/line.
func (h *MemberHandler) UnlinkLine(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	if err := h.members.UnlinkLine(c.UserContext(), actor, id); err != nil {
		return writeError(c, err)
	}
	return response.OK(c, fiber.Map{"memberId": id}, "LINE account unlinked")
}

// Tags serves GET /members/:id/tags.
func (h *MemberHandler) Tags(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	tags, err := h.members.Tags(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToMemberTagResponses(tags))
}

// AddTag serves POST /members/:id/tags.
func (h *MemberHandler) AddTag(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.AddTagRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	tags, err := h.members.AddTag(c.UserContext(), actor, id, req.Tag)
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToMemberTagResponses(tags), "Tag added")
}

// RemoveTag serves DELETE /members/:id/tags/:tag.
func (h *MemberHandler) RemoveTag(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	tags, err := h.members.RemoveTag(c.UserContext(), actor, id, c.Params("tag"))
	if err != nil {
		return writeError(c, err)
	}
	return response.OK(c, dto.ToMemberTagResponses(tags), "Tag removed")
}

// Notes serves GET /members/:id/notes.
func (h *MemberHandler) Notes(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	page, err := parseOptionalInt(c.Query("page"), "page")
	if err != nil {
		return writeError(c, err)
	}
	limit, err := parseOptionalInt(c.Query("limit"), "limit")
	if err != nil {
		return writeError(c, err)
	}
	notes, total, err := h.members.Notes(c.UserContext(), id, page, limit)
	if err != nil {
		return writeError(c, err)
	}
	out := make([]dto.MemberNoteResponse, 0, len(notes))
	for _, n := range notes {
		out = append(out, dto.ToMemberNoteResponse(n))
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return response.List(c, out, page, limit, total)
}

// AddNote serves POST /members/:id/notes.
func (h *MemberHandler) AddNote(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return writeError(c, err)
	}
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}
	var req dto.CreateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	note, err := h.members.AddNote(c.UserContext(), actor, id, req.Body)
	if err != nil {
		return writeError(c, err)
	}
	return response.Created(c, dto.ToMemberNoteResponse(*note))
}
