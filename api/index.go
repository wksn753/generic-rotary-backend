package api

// import (
// 	"crypto/subtle"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"sync"

// 	"github.com/gin-gonic/gin"
// 	"github.com/wksn753/kitende-rotary/internal/handlers"
// 	"github.com/wksn753/kitende-rotary/internal/infrastructure"
// 	"github.com/wksn753/kitende-rotary/internal/models"
// 	"github.com/wksn753/kitende-rotary/internal/pkg"
// )

// var (
// 	router   *gin.Engine
// 	initOnce sync.Once
// 	initErr  error
// )

// // setup runs once per warm container. It connects to the database,
// // migrates models, wires up handlers, and builds the Gin router.
// // It never calls log.Fatal / os.Exit — a serverless function that
// // exits the process just looks like a 500 with no useful message.
// func setup() {
// 	dsn := strings.TrimSpace(os.Getenv("dsn"))
// 	if dsn == "" {
// 		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
// 	}
// 	if dsn == "" {
// 		initErr = fmt.Errorf("database connection not set: configure dsn or DATABASE_URL")
// 		return
// 	}

// 	gormDB, err := pkg.InitializeDatabase(dsn)
// 	if err != nil {
// 		initErr = fmt.Errorf("failed to connect to database: %w", err)
// 		return
// 	}

// 	if err := gormDB.AutoMigrate(&models.RegisterRecord{}, &models.RotaryClub{}); err != nil {
// 		initErr = fmt.Errorf("auto migration failed: %w", err)
// 		return
// 	}
// 	log.Println("database migrated successfully")

// 	visitorRepo := infrastructure.NewVisitorInfrastructure(gormDB)
// 	visitorHandler := handlers.NewVisitorHandler(visitorRepo)

// 	gin.SetMode(gin.ReleaseMode)
// 	r := gin.New()
// 	r.Use(gin.Logger())
// 	r.Use(gin.Recovery())

// 	api := r.Group("/api")
// 	api.GET("/ping", func(c *gin.Context) {
// 		c.JSON(http.StatusOK, gin.H{"message": "pong"})
// 	})
// 	registerRoutes(api, visitorHandler)

// 	router = r
// }

// func registerRoutes(router *gin.RouterGroup, visitorHandler *handlers.VisitorHandler) {
// 	router.POST("/register", visitorHandler.RegisterVisitor)
// 	router.GET("/visitors/lookup", visitorHandler.LookupVisitor)
// 	router.POST("/visitors/lookup", visitorHandler.LookupVisitor)
// 	router.GET("/clubs", visitorHandler.GetRotaryClubs)
// 	router.GET("/attendance", requireAdminAPIKey(), visitorHandler.GetAttendance)
// 	router.GET("/attendance/summary", requireAdminAPIKey(), visitorHandler.GetAttendanceSummary)
// }

// // requireAdminAPIKey protects admin-only backend reads when ADMIN_API_KEY is set.
// // It is optional so existing deployments keep working during rollout, but production
// // should set ADMIN_API_KEY and let the Next.js admin proxy pass X-Admin-API-Key.
// func requireAdminAPIKey() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		expected := strings.TrimSpace(os.Getenv("ADMIN_API_KEY"))
// 		if expected == "" {
// 			c.Next()
// 			return
// 		}

// 		supplied := strings.TrimSpace(c.GetHeader("X-Admin-API-Key"))
// 		if supplied == "" {
// 			authorization := strings.TrimSpace(c.GetHeader("Authorization"))
// 			supplied = strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
// 		}

// 		if supplied == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
// 				"success": false,
// 				"code":    "UNAUTHORIZED",
// 				"message": "Admin access required",
// 			})
// 			return
// 		}

// 		c.Next()
// 	}
// }

// // Handler is the exported entrypoint Vercel's Go runtime invokes for
// // every request to this function. It lazily initializes the router
// // and database connection once per warm container (sync.Once), then
// // delegates to Gin.
// func Handler(w http.ResponseWriter, r *http.Request) {
// 	initOnce.Do(setup)

// 	if initErr != nil {
// 		log.Println("initialization error:", initErr)
// 		http.Error(w, "server initialization failed: "+initErr.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	router.ServeHTTP(w, r)
// }