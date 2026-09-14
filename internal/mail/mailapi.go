package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var savaraHTTPClient = &http.Client{Timeout: 20 * time.Second}

func savaraMailBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SAVARA_MAIL_BASE_URL")), "/")
	if base == "" {
		base = "https://mail.savaralabs.com"
	}
	return base
}

// GetMailToken authenticates against SavaraLabsMail. It intentionally obtains
// a token for the send operation rather than relying on a long-lived token so
// every outgoing email follows the required login -> bearer token -> send flow.
func GetMailToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return GetMailTokenContext(ctx)
}

func GetMailTokenContext(ctx context.Context) (string, error) {
	username := strings.TrimSpace(os.Getenv("MAIL_USER_NAME"))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("SAVARA_MAIL_EMAIL"))
	}
	password := os.Getenv("MAIL_PASSWORD")
	if password == "" {
		password = os.Getenv("SAVARA_MAIL_PASSWORD")
	}
	if username == "" || password == "" {
		return "", fmt.Errorf("mail: MAIL_USER_NAME/SAVARA_MAIL_EMAIL and MAIL_PASSWORD/SAVARA_MAIL_PASSWORD are required")
	}

	payload := map[string]string{"email": username, "pass": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, savaraMailBaseURL()+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := savaraHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("mail: login failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}

	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		Data        struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("mail: decode login response: %w", err)
	}

	token := strings.TrimSpace(result.Token)
	if token == "" {
		token = strings.TrimSpace(result.AccessToken)
	}
	if token == "" {
		token = strings.TrimSpace(result.Data.Token)
	}
	if token == "" {
		return "", fmt.Errorf("mail: login response did not contain a token")
	}
	return token, nil
}

func SendMail(to, subject, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return SendMailContext(ctx, to, subject, body)
}

func SendMailContext(ctx context.Context, to, subject, body string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("mail: recipient is required")
	}

	// Required sequence: authenticate immediately before each send, then use
	// the returned token on /send.
	token, err := GetMailTokenContext(ctx)
	if err != nil {
		return fmt.Errorf("mail: failed to get Savara Mail token: %w", err)
	}

	payload := map[string]string{"to": to, "subject": subject, "body": body}
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mail: marshal send request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, savaraMailBaseURL()+"/send", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("mail: create send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := savaraHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mail: send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail: send failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}
