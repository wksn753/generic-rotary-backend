package mail

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	// Add any dependencies or configuration needed for the handler
}

func NewHandler() *Handler {
	return &Handler{
		// Initialize any dependencies or configuration here
	}
}

// Add methods for the handler as needed	
func (h *Handler) SendRegistrationConfirmation(c *gin.Context)  {
	// Call the SendRegistrationConfirmation function from reg.mail.go
	type RequestBody struct {
		GuestEmail       string `json:"guest_email"`
		GuestName        string `json:"guest_name"`
		EventTime        string `json:"event_time"`
		RegistrationID   string `json:"registration_id"`
		EventDetailsURL  string `json:"event_details_url"`
		ContactPhone     string `json:"contact_phone"`
	}

	var reqBody RequestBody
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	

	guestEmail := reqBody.GuestEmail
	guestName := reqBody.GuestName
	eventTime := reqBody.EventTime
	registrationID := reqBody.RegistrationID
	eventDetailsURL := reqBody.EventDetailsURL
	contactPhone := reqBody.ContactPhone
	err := SendRegistrationConfirmation(c, guestEmail, guestName, eventTime, registrationID, eventDetailsURL, contactPhone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Registration confirmation sent successfully"})
}