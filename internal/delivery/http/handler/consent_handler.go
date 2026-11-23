package handler

import (
	"errors"

	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	usecaseconsent "crm-core/usecase/consent"

	"github.com/gofiber/fiber/v2"
)

// ConsentHandler manages the PDPA consent documents shown in the LINE OA.
type ConsentHandler struct {
	consents usecaseconsent.Usecase
}

// NewConsentHandler builds the handler.
func NewConsentHandler(consents usecaseconsent.Usecase) *ConsentHandler {
	return &ConsentHandler{consents: consents}
}

func (h *ConsentHandler) writeError(c *fiber.Ctx, err error) error {
	if errors.Is(err, usecaseconsent.ErrDocumentInUse) {
		return response.Error(c, fiber.StatusConflict, "CONSENT_DOCUMENT_IN_USE",
			"Members already answered this version; publish a new version instead")
	}
	return writeError(c, err)
}

func toInput(req dto.ConsentDocumentRequest) usecaseconsent.DocumentInput {
	return usecaseconsent.DocumentInput{
		Type: req.Type, Title: req.Title, Content: req.Content, PDFURL: req.PDFURL,
		IsRequired: req.IsRequired, EffectiveAt: req.EffectiveAt, IsActive: req.IsActive,
	}
}

// List serves GET /consent-documents (every version, newest first per type).
func (h *ConsentHandler) List(c *fiber.Ctx) error {
	docs, err := h.consents.ListDocuments(c.UserContext())
	if err != nil {
		return h.writeError(c, err)
	}
	out := make([]dto.ConsentDocumentResponse, 0, len(docs))
	for _, d := range docs {
		out = append(out, dto.ToConsentDocumentResponse(d))
	}
	return response.OK(c, out)
}

// Create serves POST /consent-documents: a new version of the type.
func (h *ConsentHandler) Create(c *fiber.Ctx) error {
	var req dto.ConsentDocumentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	actor, _ := middleware.ActorFrom(c)
	doc, err := h.consents.CreateDocument(c.UserContext(), actor, toInput(req))
	if err != nil {
		return h.writeError(c, err)
	}
	return response.Created(c, dto.ToConsentDocumentResponse(*doc), "Consent document created")
}

// Update serves PUT /consent-documents/:id (only before anyone answered it).
func (h *ConsentHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req dto.ConsentDocumentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	doc, err := h.consents.UpdateDocument(c.UserContext(), id, toInput(req))
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, dto.ToConsentDocumentResponse(*doc), "Consent document updated")
}

// SetActive serves PATCH /consent-documents/:id/active.
func (h *ConsentHandler) SetActive(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req dto.ConsentActiveRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	doc, err := h.consents.SetActive(c.UserContext(), id, req.IsActive)
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, dto.ToConsentDocumentResponse(*doc), "Consent document updated")
}
