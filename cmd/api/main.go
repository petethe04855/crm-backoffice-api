// Command api runs backoffice-api: the CRM backoffice API and the CRM's single
// integration point with erp-api-v2.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backoffice-api/config"
	"backoffice-api/internal/client/erp"
	"backoffice-api/internal/delivery/http/handler"
	"backoffice-api/internal/delivery/http/middleware"
	"backoffice-api/internal/delivery/http/route"
	"backoffice-api/internal/session"
	"backoffice-api/internal/worker"
	"backoffice-api/pkg/database"

	crmdatabase "crm-core/database"
	"crm-core/migrations"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[FATAL] invalid configuration: %v", err)
	}

	logLevel := gormlogger.Warn
	if !cfg.IsProduction() {
		logLevel = gormlogger.Info
	}
	db, err := gorm.Open(gormpostgres.Open(cfg.DSN()), crmdatabase.GormConfig(cfg.DBSchema, logLevel))
	if err != nil && database.IsMissingDatabase(err) {
		log.Printf("[INFO] database %q does not exist, creating it", cfg.DBName)
		if createErr := database.CreateDatabase(cfg.MaintenanceDSN(), cfg.DBName); createErr != nil {
			log.Fatalf("[FATAL] create CRM database %q: %v", cfg.DBName, createErr)
		}
		db, err = gorm.Open(gormpostgres.Open(cfg.DSN()), crmdatabase.GormConfig(cfg.DBSchema, logLevel))
	}
	if err != nil {
		log.Fatalf("[FATAL] connect CRM database (user=%s host=%s db=%s): %v", cfg.DBUser, cfg.DBHost, cfg.DBName, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("[FATAL] database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if cfg.AutoMigrate {
		if err := migrations.Run(db, cfg.DBSchema); err != nil {
			log.Fatalf("[FATAL] migrate CRM schema: %v", err)
		}
		log.Printf("[INFO] CRM schema %q migrated", cfg.DBSchema)
	}

	// Module wiring (see wire.go). main.go stays config → DB → migrate →
	// routes → listen/shutdown.
	lineNotifier, lineJobs := wireLineMessaging(db, cfg.LineChannelAccessToken, cfg.LineMemberRichMenuID, cfg.LiffURL)
	memberHandler, pointsHandler := wireMember(db, lineNotifier)
	privacyHandler, privacyJobs := wirePrivacy(db, lineNotifier)
	staffRoleHandler, staffRoleUC := wireStaffRole(db)
	uploadHandler, err := wireUpload(db, cfg.UploadDir)
	if err != nil {
		log.Fatalf("[FATAL] prepare upload storage: %v", err)
	}

	// Staff authentication: ERP JWT + live ERP session check + CRM roles.
	authCfg := middleware.StaffAuthConfig{ERPJWTSecret: cfg.ERPJWTSecret, Roles: staffRoleUC}
	erpClient := erp.New(cfg.ERPAPIBaseURL, cfg.ERPAPIToken, cfg.ERPRequestTimeout)
	if cfg.ERPSessionCheck {
		authCfg.Sessions = session.NewChecker(erpClient, cfg.ERPSessionTTL)
	} else {
		log.Println("[WARN] ERP_SESSION_CHECK is off: disabled ERP accounts keep access until their token expires")
	}

	app := fiber.New(fiber.Config{
		AppName:      "backoffice-api",
		ErrorHandler: middleware.ErrorHandler,
		BodyLimit:    4 * 1024 * 1024,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.AllowedOrigins,
		AllowHeaders:  "Origin, Content-Type, Accept, Authorization",
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		ExposeHeaders: "Content-Disposition",
	}))

	if len(cfg.ProductCodeHMACSecret) < 32 {
		log.Println("[WARN] PRODUCT_CODE_HMAC_SECRET is missing or shorter than 32 characters: product code export is disabled until it is set (same value as line-oa-api)")
	}

	pointRulesHandler, pointRulesJobs := wirePointRules(db)
	couponHandler, couponJobs := wireCoupon(db)
	rewardHandler, rewardJobs := wireReward(db, erpClient, lineNotifier)

	route.Register(app, route.Handlers{
		Health:         handler.NewHealthHandler(sqlDB),
		Member:         memberHandler,
		Points:         pointsHandler,
		StaffRole:      staffRoleHandler,
		SalesDashboard: wireSalesDashboard(erpClient),
		Upload:         uploadHandler,
		Promotion:      wirePromotion(db),
		PointRules:     pointRulesHandler,
		Coupon:         couponHandler,
		Reward:         rewardHandler,
		MemberReport:   wireMemberReport(db),
		Consent:        wireConsent(db),
		ProductCode:    wireProductCode(db, cfg.ProductCodeHMACSecret),
		Privacy:        privacyHandler,
	}, middleware.StaffAuth(authCfg))

	// Background jobs. Modules append their jobs to workerJobs; an empty list
	// still starts the runner so the wiring is exercised.
	var workerJobs []worker.Job
	workerJobs = append(workerJobs, pointRulesJobs...)
	workerJobs = append(workerJobs, couponJobs...)
	workerJobs = append(workerJobs, rewardJobs...)
	workerJobs = append(workerJobs, lineJobs...)
	workerJobs = append(workerJobs, privacyJobs...)
	workers := worker.New(workerJobs, log.Default())
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	if cfg.WorkerEnabled {
		workers.Start(workerCtx)
		log.Printf("[INFO] worker runner started (%d jobs)", len(workerJobs))
	} else {
		log.Println("[INFO] worker runner disabled (WORKER_ENABLED=false)")
	}

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatalf("[FATAL] server: %v", err)
		}
	}()
	log.Printf("[INFO] backoffice-api listening on :%s (env=%s)", cfg.Port, cfg.Environment)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("[INFO] shutting down")
	stopWorkers()
	workers.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("[WARN] shutdown: %v", err)
	}
	_ = sqlDB.Close()
}
