package handler

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"backoffice-api/internal/delivery/http/dto"
	"backoffice-api/internal/delivery/http/export"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/pkg/response"

	"crm-core/domain"
	domainproductcode "crm-core/domain/productcode"
	usecaseproductcode "crm-core/usecase/productcode"

	"github.com/gofiber/fiber/v2"
)

// ProductCodeHandler manages packaging-code batches (L4B), the member-side
// limits, flagged members and test members (L4).
type ProductCodeHandler struct {
	batches usecaseproductcode.Usecase
	members usecaseproductcode.Redeemer
}

// NewProductCodeHandler builds the handler. members serves the L4 routes;
// nil leaves them unmounted.
func NewProductCodeHandler(batches usecaseproductcode.Usecase, members usecaseproductcode.Redeemer) *ProductCodeHandler {
	return &ProductCodeHandler{batches: batches, members: members}
}

// HasMembers reports whether the member-side routes can be served.
func (h *ProductCodeHandler) HasMembers() bool { return h.members != nil }

func (h *ProductCodeHandler) writeError(c *fiber.Ctx, err error) error {
	conflict := func(code, msg string) error { return response.Error(c, fiber.StatusConflict, code, msg) }
	switch {
	case errors.Is(err, domainproductcode.ErrNotDraft):
		return conflict("BATCH_NOT_DRAFT", "Only draft batches can be edited")
	case errors.Is(err, domainproductcode.ErrAlreadyExported):
		return conflict("BATCH_ALREADY_EXPORTED", "Codes were already exported once; the full codes are no longer stored")
	case errors.Is(err, domainproductcode.ErrNotExported):
		return conflict("BATCH_NOT_EXPORTED", "Export the print file before activating")
	case errors.Is(err, domainproductcode.ErrLotRequired):
		return conflict("LOT_REQUIRED", "Enter the product lot number before activating (test batches excepted)")
	case errors.Is(err, domainproductcode.ErrExpired):
		return conflict("BATCH_EXPIRED", "The code expiry date has passed")
	case errors.Is(err, domainproductcode.ErrAlreadyVoid):
		return conflict("BATCH_VOID", "The batch is void")
	case errors.Is(err, domainproductcode.ErrNotConfigured):
		return response.Error(c, fiber.StatusServiceUnavailable, "PRODUCT_CODES_NOT_CONFIGURED", "PRODUCT_CODE_HMAC_SECRET is not set on backoffice-api")
	}
	return writeError(c, err)
}

// parseDay reads YYYY-MM-DD in Bangkok. endOfDay returns 23:59:59 of that day.
func parseDay(v, field string, endOfDay bool) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	d, err := time.ParseInLocation("2006-01-02", v, bangkok)
	if err != nil {
		return nil, domain.NewValidationError(field, "must be YYYY-MM-DD")
	}
	if endOfDay {
		d = d.Add(24*time.Hour - time.Second)
	}
	return &d, nil
}

func toBatchInput(req dto.ProductCodeBatchRequest) (usecaseproductcode.BatchInput, error) {
	lotExp, err := parseDay(req.LotExpiryDate, "lotExpiryDate", false)
	if err != nil {
		return usecaseproductcode.BatchInput{}, err
	}
	codeExp, err := parseDay(req.CodeExpiresAt, "codeExpiresAt", true)
	if err != nil {
		return usecaseproductcode.BatchInput{}, err
	}
	return usecaseproductcode.BatchInput{
		SKUID: req.ERPSKUID, SKUCode: req.SKUCode, SKUName: req.SKUName, SKUPrice: req.SKUPrice,
		Quantity: req.Quantity, PointsPerCode: req.PointsPerCode, SpendValuePerCode: req.SpendValuePerCode,
		LotNumber: req.LotNumber, LotExpiryDate: lotExp, CodeExpiresAt: codeExp, IsTest: req.IsTest, Note: req.Note,
	}, nil
}

// List serves GET /product-code-batches?search=&status=&page=&limit=.
func (h *ProductCodeHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	rows, total, err := h.batches.List(c.UserContext(), domainproductcode.Query{
		Search: c.Query("search"), Status: c.Query("status"), Page: page, Limit: limit,
	})
	if err != nil {
		return h.writeError(c, err)
	}
	out := make([]dto.ProductCodeBatchResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.ToProductCodeBatchResponse(r.Batch, r.Stats))
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return response.List(c, out, page, limit, total)
}

// Get serves GET /product-code-batches/:id.
func (h *ProductCodeHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	b, err := h.batches.Get(c.UserContext(), id)
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, dto.ToProductCodeBatchResponse(b.Batch, b.Stats))
}

// Create serves POST /product-code-batches.
func (h *ProductCodeHandler) Create(c *fiber.Ctx) error {
	var req dto.ProductCodeBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	in, err := toBatchInput(req)
	if err != nil {
		return h.writeError(c, err)
	}
	actor, _ := middleware.ActorFrom(c)
	b, err := h.batches.Create(c.UserContext(), actor, in)
	if err != nil {
		return h.writeError(c, err)
	}
	return response.Created(c, dto.ToProductCodeBatchResponse(*b, domainproductcode.Stats{}), "Batch created")
}

// Update serves PATCH /product-code-batches/:id (drafts only).
func (h *ProductCodeHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req dto.ProductCodeBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	in, err := toBatchInput(req)
	if err != nil {
		return h.writeError(c, err)
	}
	b, err := h.batches.Update(c.UserContext(), id, in)
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, dto.ToProductCodeBatchResponse(*b, domainproductcode.Stats{}), "Batch updated")
}

// Export serves POST /product-code-batches/:id/export?format=csv|xlsx. It
// generates the codes and returns the print file once.
func (h *ProductCodeHandler) Export(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	format := strings.ToLower(c.Query("format", "xlsx"))
	if format != "xlsx" && format != "csv" {
		return response.BadRequest(c, "format must be xlsx or csv", "VALIDATION_ERROR")
	}
	actor, _ := middleware.ActorFrom(c)
	exp, err := h.batches.Export(c.UserContext(), actor, id)
	if err != nil {
		return h.writeError(c, err)
	}
	var file []byte
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if format == "csv" {
		file, err = export.ProductCodesCSV(exp, bangkok)
		contentType = "text/csv; charset=utf-8"
	} else {
		file, err = export.ProductCodesXLSX(exp, bangkok)
	}
	if err != nil {
		// The codes are committed but the file failed: the batch must be
		// voided and recreated. Logged loudly so staff can tell.
		log.Printf("[ERROR] product code export %s rendered no file: %v", exp.Batch.BatchNo, err)
		return response.InternalServerError(c, "Codes were generated but the file failed; void this batch and create a new one")
	}
	log.Printf("[AUDIT] product codes export batch=%s codes=%d format=%s by erp_user_id=%d",
		exp.Batch.BatchNo, len(exp.Codes), format, actor.ERPUserID)
	name := fmt.Sprintf("%s-codes.%s", exp.Batch.BatchNo, format)
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Send(file)
}

// Activate serves POST /product-code-batches/:id/activate.
func (h *ProductCodeHandler) Activate(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	actor, _ := middleware.ActorFrom(c)
	b, err := h.batches.Activate(c.UserContext(), actor, id)
	if err != nil {
		return h.writeError(c, err)
	}
	return h.withStats(c, b.ID, "Batch activated")
}

// Void serves POST /product-code-batches/:id/void {reason}.
func (h *ProductCodeHandler) Void(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"), "id")
	if err != nil {
		return h.writeError(c, err)
	}
	var req dto.VoidBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	actor, _ := middleware.ActorFrom(c)
	b, err := h.batches.Void(c.UserContext(), actor, id, req.Reason)
	if err != nil {
		return h.writeError(c, err)
	}
	log.Printf("[AUDIT] product code batch void batch=%s by erp_user_id=%d", b.BatchNo, actor.ERPUserID)
	return h.withStats(c, b.ID, "Batch voided")
}

func (h *ProductCodeHandler) withStats(c *fiber.Ctx, id uint, msg string) error {
	b, err := h.batches.Get(c.UserContext(), id)
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, dto.ToProductCodeBatchResponse(b.Batch, b.Stats), msg)
}

// policyDTO is the member-side limits.
type policyDTO struct {
	DailyQuota   int `json:"dailyQuota"`
	WrongPerHour int `json:"wrongPerHour"`
	WrongPerDay  int `json:"wrongPerDay"`
}

// Policy serves GET /product-code-policy.
func (h *ProductCodeHandler) Policy(c *fiber.Ctx) error {
	p, err := h.members.Policy(c.UserContext())
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, policyDTO(p))
}

// PutPolicy serves PUT /product-code-policy (owner, checked in crm-core).
func (h *ProductCodeHandler) PutPolicy(c *fiber.Ctx) error {
	var req policyDTO
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	actor, _ := middleware.ActorFrom(c)
	p, err := h.members.PutPolicy(c.UserContext(), actor, domainproductcode.Policy(req))
	if err != nil {
		return h.writeError(c, err)
	}
	return response.OK(c, policyDTO(p), "Policy saved")
}

type flagDTO struct {
	MemberID   uint      `json:"memberId"`
	MemberCode string    `json:"memberCode"`
	Name       string    `json:"name"`
	Wrong      int64     `json:"wrong"`
	LastAt     time.Time `json:"lastAt"`
}

// Flagged serves GET /product-code-flags: members over the daily wrong-code limit.
func (h *ProductCodeHandler) Flagged(c *fiber.Ctx) error {
	rows, err := h.members.Flagged(c.UserContext())
	if err != nil {
		return h.writeError(c, err)
	}
	out := make([]flagDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, flagDTO{MemberID: r.MemberID, MemberCode: r.MemberCode,
			Name: strings.TrimSpace(r.FirstName + " " + r.LastName), Wrong: r.Wrong, LastAt: r.LastAt})
	}
	return response.OK(c, out)
}

type testerDTO struct {
	MemberID   uint      `json:"memberId"`
	MemberCode string    `json:"memberCode"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	AddedAt    time.Time `json:"addedAt"`
}

// TestMembers serves GET /test-members.
func (h *ProductCodeHandler) TestMembers(c *fiber.Ctx) error {
	rows, err := h.members.TestMembers(c.UserContext())
	if err != nil {
		return h.writeError(c, err)
	}
	out := make([]testerDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, testerDTO{MemberID: r.MemberID, MemberCode: r.MemberCode,
			Name: strings.TrimSpace(r.FirstName + " " + r.LastName), Email: r.Email, AddedAt: r.AddedAt})
	}
	return response.OK(c, out)
}

// AddTestMember serves PUT /test-members/:memberId.
func (h *ProductCodeHandler) AddTestMember(c *fiber.Ctx) error {
	id, err := parseID(c.Params("memberId"), "memberId")
	if err != nil {
		return h.writeError(c, err)
	}
	actor, _ := middleware.ActorFrom(c)
	if err := h.members.AddTestMember(c.UserContext(), actor, id); err != nil {
		return h.writeError(c, err)
	}
	log.Printf("[AUDIT] test member added member_id=%d by erp_user_id=%d", id, actor.ERPUserID)
	return response.OK(c, fiber.Map{"memberId": id}, "Test member added")
}

// RemoveTestMember serves DELETE /test-members/:memberId.
func (h *ProductCodeHandler) RemoveTestMember(c *fiber.Ctx) error {
	id, err := parseID(c.Params("memberId"), "memberId")
	if err != nil {
		return h.writeError(c, err)
	}
	if err := h.members.RemoveTestMember(c.UserContext(), id); err != nil {
		return h.writeError(c, err)
	}
	actor, _ := middleware.ActorFrom(c)
	log.Printf("[AUDIT] test member removed member_id=%d by erp_user_id=%d", id, actor.ERPUserID)
	return response.OK(c, fiber.Map{"memberId": id}, "Test member removed")
}
