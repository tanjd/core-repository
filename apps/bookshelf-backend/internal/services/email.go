// Package services contains the business logic services for the bookshelf app.
package services

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog"
)

// EmailService sends transactional emails over SMTP.
type EmailService struct {
	host             string
	port             string
	username         string
	password         string
	from             string
	env              string
	devEmailOverride string
}

// NewEmailService creates a new EmailService.
func NewEmailService(host, port, username, password, from, env, devEmailOverride string) *EmailService {
	return &EmailService{
		host:             host,
		port:             port,
		username:         username,
		password:         password,
		from:             from,
		env:              env,
		devEmailOverride: devEmailOverride,
	}
}

// SendEmail sends an HTML email over SMTP. net/smtp.SendMail negotiates
// STARTTLS automatically when the server advertises it (true of every
// mainstream SMTP provider on port 587), so no manual TLS handling is
// needed. If no host is configured the call is silently skipped — lets
// email stay optional for local/internal testing.
func (s *EmailService) SendEmail(ctx context.Context, recipient, subject, html string) error {
	to := recipient
	if s.env == "dev" && s.devEmailOverride != "" {
		zerolog.Ctx(ctx).Debug().Str("original", recipient).Str("override", s.devEmailOverride).Msg("email: dev override active")
		to = s.devEmailOverride
	}
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("email: recipient or subject contains invalid control characters")
	}
	if s.host == "" {
		zerolog.Ctx(ctx).Warn().Str("to", to).Str("subject", subject).Str("body", html).
			Msg("email skipped: SMTP_HOST not set — printing body here so codes/links are still usable for local testing")
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, html,
	)

	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	addr := net.JoinHostPort(s.host, s.port)
	if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("email: send: %w", err)
	}

	zerolog.Ctx(ctx).Debug().Str("to", to).Str("subject", subject).Msg("email sent")
	return nil
}

// SendEmailAsync sends an email in a background goroutine so callers on the
// HTTP request path don't block on the SMTP round trip. Errors are logged
// rather than returned — every call site already treats email delivery as
// best-effort. ctx is only used for its logger (carried into the goroutine
// for request correlation); the send itself isn't cancelled if ctx ends.
func (s *EmailService) SendEmailAsync(ctx context.Context, recipient, subject, html string) {
	go func() {
		if err := s.SendEmail(ctx, recipient, subject, html); err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Str("to", recipient).Str("subject", subject).Msg("async email send failed")
		}
	}()
}
