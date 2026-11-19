package handler

import (
	"bytes"
	"errors"
	"io"
	"log"
	"time"

	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/internal/storage"
	"backoffice-api/pkg/response"

	"crm-core/domain"
	domainupload "crm-core/domain/upload"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// UploadHandler stores the images used by promotions and rewards, and serves
// them back by id. Only the metadata is in Postgres; the bytes live in a
// storage backend.
type UploadHandler struct {
	files domainupload.Repository
	store storage.Store
	// newID and now are seams for tests.
	newID func() string
	now   func() time.Time
}

// NewUploadHandler builds the handler.
func NewUploadHandler(files domainupload.Repository, store storage.Store) *UploadHandler {
	return &UploadHandler{
		files: files,
		store: store,
		newID: func() string { return uuid.NewString() },
		now:   time.Now,
	}
}

// UploadedImageResponse is the body of a successful upload.
type UploadedImageResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// UploadImage serves POST /api/v1/crm/uploads/images.
//
// The multipart field is "file". The type is decided from the magic bytes, not
// from the file name or the client's Content-Type; only jpg, png and webp up to
// 2 MB are accepted.
func (h *UploadHandler) UploadImage(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "multipart field \"file\" is required", "VALIDATION_ERROR")
	}
	opened, err := file.Open()
	if err != nil {
		log.Printf("[ERROR] open upload %q: %v", file.Filename, err)
		return response.InternalServerError(c, "Cannot read the uploaded file")
	}
	data, err := io.ReadAll(io.LimitReader(opened, domainupload.MaxImageBytes+1))
	opened.Close()
	if err != nil {
		log.Printf("[ERROR] read upload %q: %v", file.Filename, err)
		return response.InternalServerError(c, "Cannot read the uploaded file")
	}
	if len(data) == 0 {
		return response.BadRequest(c, "the uploaded file is empty", "VALIDATION_ERROR")
	}
	if int64(len(data)) > domainupload.MaxImageBytes {
		return response.Error(c, fiber.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
			"Image must be 2 MB or smaller")
	}
	mime := sniffImage(data)
	if mime == "" {
		return response.BadRequest(c, "Only jpg, png and webp images are accepted", "UNSUPPORTED_MEDIA_TYPE")
	}

	id := h.newID()
	url, err := h.store.Save(c.UserContext(), id, bytes.NewReader(data))
	if err != nil {
		log.Printf("[ERROR] store upload %s: %v", id, err)
		return response.InternalServerError(c, "Cannot store the uploaded file")
	}

	record := &domainupload.UploadedFile{
		ID:         id,
		Path:       id,
		MIME:       mime,
		Size:       int64(len(data)),
		UploadedBy: actor.Email,
		CreatedAt:  h.now().UTC(),
	}
	if err := h.files.Create(c.UserContext(), record); err != nil {
		// The bytes are already on disk; log the orphan so it can be cleaned.
		log.Printf("[ERROR] save upload metadata %s (orphan file): %v", id, err)
		return response.InternalServerError(c, "Cannot store the uploaded file")
	}
	return response.Created(c, UploadedImageResponse{ID: id, URL: url}, "Uploaded")
}

// Serve streams a stored image by id.
//
// The route is public: <img> tags cannot send a bearer token and the uuid is
// the unguessable key. It never lists files and never exposes buyer data.
func (h *UploadHandler) Serve(c *fiber.Ctx) error {
	record, err := h.files.FindByID(c.UserContext(), c.Params("id"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return response.NotFound(c, "File not found")
		}
		log.Printf("[ERROR] load upload %q: %v", c.Params("id"), err)
		return response.InternalServerError(c, "Cannot read the file right now")
	}

	rc, err := h.store.Open(c.UserContext(), record.Path)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return response.NotFound(c, "File not found")
		}
		log.Printf("[ERROR] open upload %q: %v", record.ID, err)
		return response.InternalServerError(c, "Cannot read the file right now")
	}
	defer rc.Close()

	// Images are capped at 2 MB, so reading the blob here keeps the response
	// simple and lets Fiber send a correct Content-Length.
	data, err := io.ReadAll(io.LimitReader(rc, domainupload.MaxImageBytes+1))
	if err != nil {
		log.Printf("[ERROR] read upload %q: %v", record.ID, err)
		return response.InternalServerError(c, "Cannot read the file right now")
	}

	c.Set(fiber.HeaderContentType, record.MIME)
	// The id is immutable content: the same URL always returns the same bytes.
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(data)
}

// sniffImage returns the MIME type of the first bytes, or "" when the payload
// is not a supported image.
func sniffImage(data []byte) string {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return domainupload.MIMEJPEG
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return domainupload.MIMEPNG
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return domainupload.MIMEWebP
	}
	return ""
}
