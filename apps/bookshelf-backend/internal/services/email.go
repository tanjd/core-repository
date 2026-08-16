// Package services contains the business logic services for the bookshelf app.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

// EmailService sends transactional emails via the Resend API.
type EmailService struct {
	apiKey           string
	from             string
	env              string
	devEmailOverride string
}

// NewEmailService creates a new EmailService.
func NewEmailService(apiKey, from, env, devEmailOverride string) *EmailService {
	return &EmailService{apiKey: apiKey, from: from, env: env, devEmailOverride: devEmailOverride}
}

type resendPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendEmail posts an email via Resend. If no API key is configured the call is
// silently skipped.
func (s *EmailService) SendEmail(recipient, subject, html string) error {
	to := recipient
	if s.env == "dev" && s.devEmailOverride != "" {
		log.Debug().Str("original", recipient).Str("override", s.devEmailOverride).Msg("email: dev override active")
		to = s.devEmailOverride
	}
	if s.apiKey == "" {
		log.Warn().Str("to", to).Str("subject", subject).Msg("email skipped: RESEND_API_KEY not set")
		return nil
	}

	payload := resendPayload{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		HTML:    html,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("email: send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("email: close response body")
		}
	}()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("email: resend returned status %d: %s", resp.StatusCode, respBody)
	}

	log.Debug().Str("to", to).Str("subject", subject).Msg("email sent")
	return nil
}
