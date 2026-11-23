package route

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backoffice-api/internal/client/erp"
	"backoffice-api/internal/delivery/http/handler"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/internal/storage"

	crmdatabase "crm-core/database"
	domainloyalty "crm-core/domain/loyalty"
	domainmember "crm-core/domain/member"
	domainsales "crm-core/domain/sales"
	"crm-core/migrations"
	"crm-core/repository/postgres"
	usecaseconsent "crm-core/usecase/consent"
	usecasecoupon "crm-core/usecase/coupon"
	usecasemember "crm-core/usecase/member"
	"crm-core/usecase/memberreport"
	usecasepoints "crm-core/usecase/points"
	usecaseprivacy "crm-core/usecase/privacy"
	usecaseproductcode "crm-core/usecase/productcode"
	usecasepromotion "crm-core/usecase/promotion"
	usecasereward "crm-core/usecase/reward"
	usecasestaffrole "crm-core/usecase/staffrole"
	usecasetier "crm-core/usecase/tier"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testSecret = "route-test-secret-123"

// fakeSessions plays the ERP /auth/me role: users keyed by ERP user id.
type fakeSessions struct {
	users map[uint]erp.StaffUser
	err   error
}

func (f *fakeSessions) Check(_ context.Context, token string) (*erp.StaffUser, error) {
	if f.err != nil {
		return nil, f.err
	}
	claims := &struct {
		UserID uint `json:"user_id"`
		gojwt.RegisteredClaims
	}{}
	if _, _, err := gojwt.NewParser().ParseUnverified(token, claims); err != nil {
		return nil, erp.ErrUnauthorized
	}
	u, ok := f.users[claims.UserID]
	if !ok {
		return nil, erp.ErrUnauthorized
	}
	return &u, nil
}

// salesCall records one ERP fetch so tests can assert token forwarding and
// caching.
type salesCall struct {
	bearer          string
	from, to        time.Time
	channel, region string
}

// fakeSales plays the ERP sales-analytics endpoint. The handler always fetches
// the current range first and the previous range second.
type fakeSales struct {
	current, previous []domainsales.SalesOrder
	err               error
	calls             []salesCall
}

func (f *fakeSales) SalesOrders(_ context.Context, bearer string, from, to time.Time, channel, region string) ([]domainsales.SalesOrder, error) {
	f.calls = append(f.calls, salesCall{bearer: bearer, from: from, to: to, channel: channel, region: region})
	if f.err != nil {
		return nil, f.err
	}
	if len(f.calls) == 1 {
		return f.current, nil
	}
	return f.previous, nil
}

type testEnv struct {
	app      *fiber.App
	db       *gorm.DB
	sessions *fakeSessions
	sales    *fakeSales
	erp      *fakeRewardERP
	tierID   uint
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), crmdatabase.GormConfig("", logger.Silent))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, migrations.Run(db, ""))

	var tier domainloyalty.Tier
	require.NoError(t, db.Where("code = ?", "bronze").First(&tier).Error)

	memberRepo := postgres.NewMemberRepository(db)
	tierRepo := postgres.NewTierRepository(db)
	staffUC := usecasestaffrole.New(postgres.NewStaffRoleRepository(db))

	sessions := &fakeSessions{users: map[uint]erp.StaffUser{
		1: {ID: 1, Email: "owner@example.com", Role: "owner", IsActive: true},
		2: {ID: 2, Email: "sales@example.com", Role: "sales", IsActive: true},
		3: {ID: 3, Email: "wh@example.com", Role: "warehouse", IsActive: true},
		4: {ID: 4, Email: "off@example.com", Role: "sales", IsActive: false},
		5: {ID: 5, Email: "reset@example.com", Role: "sales", IsActive: true, MustResetPassword: true},
		6: {ID: 6, Email: "acc@example.com", Role: "accountant", IsActive: true},
	}}

	sales := &fakeSales{}
	rewardERP := newFakeRewardERP()

	uploadStore, err := storage.NewLocal(t.TempDir(), storage.URLPrefix)
	require.NoError(t, err)

	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
	Register(app, Handlers{
		Health:         handler.NewHealthHandler(sqlDB),
		Member:         handler.NewMemberHandler(usecasemember.New(memberRepo, tierRepo), usecasemember.NewExporter(memberRepo, tierRepo), usecasetier.New(tierRepo)),
		Points:         handler.NewPointsHandler(usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db))),
		StaffRole:      handler.NewStaffRoleHandler(staffUC),
		SalesDashboard: handler.NewSalesDashboardHandler(sales),
		Upload:         handler.NewUploadHandler(postgres.NewUploadRepository(db), uploadStore),
		Promotion:      handler.NewPromotionHandler(usecasepromotion.New(postgres.NewPromotionRepository(db), nil)),
		PointRules: handler.NewPointRulesHandler(usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db),
			usecasepoints.WithEarnRules(postgres.NewEarnRuleRepository(db)), usecasepoints.WithMembers(memberRepo), usecasepoints.WithTiers(tierRepo))),
		Coupon: handler.NewCouponHandler(usecasecoupon.New(postgres.NewCouponRepository(db), postgres.NewCouponUnitOfWork(db), memberRepo, tierRepo,
			usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db), usecasepoints.WithMembers(memberRepo)))),
		MemberReport: handler.NewMemberReportHandler(memberreport.New(postgres.NewMemberReportRepository(db))),
		Consent:      handler.NewConsentHandler(usecaseconsent.New(postgres.NewConsentRepository(db))),
		Privacy: handler.NewPrivacyHandler(usecaseprivacy.New(postgres.NewPrivacyRepository(db), postgres.NewPrivacyUnitOfWork(db), usecaseprivacy.Sources{
			Members: memberRepo, Profiles: postgres.NewProfileRepository(db), Consents: usecaseconsent.New(postgres.NewConsentRepository(db)),
			Points: postgres.NewPointsRepository(db), Coupons: postgres.NewCouponRepository(db), Rewards: postgres.NewRewardRepository(db),
			Codes: postgres.NewProductCodeAttemptRepository(db), MemberCards: postgres.NewMemberCardRepository(db),
		})),
		ProductCode: handler.NewProductCodeHandler(usecaseproductcode.New(postgres.NewProductCodeRepository(db),
			postgres.NewProductCodeUnitOfWork(db), []byte("route-test-product-code-secret")),
			usecaseproductcode.NewRedeemer(usecaseproductcode.RedeemerDeps{
				Attempts: postgres.NewProductCodeAttemptRepository(db), Settings: postgres.NewSettingsRepository(db),
			})),
		Reward: handler.NewRewardHandler(usecasereward.New(postgres.NewRewardRepository(db), postgres.NewRewardUnitOfWork(db), memberRepo, tierRepo,
			usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db), usecasepoints.WithMembers(memberRepo)), rewardERP), rewardERP,
			handler.WithLineOutbox(postgres.NewLineOutboxRepository(db))),
	}, middleware.StaffAuth(middleware.StaffAuthConfig{ERPJWTSecret: testSecret, Sessions: sessions, Roles: staffUC}))

	return &testEnv{app: app, db: db, sessions: sessions, sales: sales, erp: rewardERP, tierID: tier.ID}
}

func token(t *testing.T, userID uint, role string) string {
	t.Helper()
	claims := gojwt.MapClaims{
		"user_id": userID, "email": "x@example.com", "role": role,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	s, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

type apiResult struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Meta    struct {
			Page  int   `json:"page"`
			Limit int   `json:"limit"`
			Total int64 `json:"total"`
		} `json:"meta"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
}

func (e *testEnv) do(t *testing.T, method, path, bearer string, body any) apiResult {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := e.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	res := apiResult{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		require.NoError(t, json.Unmarshal(raw, &res.Body))
	}
	return res
}

func (e *testEnv) seedMembers(t *testing.T) {
	t.Helper()
	phone := "+66971166657"
	base := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC) // 10:00 Bangkok
	rows := []domainmember.Member{
		{MemberCode: "M2611-000001", FirstName: "Nopharat", LastName: "Boonnak", Email: "nopharat@gmail.com", PhoneE164: &phone, Status: "active", CreatedAt: base},
		{MemberCode: "M2611-000002", FirstName: "Somchai", LastName: "Jaidee", Email: "somchai@example.com", Status: "suspended", CreatedAt: base.AddDate(0, 0, 2)},
		{MemberCode: "M2611-000003", FirstName: "Mali", LastName: "Srisuk", Email: "mali@example.com", Status: "active", CreatedAt: base.AddDate(0, 0, 4)},
	}
	for i := range rows {
		rows[i].TierID = e.tierID
		rows[i].Gender = "unspecified"
		rows[i].RegisteredChannel = "line"
		rows[i].UpdatedAt = rows[i].CreatedAt
		require.NoError(t, e.db.Create(&rows[i]).Error)
	}
}

func TestHealthIsPublic(t *testing.T) {
	e := newEnv(t)
	res := e.do(t, http.MethodGet, "/api/v1/health", "", nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.True(t, res.Body.Success)
}

func TestStaffAuth(t *testing.T) {
	e := newEnv(t)

	cases := []struct {
		name   string
		bearer string
		status int
		code   string
	}{
		{"missing token", "", http.StatusUnauthorized, "MISSING_TOKEN"},
		{"bad token", "nope", http.StatusUnauthorized, "INVALID_TOKEN"},
		{"unknown ERP user", token(t, 99, "owner"), http.StatusUnauthorized, "INVALID_SESSION"},
		{"disabled ERP account", token(t, 4, "sales"), http.StatusUnauthorized, "ACCOUNT_DISABLED"},
		{"password reset pending", token(t, 5, "sales"), http.StatusForbidden, "PASSWORD_RESET_REQUIRED"},
	}
	for _, c := range cases {
		res := e.do(t, http.MethodGet, "/api/v1/crm/me", c.bearer, nil)
		require.Equal(t, c.status, res.Status, c.name)
		require.Equal(t, c.code, res.Body.Error.Code, c.name)
	}

	e.sessions.err = erp.ErrUnavailable
	res := e.do(t, http.MethodGet, "/api/v1/crm/me", token(t, 1, "owner"), nil)
	require.Equal(t, http.StatusServiceUnavailable, res.Status, "fail closed when the ERP is down")
}

func TestERPRoleBeatsStaleTokenRole(t *testing.T) {
	e := newEnv(t)
	// Token still says owner, but the ERP now reports the user as warehouse.
	res := e.do(t, http.MethodGet, "/api/v1/crm/members", token(t, 3, "owner"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)
}

func TestMeListsPermissions(t *testing.T) {
	e := newEnv(t)
	res := e.do(t, http.MethodGet, "/api/v1/crm/me", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusOK, res.Status)
	var me struct {
		Roles       []string `json:"roles"`
		Permissions []string `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &me))
	require.Equal(t, []string{"sales"}, me.Roles)
	require.Contains(t, me.Permissions, "members.view")
	require.NotContains(t, me.Permissions, "members.export")
}

func TestMembersListFiltersAndPaging(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	sales := token(t, 2, "sales")

	res := e.do(t, http.MethodGet, "/api/v1/crm/members?limit=2", sales, nil)
	require.Equal(t, http.StatusOK, res.Status)
	require.EqualValues(t, 3, res.Body.Meta.Total)
	require.Equal(t, 1, res.Body.Meta.Page)
	require.Equal(t, 2, res.Body.Meta.Limit)
	var rows []struct {
		MemberCode string `json:"memberCode"`
		TierName   string `json:"tierName"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &rows))
	require.Len(t, rows, 2)
	require.Equal(t, "M2611-000003", rows[0].MemberCode, "newest first")
	require.Equal(t, "Bronze", rows[0].TierName)

	res = e.do(t, http.MethodGet, "/api/v1/crm/members?search=somchai", sales, nil)
	require.EqualValues(t, 1, res.Body.Meta.Total)

	res = e.do(t, http.MethodGet, "/api/v1/crm/members?status=suspended", sales, nil)
	require.EqualValues(t, 1, res.Body.Meta.Total)

	// registeredTo is inclusive of the whole Bangkok day.
	res = e.do(t, http.MethodGet, "/api/v1/crm/members?registeredFrom=2026-11-10&registeredTo=2026-11-12", sales, nil)
	require.EqualValues(t, 2, res.Body.Meta.Total)
}

func TestMembersListRejectsBadInput(t *testing.T) {
	e := newEnv(t)
	sales := token(t, 2, "sales")
	for _, q := range []string{"registeredFrom=10-11-2026", "page=abc", "tierId=0", "status=vip"} {
		res := e.do(t, http.MethodGet, "/api/v1/crm/members?"+q, sales, nil)
		require.Equal(t, http.StatusBadRequest, res.Status, q)
		require.Equal(t, "VALIDATION_ERROR", res.Body.Error.Code, q)
	}
}

func TestMemberDetail(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)
	sales := token(t, 2, "sales")

	res := e.do(t, http.MethodGet, "/api/v1/crm/members/1", sales, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var d struct {
		MemberCode string `json:"memberCode"`
		Tier       struct {
			Code string `json:"code"`
		} `json:"tier"`
		Line *struct{} `json:"line"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &d))
	require.Equal(t, "M2611-000001", d.MemberCode)
	require.Equal(t, "bronze", d.Tier.Code)
	require.Nil(t, d.Line)

	res = e.do(t, http.MethodGet, "/api/v1/crm/members/999", sales, nil)
	require.Equal(t, http.StatusNotFound, res.Status)
}

func TestMembersExport(t *testing.T) {
	e := newEnv(t)
	e.seedMembers(t)

	res := e.do(t, http.MethodGet, "/api/v1/crm/members?format=xlsx", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status, "sales cannot export")

	readFirstEmail := func(raw []byte) string {
		f, err := excelize.OpenReader(bytes.NewReader(raw))
		require.NoError(t, err)
		defer f.Close()
		rows, err := f.GetRows("Members")
		require.NoError(t, err)
		require.Len(t, rows, 4, "header + 3 members")
		// rows[3] is the oldest member, Nopharat (newest first).
		return rows[3][3]
	}

	owner := e.do(t, http.MethodGet, "/api/v1/crm/members?format=xlsx", token(t, 1, "owner"), nil)
	require.Equal(t, http.StatusOK, owner.Status)
	require.Contains(t, owner.Header.Get("Content-Disposition"), ".xlsx")
	require.Equal(t, "nopharat@gmail.com", readFirstEmail(owner.Raw), "owner gets unmasked data")

	// Grant CRM admin to the sales user, then export again: allowed but masked.
	grant := e.do(t, http.MethodPut, "/api/v1/crm/staff-roles/2", token(t, 1, "owner"), map[string]string{"email": "sales@example.com", "role": "admin"})
	require.Equal(t, http.StatusOK, grant.Status)

	admin := e.do(t, http.MethodGet, "/api/v1/crm/members?format=xlsx", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusOK, admin.Status)
	require.Equal(t, "no******@gmail.com", readFirstEmail(admin.Raw))
}

func TestStaffRolesOwnerOnly(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	res := e.do(t, http.MethodPut, "/api/v1/crm/staff-roles/2", owner, map[string]string{"role": "admin"})
	require.Equal(t, http.StatusOK, res.Status)

	// The new admin still cannot manage roles.
	res = e.do(t, http.MethodGet, "/api/v1/crm/staff-roles", token(t, 2, "sales"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)

	res = e.do(t, http.MethodPut, "/api/v1/crm/staff-roles/3", owner, map[string]string{"role": "owner"})
	require.Equal(t, http.StatusBadRequest, res.Status, "owner is not a grantable CRM role")

	res = e.do(t, http.MethodGet, "/api/v1/crm/staff-roles", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	var grants []struct {
		ERPUserID uint   `json:"erpUserId"`
		Role      string `json:"role"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &grants))
	require.Len(t, grants, 1)
	require.EqualValues(t, 2, grants[0].ERPUserID)

	res = e.do(t, http.MethodDelete, "/api/v1/crm/staff-roles/2", owner, nil)
	require.Equal(t, http.StatusOK, res.Status)
	res = e.do(t, http.MethodDelete, "/api/v1/crm/staff-roles/2", owner, nil)
	require.Equal(t, http.StatusNotFound, res.Status)
}

func TestUnknownRouteUsesStandardError(t *testing.T) {
	e := newEnv(t)
	res := e.do(t, http.MethodGet, "/api/v1/nope", "", nil)
	require.Equal(t, http.StatusNotFound, res.Status)
	require.False(t, res.Body.Success)
	require.Equal(t, "NOT_FOUND", res.Body.Error.Code)
}

const dashboardQuery = "?dateFrom=2026-11-01&dateTo=2026-11-30"

// seedSales fills the fake ERP with two November orders and one October order.
func (e *testEnv) seedSales() {
	chicken := domainsales.SalesOrder{
		Platform: domainsales.PlatformShopee, OrderNo: "A", OrderedAt: "2026-11-02 10:00:00",
		HasTime: true, BuyerID: "alice1234", Buyer: "alice1234", DateDay: "2026-11-02",
		Amount: 100, Province: "Bangkok", ChickenBags: 5, TotalBags: 5,
	}
	tuna := domainsales.SalesOrder{
		Platform: domainsales.PlatformTikTok, OrderNo: "B", OrderedAt: "2026-11-03 12:00:00",
		HasTime: true, BuyerID: "bob", Buyer: "bob", DateDay: "2026-11-03",
		Amount: 200, Province: "Chiang Mai", TunaBags: 5, TotalBags: 5,
	}
	previous := domainsales.SalesOrder{
		Platform: domainsales.PlatformShopee, OrderNo: "C", OrderedAt: "2026-10-02 10:00:00",
		HasTime: true, BuyerID: "carol", Buyer: "carol", DateDay: "2026-10-02",
		Amount: 60, Province: "Bangkok", ChickenBags: 1, TotalBags: 1,
	}
	e.sales.current = []domainsales.SalesOrder{chicken, tuna}
	e.sales.previous = []domainsales.SalesOrder{previous}
}

func TestSalesDashboardRBAC(t *testing.T) {
	e := newEnv(t)
	e.seedSales()

	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, "", nil)
	require.Equal(t, http.StatusUnauthorized, res.Status)

	// warehouse holds neither dashboard permission.
	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, token(t, 3, "warehouse"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)

	for _, c := range []struct {
		name  string
		token string
	}{
		{"owner", token(t, 1, "owner")},
		{"sales", token(t, 2, "sales")},
		{"accountant", token(t, 6, "accountant")},
	} {
		res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, c.token, nil)
		require.Equal(t, http.StatusOK, res.Status, c.name)
		require.True(t, res.Body.Success, c.name)
	}
}

func TestSalesDashboardReportNumbers(t *testing.T) {
	e := newEnv(t)
	e.seedSales()
	bearer := token(t, 2, "sales")

	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, bearer, nil)
	require.Equal(t, http.StatusOK, res.Status)

	var report struct {
		Masked bool `json:"masked"`
		Range  struct {
			From string `json:"from"`
			To   string `json:"to"`
			Days int    `json:"days"`
		} `json:"range"`
		Summary struct {
			Sales  float64 `json:"sales"`
			Orders int64   `json:"orders"`
			Buyers int64   `json:"buyers"`
			Items  int64   `json:"items"`
			AOV    float64 `json:"aov"`
		} `json:"summary"`
		Previous struct {
			Sales float64 `json:"sales"`
		} `json:"previous"`
		Channels []struct {
			Channel string  `json:"channel"`
			Sales   float64 `json:"sales"`
		} `json:"channels"`
		Provinces []struct {
			Province string `json:"province"`
			Orders   int64  `json:"orders"`
		} `json:"provinces"`
		Customers []struct {
			Buyer  string `json:"buyer"`
			Orders int64  `json:"orders"`
		} `json:"customers"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &report))

	require.False(t, report.Masked, "sales holds sales_dashboard.customers")
	require.Equal(t, "2026-11-01", report.Range.From)
	require.Equal(t, "2026-11-30", report.Range.To)
	require.Equal(t, 30, report.Range.Days)
	require.Equal(t, 300.0, report.Summary.Sales)
	require.EqualValues(t, 2, report.Summary.Orders)
	require.EqualValues(t, 2, report.Summary.Buyers)
	require.EqualValues(t, 10, report.Summary.Items)
	require.Equal(t, 150.0, report.Summary.AOV)
	require.Equal(t, 60.0, report.Previous.Sales)
	require.Len(t, report.Channels, 2)
	require.Len(t, report.Provinces, 2)
	require.Len(t, report.Customers, 2)
	require.Equal(t, "bob", report.Customers[0].Buyer, "ranked by sales")
	require.Equal(t, "alice1234", report.Customers[1].Buyer)

	require.NotEmpty(t, e.sales.calls)
	require.Equal(t, bearer, e.sales.calls[0].bearer, "the staff token is forwarded to the ERP")
}

func TestSalesDashboardAccountantSeesAggregatesOnly(t *testing.T) {
	e := newEnv(t)
	e.seedSales()

	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusOK, res.Status)

	var report struct {
		Masked  bool `json:"masked"`
		Summary struct {
			Sales float64 `json:"sales"`
		} `json:"summary"`
		Customers []struct {
			Buyer string `json:"buyer"`
		} `json:"customers"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &report))
	require.Equal(t, 300.0, report.Summary.Sales)
	require.Empty(t, report.Customers, "buyer rows never reach accountant")

	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers"+dashboardQuery, token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusForbidden, res.Status, "the customer endpoint is refused, not just masked")

	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers/shopee/alice1234"+dashboardQuery, token(t, 6, "accountant"), nil)
	require.Equal(t, http.StatusForbidden, res.Status)
}

func TestSalesDashboardCustomerDetail(t *testing.T) {
	e := newEnv(t)
	e.seedSales()
	sales := token(t, 2, "sales")

	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers/shopee/alice1234"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusOK, res.Status)

	var detail struct {
		Customer struct {
			Buyer  string `json:"buyer"`
			Orders int64  `json:"orders"`
		} `json:"customer"`
		Orders []struct {
			OrderNo string  `json:"orderNo"`
			Amount  float64 `json:"amount"`
		} `json:"orders"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &detail))
	require.Equal(t, "alice1234", detail.Customer.Buyer)
	require.EqualValues(t, 1, detail.Customer.Orders)
	require.Len(t, detail.Orders, 1)
	require.Equal(t, "A", detail.Orders[0].OrderNo)

	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers/shopee/nobody"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusNotFound, res.Status)

	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/customers/lazada/alice1234"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusBadRequest, res.Status)
}

func TestSalesDashboardValidatesFilters(t *testing.T) {
	e := newEnv(t)
	e.seedSales()
	sales := token(t, 2, "sales")

	cases := []string{
		"?dateFrom=2026-11-30&dateTo=2026-11-01",
		"?dateFrom=2025-01-01&dateTo=2026-11-30",
		"?channel=lazada",
		"?region=Atlantis",
		"?dateFrom=30-11-2026",
	}
	for _, query := range cases {
		res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+query, sales, nil)
		require.Equal(t, http.StatusBadRequest, res.Status, query)
		require.Equal(t, "VALIDATION_ERROR", res.Body.Error.Code, query)
	}
}

func TestSalesDashboardMapsERPFailures(t *testing.T) {
	e := newEnv(t)
	sales := token(t, 2, "sales")

	e.sales.err = erp.ErrUnauthorized
	res := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusForbidden, res.Status)
	require.Equal(t, "ERP_FORBIDDEN", res.Body.Error.Code)

	e.sales.err = erp.ErrUnavailable
	res = e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusServiceUnavailable, res.Status)
	require.Equal(t, "ERP_UNAVAILABLE", res.Body.Error.Code)
}

func TestSalesDashboardCachesERPFacts(t *testing.T) {
	e := newEnv(t)
	e.seedSales()
	sales := token(t, 2, "sales")

	first := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusOK, first.Status)
	require.Len(t, e.sales.calls, 2, "current + previous period")

	second := e.do(t, http.MethodGet, "/api/v1/crm/sales-dashboard/summary"+dashboardQuery, sales, nil)
	require.Equal(t, http.StatusOK, second.Status)
	require.Len(t, e.sales.calls, 2, "a second request inside the TTL is served from cache")
}
