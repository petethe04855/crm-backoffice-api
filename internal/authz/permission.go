// Package authz is the backoffice permission matrix (plan: CRM_BACKOFFICE.md §2).
// Roles are the actor's effective roles: the ERP role from the JWT plus CRM
// roles from crm.staff_roles (e.g. "admin").
package authz

import (
	"sort"
	"strings"

	"crm-core/domain"
)

// Permission names.
const (
	MembersView        = "members.view"
	MembersManage      = "members.manage"
	MembersExport      = "members.export"
	MembersNotes       = "members.notes"
	PointsView         = "points.view"
	PointsAdjust       = "points.adjust"
	ProductCodesManage = "product_codes.manage"
	ProductCodesExport = "product_codes.export"
	RewardsManage      = "rewards.manage"
	RedemptionsFulfill = "redemptions.fulfill"
	PrivilegesManage   = "privileges.manage"
	CouponsUse         = "coupons.use"
	BroadcastsSend     = "broadcasts.send"
	ReportsView        = "reports.view"
	SettingsManage     = "settings.manage"
	StaffRolesManage   = "staff_roles.manage"
	// SalesDashboardView unlocks the marketplace sales dashboard.
	SalesDashboardView = "sales_dashboard.view"
	// SalesDashboardCustomers unlocks buyer-level rows and unmasked usernames.
	SalesDashboardCustomers = "sales_dashboard.customers"

	// PromotionsView opens the monthly promotion calendar; PromotionsManage
	// creates and edits promotions.
	PromotionsView   = "promotions.view"
	PromotionsManage = "promotions.manage"
	// RewardsView lists the reward catalogue (editing uses RewardsManage).
	RewardsView = "rewards.view"
	// CouponsView lists coupons; CouponsManage issues and edits them.
	CouponsView   = "coupons.view"
	CouponsManage = "coupons.manage"
	// SettingsView opens the CRM settings screens (point rules). Granting CRM
	// roles stays owner-only under StaffRolesManage.
	SettingsView = "settings.view"
	// ConsentsManage publishes the PDPA consent documents shown in the LINE OA.
	ConsentsManage = "consents.manage"
)

const (
	owner      = domain.ERPRoleOwner
	admin      = domain.RoleAdmin
	sales      = domain.ERPRoleSales
	warehouse  = domain.ERPRoleWarehouse
	accountant = domain.ERPRoleAccountant
)

// Matrix maps each permission to the roles that hold it.
var Matrix = map[string][]string{
	MembersView:        {owner, admin, sales, accountant},
	MembersManage:      {owner, admin},
	MembersExport:      {owner, admin},
	MembersNotes:       {owner, admin, sales},
	PointsView:         {owner, admin, accountant},
	PointsAdjust:       {owner, admin}, // admin limited by owner-set caps (P2)
	ProductCodesManage: {owner, admin},
	ProductCodesExport: {owner}, // admin exports need owner approval
	RewardsManage:      {owner, admin},
	RedemptionsFulfill: {owner, admin, warehouse},
	PrivilegesManage:   {owner, admin},
	CouponsUse:         {owner, admin, sales},
	BroadcastsSend:     {owner, admin},
	ReportsView:        {owner, admin, sales, accountant},
	SettingsManage:     {owner},
	StaffRolesManage:   {owner},
	// accountant sees aggregate numbers only (plan §6.4), so they hold view but
	// not the customer-level permission.
	SalesDashboardView:      {owner, admin, sales, accountant},
	SalesDashboardCustomers: {owner, admin, sales},

	// Permissions reserved by T1 so the module route files can be written
	// before the modules exist (plan §11).
	PromotionsView:   {owner, admin, sales},
	PromotionsManage: {owner, admin},
	RewardsView:      {owner, admin, sales, warehouse, accountant},
	CouponsView:      {owner, admin, sales, accountant},
	CouponsManage:    {owner, admin},
	SettingsView:     {owner, admin, accountant},
	ConsentsManage:   {owner, admin},
}

// Allowed reports whether any of roles holds perm. Unknown permissions are denied.
func Allowed(roles []string, perm string) bool {
	for _, want := range Matrix[perm] {
		for _, have := range roles {
			if strings.EqualFold(have, want) {
				return true
			}
		}
	}
	return false
}

// PermissionsFor lists every permission the roles hold, sorted.
func PermissionsFor(roles []string) []string {
	out := []string{}
	for perm := range Matrix {
		if Allowed(roles, perm) {
			out = append(out, perm)
		}
	}
	sort.Strings(out)
	return out
}
