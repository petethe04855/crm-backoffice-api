package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerHoldsEveryPermission(t *testing.T) {
	require.Len(t, PermissionsFor([]string{"owner"}), len(Matrix))
}

func TestAdminCannotManageOwnerOnlyAreas(t *testing.T) {
	admin := []string{"sales", "admin"}
	require.True(t, Allowed(admin, MembersExport))
	require.True(t, Allowed(admin, PointsAdjust))
	require.False(t, Allowed(admin, StaffRolesManage), "only owners grant CRM roles")
	require.False(t, Allowed(admin, SettingsManage))
	require.False(t, Allowed(admin, ProductCodesExport), "code export needs owner approval")
}

func TestERPRolesWithoutCRMGrant(t *testing.T) {
	// warehouse holds the fulfilment queue and (since T1) the reward catalogue.
	require.Equal(t, []string{RedemptionsFulfill, RewardsView}, PermissionsFor([]string{"warehouse"}))
	require.True(t, Allowed([]string{"sales"}, MembersView))
	require.False(t, Allowed([]string{"sales"}, MembersExport))
	require.True(t, Allowed([]string{"accountant"}, PointsView))
	require.False(t, Allowed([]string{"live"}, MembersView), "roles outside the matrix get nothing")
}

// TestReservedModulePermissions pins the T1 permission table (plan §11) so the
// module tasks can rely on it before their own tests exist.
func TestReservedModulePermissions(t *testing.T) {
	cases := []struct {
		perm    string
		allowed []string
		denied  []string
	}{
		{PromotionsView, []string{owner, admin, sales}, []string{warehouse, accountant}},
		{PromotionsManage, []string{owner, admin}, []string{sales, warehouse, accountant}},
		{RewardsView, []string{owner, admin, sales, warehouse, accountant}, nil},
		{CouponsView, []string{owner, admin, sales, accountant}, []string{warehouse}},
		{CouponsManage, []string{owner, admin}, []string{sales, accountant}},
		{SettingsView, []string{owner, admin, accountant}, []string{sales, warehouse}},
	}
	for _, c := range cases {
		for _, role := range c.allowed {
			require.True(t, Allowed([]string{role}, c.perm), "%s should hold %s", role, c.perm)
		}
		for _, role := range c.denied {
			require.False(t, Allowed([]string{role}, c.perm), "%s should not hold %s", role, c.perm)
		}
	}
}

func TestRoleMatchIsCaseInsensitiveAndUnknownPermissionDenied(t *testing.T) {
	require.True(t, Allowed([]string{"Owner"}, MembersView))
	require.False(t, Allowed([]string{"owner"}, "does.not.exist"))
}
