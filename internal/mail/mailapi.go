package mail

import (
	"fmt"
	"net/http"
	"os"
	"encoding/json"
	"bytes"
)


func GetMailToken()(string, error) {
	// Get the token from the environment variable
	username := os.Getenv("MAIL_USER_NAME")
	password := os.Getenv("MAIL_PASSWORD")
	
	type MailTokenRequest struct {
		Username string `json:"email"`
		Password string `json:"pass"`
	}

	req := &MailTokenRequest{
		Username: username,
		Password: password,
	}

	body,err:=json.Marshal(req)

	if err != nil {
		return "", err
	}

	bodyBuffer := bytes.NewBuffer(body)
	httpClient := &http.Client{}
	reqest, err := http.NewRequest("POST", "https://mail.savaralabs.com/login", bodyBuffer)
	if err != nil {
		return "", err
	}

	reqest.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(reqest)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get mail token, status code: %d", resp.StatusCode)
	}

	type MailTokenResponse struct {
		Token string `json:"token"`
	}

	var mailTokenResponse MailTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&mailTokenResponse); err != nil {
		return "", err
	}

	return mailTokenResponse.Token, nil
}

func SendMail(to,subject,body string) error {
	token, err := GetMailToken()
	if err != nil {
		return fmt.Errorf("failed to get mail token: %w", err)
	}

	var mailRequest struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	mailRequest.To = to
	mailRequest.Subject = subject
	mailRequest.Body = body

	requestBody, err := json.Marshal(mailRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal mail request: %w", err)
	}

	httpClient := &http.Client{}
	req, err := http.NewRequest("POST", "https://mail.savaralabs.com/send", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create mail request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send mail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send mail, status code: %d", resp.StatusCode)
	}

	return nil
}

