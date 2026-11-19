package dto

import (
	"time"

	domainstaffrole "crm-core/domain/staffrole"
)

// MeResponse is the signed-in staff member as the CRM sees them.
type MeResponse struct {
	ERPUserID   uint     `json:"erpUserId"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// GrantStaffRoleRequest grants a CRM role to an ERP user.
type GrantStaffRoleRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// StaffRoleResponse is a CRM role grant.
type StaffRoleResponse struct {
	ERPUserID uint      `json:"erpUserId"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	GrantedBy uint      `json:"grantedBy"`
	GrantedAt time.Time `json:"grantedAt"`
}

// ToStaffRoleResponse maps a grant.
func ToStaffRoleResponse(r domainstaffrole.StaffRole) StaffRoleResponse {
	return StaffRoleResponse{ERPUserID: r.ERPUserID, Email: r.Email, Role: r.Role, GrantedBy: r.GrantedBy, GrantedAt: r.GrantedAt}
}
