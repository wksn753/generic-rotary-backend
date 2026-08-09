package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

// EmailAttachment is a caller-supplied attachment. Content is raw bytes;
// base64 encoding happens internally.
type EmailAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// EmailPayload is the dynamic, caller-facing description of an email.
// Any combination of To/Cc/Bcc/ReplyTo can be set, so this can be used
// to send to a single address or to an arbitrary list of recipients.
type EmailPayload struct {
	To      []string
	Cc      []string
	Bcc     []string
	ReplyTo []string

	Subject string

	// Exactly one of HTMLBody / TextBody should normally be set.
	// If both are set, HTMLBody wins.
	HTMLBody string
	TextBody string

	Attachments []EmailAttachment

	// SaveToSentItems controls whether Graph copies the sent message
	// into the sender mailbox's Sent Items folder. Defaults to true
	// (Graph's own default) when left nil.
	SaveToSentItems *bool
}

// Validate checks that the payload is well-formed enough to send:
// at least one "To" recipient, and every address in To/Cc/Bcc/ReplyTo
// parses as a valid RFC 5322 address.
func (p EmailPayload) Validate() error {
	if len(p.To) == 0 {
		return fmt.Errorf("mail: at least one To recipient is required")
	}
	if p.Subject == "" {
		return fmt.Errorf("mail: subject is required")
	}
	if p.HTMLBody == "" && p.TextBody == "" {
		return fmt.Errorf("mail: either HTMLBody or TextBody is required")
	}

	var invalid []string
	checkAll := func(label string, addrs []string) {
		for _, a := range addrs {
			if _, err := mail.ParseAddress(a); err != nil {
				invalid = append(invalid, fmt.Sprintf("%s:%q", label, a))
			}
		}
	}
	checkAll("to", p.To)
	checkAll("cc", p.Cc)
	checkAll("bcc", p.Bcc)
	checkAll("replyTo", p.ReplyTo)

	if len(invalid) > 0 {
		return fmt.Errorf("mail: invalid address(es): %s", strings.Join(invalid, ", "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Graph API wire types
// ---------------------------------------------------------------------------

type graphAttachment struct {
	ODataType    string `json:"@odata.type"`
	Name         string `json:"name"`
	ContentType  string `json:"contentType"`
	ContentBytes string `json:"contentBytes"`
}

type graphRecipient struct {
	EmailAddress struct {
		Address string `json:"address"`
		Name    string `json:"name,omitempty"`
	} `json:"emailAddress"`
}

type graphMessage struct {
	Subject string `json:"subject"`
	Body    struct {
		ContentType string `json:"contentType"`
		Content     string `json:"content"`
	} `json:"body"`
	From          *graphRecipient   `json:"from,omitempty"`
	ToRecipients  []graphRecipient  `json:"toRecipients"`
	CcRecipients  []graphRecipient  `json:"ccRecipients,omitempty"`
	BccRecipients []graphRecipient  `json:"bccRecipients,omitempty"`
	ReplyTo       []graphRecipient  `json:"replyTo,omitempty"`
	Attachments   []graphAttachment `json:"attachments,omitempty"`
}

type sendMailRequest struct {
	Message         graphMessage `json:"message"`
	SaveToSentItems *bool        `json:"saveToSentItems,omitempty"`
}

// ---------------------------------------------------------------------------
// Token cache
// ---------------------------------------------------------------------------

type tokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

var cache = &tokenCache{}

// httpClient is shared across calls so we're not paying connection setup
// cost per-send, and so every request has a sane timeout.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// ---------------------------------------------------------------------------
// Sending
// ---------------------------------------------------------------------------

// SendViaGraph sends p on behalf of the mailbox configured via AZURE_MAIL,
// to whatever recipients p specifies. It is safe to call concurrently.
func SendViaGraph(ctx context.Context, p EmailPayload) error {
	if err := p.Validate(); err != nil {
		return err
	}

	sender := os.Getenv("AZURE_MAIL")
	if sender == "" {
		return fmt.Errorf("mail: AZURE_MAIL env var is not set")
	}

	token, err := getGraphToken(ctx)
	if err != nil {
		return fmt.Errorf("mail: failed to get graph token: %w", err)
	}

	msg := graphMessage{
		Subject:       p.Subject,
		ToRecipients:  toRecipients(p.To),
		CcRecipients:  toRecipients(p.Cc),
		BccRecipients: toRecipients(p.Bcc),
		ReplyTo:       toRecipients(p.ReplyTo),
	}
	msg.Body.ContentType, msg.Body.Content = resolveBody(p)

	// AZURE_MAIL_NAME lets the visible "From" name differ from whatever
	// display name is set on the mailbox itself in Exchange/Entra ID.
	// Note: some tenants enforce the mailbox's directory display name
	// via anti-spoofing transport rules regardless of what's sent here —
	// if this doesn't change what recipients see, the reliable fix is
	// updating the mailbox's actual Display Name in the Microsoft 365
	// admin center (or using a mailbox that's already named correctly).
	if displayName := os.Getenv("AZURE_MAIL_NAME"); displayName != "" {
		from := graphRecipient{}
		from.EmailAddress.Address = sender
		from.EmailAddress.Name = displayName
		msg.From = &from
	}

	for _, att := range p.Attachments {
		msg.Attachments = append(msg.Attachments, graphAttachment{
			ODataType:    "#microsoft.graph.fileAttachment",
			Name:         att.Filename,
			ContentType:  att.ContentType,
			ContentBytes: base64.StdEncoding.EncodeToString(att.Content),
		})
	}

	reqBody, err := json.Marshal(sendMailRequest{Message: msg, SaveToSentItems: p.SaveToSentItems})
	if err != nil {
		return fmt.Errorf("mail: failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/sendMail", url.PathEscape(sender))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("mail: failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mail: graph sendMail request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10)) // cap at 4KB
		return fmt.Errorf("mail: graph sendMail failed (status %d): %s", res.StatusCode, string(detail))
	}
	return nil
}

// resolveBody picks HTML over plain text when both are set, and returns
// the Graph contentType alongside the content.
func resolveBody(p EmailPayload) (contentType, content string) {
	if p.HTMLBody != "" {
		return "HTML", p.HTMLBody
	}
	return "Text", p.TextBody
}

// toRecipients converts a slice of plain address strings into Graph
// recipient objects. Assumes addresses were already validated via
// EmailPayload.Validate.
func toRecipients(addrs []string) []graphRecipient {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]graphRecipient, 0, len(addrs))
	for _, a := range addrs {
		var r graphRecipient
		r.EmailAddress.Address = a
		out = append(out, r)
	}
	return out
}

// ---------------------------------------------------------------------------
// Token acquisition
// ---------------------------------------------------------------------------

func getGraphToken(ctx context.Context) (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.token != "" && time.Now().Add(120*time.Second).Before(cache.expiresAt) {
		return cache.token, nil
	}

	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("mail: AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET must all be set")
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("scope", "https://graph.microsoft.com/.default")
	data.Set("grant_type", "client_credentials")

	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("mail: failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mail: token request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return "", fmt.Errorf("mail: token endpoint returned status %d: %s", res.StatusCode, string(detail))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("mail: failed to decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("mail: token response contained no access_token")
	}

	cache.token = result.AccessToken
	cache.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return cache.token, nil
}
