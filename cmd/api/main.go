package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/wksn753/kitende-rotary/internal/handlers"
	"github.com/wksn753/kitende-rotary/internal/infrastructure"
	"github.com/wksn753/kitende-rotary/internal/mail"
	"github.com/wksn753/kitende-rotary/internal/operations"
	"github.com/wksn753/kitende-rotary/internal/pkg"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not loaded; using process environment")
	}

	dsn := strings.TrimSpace(os.Getenv("dsn"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		log.Fatal("database connection not set: configure dsn or DATABASE_URL")
	}

	gormDB, err := pkg.InitializeDatabase(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// IMPORTANT: do not run AutoMigrate or historical roster backfills in the
	// request-serving startup path. Vercel requires the Go server to begin
	// listening quickly; GORM schema introspection across all operations tables
	// can exceed the platform startup window. Run SQL migrations separately.
	log.Println("database configured; startup migrations/backfill skipped")

	operationsService := operations.NewService(gormDB)
	visitorRepo := infrastructure.NewVisitorInfrastructure(gormDB)
	visitorHandler := handlers.NewVisitorHandler(visitorRepo, operationsService)
	operationsHandler := handlers.NewOperationsHandler(operationsService)

	serverHandler := gin.New()
	serverHandler.Use(gin.Logger(), gin.Recovery())
	router := serverHandler.Group("/api")
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "startup": "fast", "commit": strings.TrimSpace(os.Getenv("VERCEL_GIT_COMMIT_SHA"))})
	})
	registerRoutes(router, visitorHandler, operationsHandler)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	operationsService.StartWorker(workerCtx)

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	s := &http.Server{Addr: ":" + port, Handler: serverHandler, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	fmt.Printf("Starting server on port %s...\n", port)
	log.Fatal(s.ListenAndServe())
}

func registerRoutes(router *gin.RouterGroup, visitorHandler *handlers.VisitorHandler, operationsHandler *handlers.OperationsHandler) {
	router.POST("/register", visitorHandler.RegisterVisitor)
	router.GET("/visitors/lookup", visitorHandler.LookupVisitor)
	router.POST("/visitors/lookup", visitorHandler.LookupVisitor)
	router.GET("/clubs", visitorHandler.GetRotaryClubs)
	// Backward-compatible attendance paths used by the existing Next.js proxy.
	router.GET("/attendance", requireAdminAPIKey(), visitorHandler.GetAttendance)
	router.GET("/attendance/summary", requireAdminAPIKey(), visitorHandler.GetAttendanceSummary)

	admin := router.Group("/admin")
	admin.Use(requireAdminAPIKey())
	admin.GET("/attendance", visitorHandler.GetAttendance)
	admin.GET("/attendance/summary", visitorHandler.GetAttendanceSummary)
	admin.GET("/dashboard", operationsHandler.Dashboard)
	admin.GET("/members", operationsHandler.ListMembers)
	admin.POST("/members", operationsHandler.CreateMember)
	admin.PATCH("/members/:id", operationsHandler.UpdateMember)
	admin.GET("/donations", operationsHandler.ListDonations)
	admin.POST("/donations", operationsHandler.CreateDonation)
	admin.DELETE("/donations/:id", operationsHandler.DeleteDonation)
	admin.GET("/goals", operationsHandler.ListGoals)
	admin.POST("/goals", operationsHandler.CreateGoal)
	admin.PATCH("/goals/:id", operationsHandler.UpdateGoal)
	admin.DELETE("/goals/:id", operationsHandler.DeleteGoal)
	admin.GET("/projects", operationsHandler.ListProjects)
	admin.POST("/projects", operationsHandler.CreateProject)
	admin.GET("/projects/:id", operationsHandler.GetProject)
	admin.PATCH("/projects/:id", operationsHandler.UpdateProject)
	admin.DELETE("/projects/:id", operationsHandler.DeleteProject)
	admin.POST("/projects/:id/transactions", operationsHandler.CreateProjectTransaction)
	admin.POST("/projects/:id/invoices", operationsHandler.CreateProjectInvoice)
	admin.GET("/campaigns", operationsHandler.ListCampaigns)
	admin.POST("/campaigns", operationsHandler.CreateCampaign)
	admin.GET("/email-templates", operationsHandler.ListEmailTemplates)
	admin.POST("/email-templates", operationsHandler.CreateEmailTemplate)
	admin.PATCH("/email-templates/:id", operationsHandler.UpdateEmailTemplate)
	admin.DELETE("/email-templates/:id", operationsHandler.DeleteEmailTemplate)

	// Cron/worker entrypoint. The mail work itself is entirely Go-backed; this
	// route lets serverless deployments wake the Go worker on a schedule.
	router.GET("/jobs/run", requireJobKey(), operationsHandler.RunJobs)
	router.POST("/jobs/run", requireJobKey(), operationsHandler.RunJobs)

	// Legacy manual mail endpoint retained for compatibility but no longer public.
	mailGroup := router.Group("")
	mailGroup.Use(requireAdminAPIKey())
	mail.RegisterRoutes(mailGroup)
}

func requireAdminAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
		if expected == "" {
			c.Next()
			return
		}
		supplied := suppliedToken(c)
		if supplied == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "UNAUTHORIZED", "message": "Admin access required"})
			return
		}
		c.Next()
	}
}

func requireJobKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		supplied := suppliedToken(c)
		keys := []string{strings.TrimSpace(os.Getenv("CRON_SECRET")), strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))}
		for _, key := range keys {
			if key != "" && supplied != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) == 1 {
				c.Next()
				return
			}
		}
		if keys[0] == "" && keys[1] == "" {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "UNAUTHORIZED", "message": "Job runner authorization required"})
	}
}

func suppliedToken(c *gin.Context) string {
	if value := strings.TrimSpace(c.GetHeader("X-Admin-API-Key")); value != "" {
		return value
	}
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(authorization) >= 7 && strings.EqualFold(authorization[:7], "Bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return authorization
}
