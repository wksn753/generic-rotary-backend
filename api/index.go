package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wksn753/kitende-rotary/internal/handlers"
	"github.com/wksn753/kitende-rotary/internal/infrastructure"
	"github.com/wksn753/kitende-rotary/internal/mail"
	"github.com/wksn753/kitende-rotary/internal/operations"
	"github.com/wksn753/kitende-rotary/internal/pkg"
)

var (
	router            *gin.Engine
	initOnce          sync.Once
	initErr           error
	operationsService *operations.Service
)

func setup() {
	dsn := strings.TrimSpace(os.Getenv("dsn"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		initErr = fmt.Errorf("database connection not set: configure dsn or DATABASE_URL")
		return
	}

	db, err := pkg.InitializeDatabase(dsn)
	if err != nil {
		initErr = fmt.Errorf("failed to connect to database: %w", err)
		return
	}
	// Keep Vercel cold starts lightweight. Schema migrations and historical
	// roster backfills must be run as explicit maintenance/deployment work, not
	// while a serverless request is waiting for the function to initialize.
	operationsService = operations.NewService(db)
	visitorRepo := infrastructure.NewVisitorInfrastructure(db)
	visitorHandler := handlers.NewVisitorHandler(visitorRepo, operationsService)
	operationsHandler := handlers.NewOperationsHandler(operationsService)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/api")
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "startup": "fast", "commit": strings.TrimSpace(os.Getenv("VERCEL_GIT_COMMIT_SHA"))})
	})
	registerRoutes(api, visitorHandler, operationsHandler)
	router = r
}

func registerRoutes(api *gin.RouterGroup, visitorHandler *handlers.VisitorHandler, operationsHandler *handlers.OperationsHandler) {
	api.POST("/register", visitorHandler.RegisterVisitor)
	api.GET("/visitors/lookup", visitorHandler.LookupVisitor)
	api.POST("/visitors/lookup", visitorHandler.LookupVisitor)
	api.GET("/clubs", visitorHandler.GetRotaryClubs)
	// Backward-compatible attendance paths used by the existing Next.js proxy.
	api.GET("/attendance", requireAdminAPIKey(), visitorHandler.GetAttendance)
	api.GET("/attendance/summary", requireAdminAPIKey(), visitorHandler.GetAttendanceSummary)

	admin := api.Group("/admin")
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

	api.GET("/jobs/run", requireJobKey(), operationsHandler.RunJobs)
	api.POST("/jobs/run", requireJobKey(), operationsHandler.RunJobs)

	legacy := api.Group("")
	legacy.Use(requireAdminAPIKey())
	mail.RegisterRoutes(legacy)
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

// Handler is the Vercel serverless entrypoint. Scheduled work is explicitly
// invoked through /api/jobs/run (Vercel Cron) so mail jobs remain durable.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(setup)
	if initErr != nil {
		log.Printf("api initialization error: %v", initErr)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"success":false,"code":"INIT_FAILED","message":"Backend initialization failed. Check the database configuration and club-operations migration in the backend deployment logs."}`))
		return
	}

	// Keep a hard upper bound for a serverless invocation doing job work.
	if strings.HasSuffix(r.URL.Path, "/jobs/run") {
		ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
	}
	router.ServeHTTP(w, r)
}
