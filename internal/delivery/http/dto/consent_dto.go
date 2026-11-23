package dto

import (
	"time"

	domainconsent "crm-core/domain/consent"
)

// ConsentDocumentRequest creates or edits a consent document version.
type ConsentDocumentRequest struct {
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	PDFURL      string     `json:"pdfUrl"`
	IsRequired  bool       `json:"isRequired"`
	EffectiveAt *time.Time `json:"effectiveAt"`
	IsActive    bool       `json:"isActive"`
}

// ConsentActiveRequest turns a version on or off.
type ConsentActiveRequest struct {
	IsActive bool `json:"isActive"`
}

// ConsentDocumentResponse is one consent document version.
type ConsentDocumentResponse struct {
	ID          uint      `json:"id"`
	Type        string    `json:"type"`
	Version     int       `json:"version"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	PDFURL      string    `json:"pdfUrl"`
	IsRequired  bool      `json:"isRequired"`
	EffectiveAt time.Time `json:"effectiveAt"`
	IsActive    bool      `json:"isActive"`
	CreatedBy   uint      `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ToConsentDocumentResponse maps a document version.
func ToConsentDocumentResponse(d domainconsent.Document) ConsentDocumentResponse {
	return ConsentDocumentResponse{
		ID: d.ID, Type: d.Type, Version: d.Version, Title: d.Title, Content: d.Content, PDFURL: d.PDFURL,
		IsRequired: d.IsRequired, EffectiveAt: d.EffectiveAt, IsActive: d.IsActive,
		CreatedBy: d.CreatedBy, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}
