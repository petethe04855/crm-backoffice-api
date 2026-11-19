// Package handler holds HTTP handlers. Handlers parse requests, call crm-core
// usecases and map results to DTOs; they hold no business rules.
package handler

import (
	"errors"
	"log"

	"backoffice-api/pkg/response"

	"crm-core/domain"
	domaincoupon "crm-core/domain/coupon"
	domainreward "crm-core/domain/reward"

	"github.com/gofiber/fiber/v2"
)

// writeError maps crm-core domain errors to the standard error envelope.
func writeError(c *fiber.Ctx, err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return response.BadRequest(c, ve.Error(), "VALIDATION_ERROR")
	case errors.Is(err, domain.ErrValidation):
		return response.BadRequest(c, err.Error(), "VALIDATION_ERROR")
	case errors.Is(err, domain.ErrNotFound):
		return response.NotFound(c, "Resource not found")
	case errors.Is(err, domain.ErrForbidden):
		return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
	case errors.Is(err, domainreward.ErrERPUnavailable):
		return response.Error(c, fiber.StatusServiceUnavailable, "ERP_UNAVAILABLE", "ERP is unavailable, try again later")
	case errors.Is(err, domainreward.ErrInsufficientERP):
		return response.Error(c, fiber.StatusConflict, "INSUFFICIENT_ERP_STOCK", "ERP stock is not enough to ship; keep packing or cancel")
	case errors.Is(err, domainreward.ErrNotAvailable):
		return response.Error(c, fiber.StatusConflict, "REWARD_NOT_AVAILABLE", "Reward is not open for redemption")
	case errors.Is(err, domainreward.ErrOutOfStock):
		return response.Error(c, fiber.StatusConflict, "REWARD_OUT_OF_STOCK", "Reward is out of stock")
	case errors.Is(err, domainreward.ErrLimitReached):
		return response.Error(c, fiber.StatusConflict, "REWARD_LIMIT_REACHED", "Member reached the limit for this reward")
	case errors.Is(err, domainreward.ErrTierTooLow):
		return response.Error(c, fiber.StatusConflict, "REWARD_TIER_TOO_LOW", "Member tier is below the reward minimum")
	case errors.Is(err, domainreward.ErrSKULocked):
		return response.Error(c, fiber.StatusConflict, "REWARD_SKU_LOCKED", "The SKU cannot change after members redeemed this reward")
	case errors.Is(err, domainreward.ErrInvalidStatus):
		return response.Error(c, fiber.StatusConflict, "INVALID_REDEMPTION_STATUS", "Redemption is not in a status that allows this")
	case errors.Is(err, domainreward.ErrCategoryInUse):
		return response.Error(c, fiber.StatusConflict, "CATEGORY_IN_USE", "Category still has rewards")
	case errors.Is(err, domainreward.ErrUnsupportedType):
		return response.BadRequest(c, "Only physical rewards are supported for now", "UNSUPPORTED_REWARD_TYPE")
	case errors.Is(err, domaincoupon.ErrQuotaReached):
		return response.Error(c, fiber.StatusConflict, "COUPON_QUOTA_REACHED", "Coupon quota reached")
	case errors.Is(err, domaincoupon.ErrLimitReached):
		return response.Error(c, fiber.StatusConflict, "COUPON_LIMIT_REACHED", "Member already holds the maximum of this coupon")
	case errors.Is(err, domaincoupon.ErrNotActive):
		return response.Error(c, fiber.StatusConflict, "COUPON_NOT_ACTIVE", "Coupon is not open for claiming")
	case errors.Is(err, domaincoupon.ErrExpired):
		return response.Error(c, fiber.StatusConflict, "COUPON_EXPIRED", "Coupon has expired")
	case errors.Is(err, domaincoupon.ErrAlreadyUsed):
		return response.Error(c, fiber.StatusConflict, "COUPON_ALREADY_USED", "Coupon was already used")
	case errors.Is(err, domaincoupon.ErrNotUsable):
		return response.Error(c, fiber.StatusConflict, "COUPON_NOT_USABLE", "Coupon was cancelled or cannot be used")
	case errors.Is(err, domaincoupon.ErrTierTooLow):
		return response.Error(c, fiber.StatusConflict, "COUPON_TIER_TOO_LOW", "Member tier is below the coupon minimum")
	case errors.Is(err, domain.ErrMemberTooNew):
		return response.Error(c, fiber.StatusConflict, "MEMBER_TOO_NEW", "Member is too new to redeem")
	case errors.Is(err, domain.ErrDailyLimitReached):
		return response.Error(c, fiber.StatusConflict, "DAILY_LIMIT_REACHED", "Daily redemption limit reached")
	case errors.Is(err, domain.ErrEmailNotVerified):
		return response.Error(c, fiber.StatusConflict, "EMAIL_NOT_VERIFIED", "Email is not verified")
	case errors.Is(err, domain.ErrMemberNotAllowed):
		return response.Error(c, fiber.StatusConflict, "MEMBER_NOT_ALLOWED", "Member status does not allow redemption")
	case errors.Is(err, domain.ErrInsufficientPoints):
		return response.Error(c, fiber.StatusConflict, "INSUFFICIENT_POINTS", "Point balance is too low for this deduction")
	case errors.Is(err, domain.ErrConflict):
		return response.Error(c, fiber.StatusConflict, "CONFLICT", "Resource conflict")
	default:
		log.Printf("[ERROR] %s %s: %v", c.Method(), c.Path(), err)
		return response.InternalServerError(c, "Internal server error")
	}
}
