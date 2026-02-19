package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const brevoAPI = "https://api.brevo.com/v3/smtp/email"

// BrevoSendRequest matches Brevo API v3 send transactional email body.
type BrevoSendRequest struct {
	Sender      BrevoSender   `json:"sender"`
	To          []BrevoTo     `json:"to"`
	Subject     string        `json:"subject"`
	HTMLContent string        `json:"htmlContent"`
	ReplyTo     *BrevoReplyTo `json:"replyTo,omitempty"`
}

type BrevoSender struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type BrevoTo struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type BrevoReplyTo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Sender sends transactional emails (welcome, invite, account updated). Nil = no-op.
type Sender interface {
	SendWelcome(ctx context.Context, toEmail, firstName string) error
	SendInvite(ctx context.Context, toEmail, inviteLink, orgName, role, subject string) error
	SendAccountUpdated(ctx context.Context, toEmail, firstName string) error
}

// BrevoClient sends emails via Brevo (Sendinblue) API. Same env as Express: SENDINBLUE_API_KEY, MAIL_FROM.
type BrevoClient struct {
	APIKey   string
	MailFrom string
	Client   *http.Client
}

func (c *BrevoClient) from() string {
	if c.MailFrom != "" {
		return c.MailFrom
	}
	return "noreply@troo.earth"
}

// send sends one email via Brevo API.
func (c *BrevoClient) send(ctx context.Context, toEmail, subject, html string) error {
	if c.APIKey == "" {
		return nil
	}
	body := BrevoSendRequest{
		Sender:      BrevoSender{Email: c.from(), Name: "troo.earth"},
		To:          []BrevoTo{{Email: toEmail}},
		Subject:     subject,
		HTMLContent: html,
		ReplyTo:     &BrevoReplyTo{Email: "support@troo.earth", Name: "troo.earth Support"},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("brevo send failed: status %d", resp.StatusCode)
	}
	return nil
}

// SendWelcome sends the welcome email after account creation (Express accountCreatedTemplate).
func (c *BrevoClient) SendWelcome(ctx context.Context, toEmail, firstName string) error {
	if c.APIKey == "" {
		return nil
	}
	if firstName == "" {
		firstName = "there"
	}
	content := welcomeContent(firstName)
	return c.send(ctx, toEmail, "Welcome to troo.earth!", EmailLayout(content))
}

// SendInvite sends the invitation email (Express invitationTemplate). Subject is caller-provided (e.g. "You have been invited to join {org}" or "Reminder: Invitation to join {org}").
func (c *BrevoClient) SendInvite(ctx context.Context, toEmail, inviteLink, orgName, role, subject string) error {
	if c.APIKey == "" {
		return nil
	}
	content := invitationContent(inviteLink, orgName, role)
	return c.send(ctx, toEmail, subject, EmailLayout(content))
}

// SendAccountUpdated sends the account-updated notification (Express accountUpdatedTemplate).
func (c *BrevoClient) SendAccountUpdated(ctx context.Context, toEmail, firstName string) error {
	if c.APIKey == "" {
		return nil
	}
	if firstName == "" {
		firstName = "there"
	}
	content := accountUpdatedContent(firstName)
	return c.send(ctx, toEmail, "Your troo.earth Account Was Updated", EmailLayout(content))
}

// welcomeContent matches Express accountCreatedTemplate content (inside layout).
func welcomeContent(userName string) string {
	dashboardURL := "https://troo.earth/"
	return fmt.Sprintf(`
    <h1>Welcome to the Movement, %s!</h1>
    <p>Thank you for joining <strong>troo.earth</strong>. Your account has been successfully created, and you are now part of a community dedicated to transparent and impactful carbon offsetting.</p>
    <p>We believe that investing in our planet should be simple, transparent, and accessible. You can now browse our verified projects and start making a difference today.</p>
    <center>
      <a href="%s" class="troo-button">Explore the Marketplace</a>
    </center>
    <p style="margin-top: 20px; font-size: 14px; color: #666;">
      If you did not sign up for this account, please contact our support team immediately.
    </p>
    <p>— The troo.earth Team</p>
`, EscapeHTML(userName), dashboardURL)
}

// invitationContent matches Express invitationTemplate content.
func invitationContent(inviteLink, orgName, role string) string {
	return fmt.Sprintf(`
    <h1>You've Been Invited to Join %s</h1>
    <p>You have been invited to join your organization on <strong>troo.earth</strong> as a <strong>%s</strong>.</p>
    <p>Click the button below to accept your invitation and get started:</p>
    <center>
      <a href="%s" class="troo-button">Accept Invitation</a>
    </center>
    <p style="margin-top:20px;font-size:14px;color:#666;">
      This invitation link will expire in 7 days. If you were not expecting this invitation, you can safely ignore this email.
    </p>
    <p>— The troo.earth Team</p>
`, EscapeHTML(orgName), EscapeHTML(role), inviteLink)
}

// accountUpdatedContent matches Express accountUpdatedTemplate content.
func accountUpdatedContent(userName string) string {
	accountURL := "https://troo.earth/"
	return fmt.Sprintf(`
    <h1>Account Details Successfully Updated</h1>
    <p>Hi %s,</p>
    <p>This is a quick notification to let you know that the information associated with your <strong>troo.earth</strong> account has been successfully updated.</p>
    <center>
      <a href="%s" class="troo-button">View Your Account</a>
    </center>
    <p><strong>Security Notice:</strong><br>
    If you did not make this change, someone else may have access to your account. Please <a href="mailto:support@troo.earth">contact support</a> immediately to secure your profile.</p>
    <p>— The troo.earth Team</p>
`, EscapeHTML(userName), accountURL)
}
