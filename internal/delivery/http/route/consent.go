package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerConsent mounts the LINE OA consent documents (PDPA).
//
//	GET   /api/v1/crm/consent-documents              consents.manage
//	POST  /api/v1/crm/consent-documents              consents.manage
//	PUT   /api/v1/crm/consent-documents/:id          consents.manage
//	PATCH /api/v1/crm/consent-documents/:id/active   consents.manage
func registerConsent(crm fiber.Router, h Handlers) {
	if h.Consent == nil {
		return
	}
	docs := crm.Group("/consent-documents", middleware.RequirePermission(authz.ConsentsManage))
	docs.Get("/", h.Consent.List)
	docs.Post("/", h.Consent.Create)
	docs.Put("/:id", h.Consent.Update)
	docs.Patch("/:id/active", h.Consent.SetActive)
}
