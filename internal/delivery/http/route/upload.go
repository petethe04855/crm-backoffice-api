package route

import (
	"backoffice-api/internal/authz"
	"backoffice-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

// registerUploads mounts the image upload used by promotions and rewards.
//
//	POST /api/v1/crm/uploads/images   promotions.manage OR rewards.manage
func registerUploads(crm fiber.Router, h Handlers) {
	if h.Upload == nil {
		return
	}
	crm.Post(
		"/uploads/images",
		middleware.RequireAnyPermission(authz.PromotionsManage, authz.RewardsManage),
		h.Upload.UploadImage,
	)
}

// registerFiles serves an uploaded image by uuid.
//
// The route is public on purpose: <img> tags cannot send the staff bearer
// token, and the uuid is an unguessable key. It exposes no listing and no
// personal data, only the bytes the backoffice itself stored.
//
//	GET /api/v1/files/:id
func registerFiles(api fiber.Router, h Handlers) {
	if h.Upload == nil {
		return
	}
	api.Get("/files/:id", h.Upload.Serve)
}
