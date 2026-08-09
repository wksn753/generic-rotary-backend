package mail

import (
	"context"
)

// SendRegistrationConfirmation renders the rotary-kitende-confirmation
// template and sends it to a single guest. This is the pattern to copy
// for any other templated email (reminders, thank-yous, etc.) — render,
// then hand the HTML straight to EmailPayload.HTMLBody.
func SendRegistrationConfirmation(ctx context.Context, guestEmail, guestName, eventTime, registrationID, eventDetailsURL, contactPhone string) error {
	html, err := RenderTemplate("rotary-kitende-confirmation", map[string]any{
		"GUEST_NAME":        guestName,
		"EVENT_TIME":        eventTime,
		"REGISTRATION_ID":   registrationID,
		"EVENT_DETAILS_URL": eventDetailsURL,
		"CONTACT_PHONE":     contactPhone,
	})
	if err != nil {
		return err
	}

	return SendMail(guestEmail,"You're registered — Nakawa Rotary Fellowship", html)
	// return SendViaGraph(ctx, EmailPayload{
	// 	To:       []string{guestEmail},
	// 	Cc:       []string{"stephen.wassanyi@gmail.com"},
	// 	Subject:  "You're registered — Kitende Breeze Fellowship",
	// 	HTMLBody: html,
	// })
}
