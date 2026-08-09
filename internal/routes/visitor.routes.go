package routes

// import (
// 	"github.com/gin-gonic/gin"
// 	"github.com/wksn753/kitende-rotary/internal/handlers"
// )

// type VisitorRoutes struct {
// 	router         *gin.RouterGroup
// 	visitorHandler *handlers.VisitorHandler
// }

// func NewVisitorRoutes(router *gin.RouterGroup) *VisitorRoutes {
// 	return &VisitorRoutes{
// 		router:         router,
// 		visitorHandler: handlers.NewVisitorHandler(visitorRepo),
// 	}
// }

// func (v *VisitorRoutes) RegisterRoutes() {
// 	guest := v.router.Group("/register")
// 	{
// 		guest.POST("/", v.visitorHandler.RegisterVisitor)
// 	}
// }
