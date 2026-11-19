package main

// Module wiring. Each module owns one wireXxx function that builds its
// handlers from the shared DB (and its config); main.go just calls them and
// passes the handlers to route.Register. A new module adds a function here and
// one field in route.Handlers - main.go gains one line.

import (
	"context"
	"log"
	"time"

	"backoffice-api/internal/client/erp"
	"backoffice-api/internal/delivery/http/handler"
	"backoffice-api/internal/storage"
	"backoffice-api/internal/worker"

	"crm-core/repository/postgres"
	usecasecoupon "crm-core/usecase/coupon"
	usecasemember "crm-core/usecase/member"
	"crm-core/usecase/memberreport"
	usecasepoints "crm-core/usecase/points"
	usecasepromotion "crm-core/usecase/promotion"
	usecasereward "crm-core/usecase/reward"
	usecasestaffrole "crm-core/usecase/staffrole"
	usecasetier "crm-core/usecase/tier"

	"gorm.io/gorm"
)

// wireMember builds the member and points handlers (they share the member
// repository).
func wireMember(db *gorm.DB) (*handler.MemberHandler, *handler.PointsHandler) {
	memberRepo := postgres.NewMemberRepository(db)
	tierRepo := postgres.NewTierRepository(db)
	memberHandler := handler.NewMemberHandler(
		usecasemember.New(memberRepo, tierRepo),
		usecasemember.NewExporter(memberRepo, tierRepo),
		usecasetier.New(tierRepo),
	)
	pointsHandler := handler.NewPointsHandler(
		usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db)),
	)
	return memberHandler, pointsHandler
}

// wireStaffRole builds the staff role handler and exposes the usecase, which
// the auth middleware also needs to resolve CRM roles.
func wireStaffRole(db *gorm.DB) (*handler.StaffRoleHandler, usecasestaffrole.Usecase) {
	uc := usecasestaffrole.New(postgres.NewStaffRoleRepository(db))
	return handler.NewStaffRoleHandler(uc), uc
}

// wireSalesDashboard builds the marketplace dashboard handler on the ERP client.
func wireSalesDashboard(client *erp.Client) *handler.SalesDashboardHandler {
	return handler.NewSalesDashboardHandler(client)
}

// wireUpload builds the image upload handler on the configured storage backend.
func wireUpload(db *gorm.DB, uploadDir string) (*handler.UploadHandler, error) {
	store, err := storage.NewLocal(uploadDir, storage.URLPrefix)
	if err != nil {
		return nil, err
	}
	return handler.NewUploadHandler(postgres.NewUploadRepository(db), store), nil
}

// wirePromotion builds the promotion handler. The CTA target checker stays nil
// until the reward (T4) and coupon (T6) modules can provide one.
func wirePromotion(db *gorm.DB) *handler.PromotionHandler {
	return handler.NewPromotionHandler(
		usecasepromotion.New(postgres.NewPromotionRepository(db), nil),
	)
}

// wirePointRules builds the earn-rule / policy / calculator handler and the
// daily point-expiry job (00:10 Bangkok, plan §4.3).
func wirePointRules(db *gorm.DB) (*handler.PointRulesHandler, []worker.Job) {
	uc := usecasepoints.New(
		postgres.NewPointsRepository(db),
		postgres.NewSettingsRepository(db),
		usecasepoints.WithEarnRules(postgres.NewEarnRuleRepository(db)),
		usecasepoints.WithMembers(postgres.NewMemberRepository(db)),
		usecasepoints.WithTiers(postgres.NewTierRepository(db)),
	)
	expire := worker.Job{
		Name: "points.expire",
		At:   &worker.DailyAt{Hour: 0, Minute: 10},
		Run: func(ctx context.Context) error {
			n, err := uc.ExpireDue(ctx, time.Now())
			if n > 0 {
				log.Printf("[INFO] points.expire: expired %d lot(s)", n)
			}
			return err
		},
	}
	return handler.NewPointRulesHandler(uc), []worker.Job{expire}
}

// wireCoupon builds the coupon handler and the daily coupon-expiry job (00:05
// Bangkok, plan §8.2). Claims spend points through the points usecase (T3).
func wireCoupon(db *gorm.DB) (*handler.CouponHandler, []worker.Job) {
	points := usecasepoints.New(
		postgres.NewPointsRepository(db),
		postgres.NewSettingsRepository(db),
		usecasepoints.WithMembers(postgres.NewMemberRepository(db)),
	)
	uc := usecasecoupon.New(
		postgres.NewCouponRepository(db),
		postgres.NewCouponUnitOfWork(db),
		postgres.NewMemberRepository(db),
		postgres.NewTierRepository(db),
		points,
	)
	expire := worker.Job{
		Name: "coupons.expire",
		At:   &worker.DailyAt{Hour: 0, Minute: 5},
		Run: func(ctx context.Context) error {
			n, err := uc.ExpireDue(ctx, time.Now())
			if n > 0 {
				log.Printf("[INFO] coupons.expire: expired %d coupon(s)", n)
			}
			return err
		},
	}
	return handler.NewCouponHandler(uc), []worker.Job{expire}
}

// wireReward builds the reward/redemption handler and the stock-sync job that
// refreshes the ERP stock cache every 5 minutes (plan CRM_BACKOFFICE.md §5).
// Without ERP_API_TOKEN the job is skipped and shipping answers ERP_UNAVAILABLE.
func wireReward(db *gorm.DB, client *erp.Client) (*handler.RewardHandler, []worker.Job) {
	points := usecasepoints.New(
		postgres.NewPointsRepository(db),
		postgres.NewSettingsRepository(db),
		usecasepoints.WithMembers(postgres.NewMemberRepository(db)),
	)
	uc := usecasereward.New(
		postgres.NewRewardRepository(db),
		postgres.NewRewardUnitOfWork(db),
		postgres.NewMemberRepository(db),
		postgres.NewTierRepository(db),
		points,
		erp.NewRewardERP(client),
	)
	sync := worker.Job{
		Name:  "rewards.sync_stock",
		Every: 5 * time.Minute,
		Run: func(ctx context.Context) error {
			if !client.HasServiceToken() {
				return nil
			}
			n, err := uc.SyncStock(ctx)
			if n > 0 {
				log.Printf("[INFO] rewards.sync_stock: synced %d SKU(s)", n)
			}
			return err
		},
	}
	return handler.NewRewardHandler(uc, client), []worker.Job{sync}
}

// wireMemberReport builds the CRM member report (dashboard part B).
func wireMemberReport(db *gorm.DB) *handler.MemberReportHandler {
	return handler.NewMemberReportHandler(memberreport.New(postgres.NewMemberReportRepository(db)))
}
