// Package middleware holds HTTP middleware for backoffice-api.
package middleware

import (
	"context"
	"errors"
	"log"
	"strings"

	"backoffice-api/internal/authz"
	"backoffice-api/internal/client/erp"
	"backoffice-api/pkg/jwt"
	"backoffice-api/pkg/response"

	"crm-core/domain"

	"github.com/gofiber/fiber/v2"
)

const actorKey = "actor"

// SessionChecker returns the live ERP staff record for a bearer token.
type SessionChecker interface {
	Check(ctx context.Context, token string) (*erp.StaffUser, error)
}

// RoleResolver adds CRM roles (crm.staff_roles) to the ERP role.
type RoleResolver interface {
	EffectiveRoles(ctx context.Context, erpUserID uint, erpRole string) ([]string, error)
}

// StaffAuthConfig configures StaffAuth.
type StaffAuthConfig struct {
	ERPJWTSecret string
	// Sessions re-checks the account with the ERP (disabled/demoted users).
	// nil disables the check; production config refuses that.
	Sessions SessionChecker
	Roles    RoleResolver
}

// StaffAuth authenticates staff with the ERP-issued JWT and stores the actor
// (ERP user + effective roles) in the request context.
func StaffAuth(cfg StaffAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return response.Unauthorized(c, "Authorization header with Bearer token is required", "MISSING_TOKEN")
		}

		claims, err := jwt.ValidateERPToken(token, cfg.ERPJWTSecret)
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired token", "INVALID_TOKEN")
		}

		erpRole := claims.Role
		email := claims.Email
		if cfg.Sessions != nil {
			user, err := cfg.Sessions.Check(c.UserContext(), token)
			switch {
			case errors.Is(err, erp.ErrUnauthorized):
				return response.Unauthorized(c, "Session is no longer valid", "INVALID_SESSION")
			case err != nil:
				log.Printf("[WARN] ERP session check failed: %v", err)
				return response.Error(c, fiber.StatusServiceUnavailable, "ERP_UNAVAILABLE", "Cannot verify the session with the ERP right now")
			case user.ID != claims.UserID || !user.IsActive:
				return response.Unauthorized(c, "Account is disabled or no longer exists", "ACCOUNT_DISABLED")
			case user.MustResetPassword:
				return response.Forbidden(c, "Password reset required in the ERP before using the CRM", "PASSWORD_RESET_REQUIRED")
			}
			erpRole = user.Role // current role wins over the role baked into the token
			email = user.Email
		}

		roles, err := cfg.Roles.EffectiveRoles(c.UserContext(), claims.UserID, erpRole)
		if err != nil {
			log.Printf("[ERROR] resolve staff roles: %v", err)
			return response.InternalServerError(c, "Cannot resolve staff roles")
		}

		c.Locals(actorKey, domain.Actor{ERPUserID: claims.UserID, Email: email, Roles: roles})
		return c.Next()
	}
}

// RequirePermission rejects actors without perm.
func RequirePermission(perm string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		actor, ok := ActorFrom(c)
		if !ok {
			return response.Unauthorized(c, "Unauthorized")
		}
		if !authz.Allowed(actor.Roles, perm) {
			return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
		}
		return c.Next()
	}
}

// RequireAnyPermission rejects actors who hold none of perms. Use it for a
// shared route such as the image upload (promotions.manage OR rewards.manage).
func RequireAnyPermission(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		actor, ok := ActorFrom(c)
		if !ok {
			return response.Unauthorized(c, "Unauthorized")
		}
		for _, perm := range perms {
			if authz.Allowed(actor.Roles, perm) {
				return c.Next()
			}
		}
		return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
	}
}

// ActorFrom returns the authenticated actor set by StaffAuth.
func ActorFrom(c *fiber.Ctx) (domain.Actor, bool) {
	actor, ok := c.Locals(actorKey).(domain.Actor)
	return actor, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
