package mail
import ("github.com/gin-gonic/gin")

func RegisterRoutes(router *gin.RouterGroup) {
	mailHandler := NewHandler()
	router.POST("/send-registration-confirmation", mailHandler.SendRegistrationConfirmation)
}