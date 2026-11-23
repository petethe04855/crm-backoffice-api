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
	"backoffice-api/pkg/line"

	"crm-core/repository/postgres"
	usecaseconsent "crm-core/usecase/consent"
	usecasecoupon "crm-core/usecase/coupon"
	usecaseengagement "crm-core/usecase/engagement"
	usecaselinemsg "crm-core/usecase/linemsg"
	usecasemember "crm-core/usecase/member"
	"crm-core/usecase/memberreport"
	usecasepoints "crm-core/usecase/points"
	usecaseprivacy "crm-core/usecase/privacy"
	usecaseproductcode "crm-core/usecase/productcode"
	usecasepromotion "crm-core/usecase/promotion"
	usecasereward "crm-core/usecase/reward"
	usecasestaffrole "crm-core/usecase/staffrole"
	usecasetier "crm-core/usecase/tier"

	"gorm.io/gorm"
)

// wireMember builds the member and points handlers (they share the member
// repository).
func wireMember(db *gorm.DB, notifier usecasemember.UnlinkNotifier) (*handler.MemberHandler, *handler.PointsHandler) {
	memberRepo := postgres.NewMemberRepository(db)
	tierRepo := postgres.NewTierRepository(db)
	memberHandler := handler.NewMemberHandler(
		usecasemember.New(memberRepo, tierRepo, usecasemember.WithUnlinkNotifier(notifier)),
		usecasemember.NewExporter(memberRepo, tierRepo),
		usecasetier.New(tierRepo),
	)
	pointsHandler := handler.NewPointsHandler(
		usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db)),
	)
	return memberHandler, pointsHandler
}

// wireLineMessaging builds the LINE notifier (queues messages) and, when a
// channel access token is set, the worker job that sends the outbox.
func wireLineMessaging(db *gorm.DB, token, memberMenuID, liffURL string) (*usecaselinemsg.Notifier, []worker.Job) {
	outbox := postgres.NewLineOutboxRepository(db)
	notifier := usecaselinemsg.NewNotifier(outbox, postgres.NewMemberRepository(db),
		usecaseconsent.New(postgres.NewConsentRepository(db)), liffURL).WithPrefs(postgres.NewPrivacyRepository(db))
	if token == "" {
		log.Println("[WARN] LINE_CHANNEL_ACCESS_TOKEN is empty: LINE messages are queued but not sent")
		return notifier, nil
	}
	sender := line.New(token, memberMenuID)
	return notifier, []worker.Job{{
		Name:  "line-outbox",
		Every: 15 * time.Second,
		Run: func(ctx context.Context) error {
			res, err := usecaselinemsg.Dispatch(ctx, outbox, sender, time.Now(), 100)
			if res.Sent+res.Failed+res.Dead > 0 {
				log.Printf("[INFO] line-outbox sent=%d retry=%d dead=%d", res.Sent, res.Failed, res.Dead)
			}
			return err
		},
	}}
}

// wirePrivacy builds the PDPA deletion queue and the daily engagement jobs:
// expiring-points reminders (10:00) and birthday bonus + greeting (08:00).
func wirePrivacy(db *gorm.DB, notifier *usecaselinemsg.Notifier) (*handler.PrivacyHandler, []worker.Job) {
	consents := usecaseconsent.New(postgres.NewConsentRepository(db))
	uc := usecaseprivacy.New(postgres.NewPrivacyRepository(db), postgres.NewPrivacyUnitOfWork(db), usecaseprivacy.Sources{
		Members: postgres.NewMemberRepository(db), Profiles: postgres.NewProfileRepository(db), Consents: consents,
		Points: postgres.NewPointsRepository(db), Coupons: postgres.NewCouponRepository(db), Rewards: postgres.NewRewardRepository(db),
		Codes: postgres.NewProductCodeAttemptRepository(db), MemberCards: postgres.NewMemberCardRepository(db),
	}, usecaseprivacy.WithMenuReset(notifier))

	points := usecasepoints.New(postgres.NewPointsRepository(db), postgres.NewSettingsRepository(db))
	jobs := usecaseengagement.New(postgres.NewEngagementRepository(db), postgres.NewPointsRepository(db),
		postgres.NewEarnRuleRepository(db), func(ctx context.Context) (int, error) {
			p, err := points.ExpiryPolicy(ctx)
			return p.Months, err
		}, notifier)
	return handler.NewPrivacyHandler(uc), []worker.Job{
		{
			Name: "points.expiring-reminder",
			At:   &worker.DailyAt{Hour: 10, Minute: 0},
			Run: func(ctx context.Context) error {
				n, err := jobs.RemindExpiring(ctx, time.Now())
				if n > 0 {
					log.Printf("[INFO] points.expiring-reminder: %d member(s)", n)
				}
				return err
			},
		},
		{
			Name: "members.birthday",
			At:   &worker.DailyAt{Hour: 8, Minute: 0},
			Run: func(ctx context.Context) error {
				n, err := jobs.Birthdays(ctx, time.Now())
				if n > 0 {
					log.Printf("[INFO] members.birthday: credited %d member(s)", n)
				}
				return err
			},
		},
	}
}

// wireConsent builds the consent document handler (LINE OA PDPA documents).
func wireConsent(db *gorm.DB) *handler.ConsentHandler {
	return handler.NewConsentHandler(usecaseconsent.New(postgres.NewConsentRepository(db)))
}

// wireProductCode builds the packaging-code batch handler (L4B).
func wireProductCode(db *gorm.DB, secret string) *handler.ProductCodeHandler {
	members := usecaseproductcode.NewRedeemer(usecaseproductcode.RedeemerDeps{
		Attempts: postgres.NewProductCodeAttemptRepository(db),
		Settings: postgres.NewSettingsRepository(db),
	})
	return handler.NewProductCodeHandler(usecaseproductcode.New(
		postgres.NewProductCodeRepository(db), postgres.NewProductCodeUnitOfWork(db), []byte(secret)), members)
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

// wirePromotion builds the promotion handler. CTAs that point at a coupon or a
// reward are checked against those modules when a promotion is published.
func wirePromotion(db *gorm.DB) *handler.PromotionHandler {
	checker := usecasepromotion.NewTargetChecker(postgres.NewCouponRepository(db), postgres.NewRewardRepository(db))
	return handler.NewPromotionHandler(
		usecasepromotion.New(postgres.NewPromotionRepository(db), checker),
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
func wireReward(db *gorm.DB, client *erp.Client, notifier usecasereward.Notifier) (*handler.RewardHandler, []worker.Job) {
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
		usecasereward.WithNotifier(notifier),
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
	return handler.NewRewardHandler(uc, client, handler.WithLineOutbox(postgres.NewLineOutboxRepository(db))), []worker.Job{sync}
}

// wireMemberReport builds the CRM member report (dashboard part B).
func wireMemberReport(db *gorm.DB) *handler.MemberReportHandler {
	return handler.NewMemberReportHandler(memberreport.New(postgres.NewMemberReportRepository(db)))
}
