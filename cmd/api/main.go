package main

import (
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
	"github.com/wksn753/kitende-rotary/internal/models"
	"github.com/wksn753/kitende-rotary/internal/pkg"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Default().Println("Error loading .env file")
	}

	dsn := strings.TrimSpace(os.Getenv("dsn"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		log.Fatal("database connection not set: configure dsn or DATABASE_URL")
	}

	// --- Database ---
	gormDB, err := pkg.InitializeDatabase(dsn)

	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// AutoMigrate creates/updates tables to match your struct definitions.
	// RegisterRecord is used as an attendance/check-in table; new columns are
	// added safely when the app starts.
	if err := gormDB.AutoMigrate(&models.RegisterRecord{}, &models.RotaryClub{}); err != nil {
		log.Fatalf("auto migration failed: %v", err)
	}
	log.Println("database migrated successfully")

	// --- Repositories / Handlers ---
	// NOTE: adjust this constructor name to match whatever your
	// repository package actually exposes (e.g. NewVisitorRepository).
	visitorRepo := infrastructure.NewVisitorInfrastructure(gormDB)
	visitorHandler := handlers.NewVisitorHandler(visitorRepo)

	// --- Router ---
	serverHandler := gin.New()
	serverHandler.Use(gin.Logger())
	serverHandler.Use(gin.Recovery())

	router := serverHandler.Group("/api")

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	registerRoutes(router, visitorHandler)

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	
	s := &http.Server{
		Addr:           ":" + port,
		Handler:        serverHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	fmt.Printf("Starting server on port %s...\n", port)
	log.Fatal(s.ListenAndServe())
	log.Fatal(s.ListenAndServe())

}

func registerRoutes(router *gin.RouterGroup, visitorHandler *handlers.VisitorHandler) {
	router.POST("/register", visitorHandler.RegisterVisitor)
	router.GET("/visitors/lookup", visitorHandler.LookupVisitor)
	router.POST("/visitors/lookup", visitorHandler.LookupVisitor)
	router.GET("/clubs", visitorHandler.GetRotaryClubs)
	router.GET("/attendance", requireAdminAPIKey(), visitorHandler.GetAttendance)
	router.GET("/attendance/summary", requireAdminAPIKey(), visitorHandler.GetAttendanceSummary)
}

// requireAdminAPIKey protects admin-only backend reads when ADMIN_API_KEY is set.
// It is optional so existing deployments keep working during rollout, but production
// should set ADMIN_API_KEY and let the Next.js admin proxy pass X-Admin-API-Key.
func requireAdminAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
		if expected == "" {
			c.Next()
			return
		}

		supplied := strings.TrimSpace(c.GetHeader("X-Admin-API-Key"))
		if supplied == "" {
			authorization := strings.TrimSpace(c.GetHeader("Authorization"))
			supplied = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		}

		if supplied == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Admin access required",
			})
			return
		}

		c.Next()
	}
}
