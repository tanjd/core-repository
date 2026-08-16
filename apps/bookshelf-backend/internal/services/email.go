// Package services contains the business logic services for the bookshelf app.
package services

import (
	"fmt"
	"net"
	"net/smtp"

	"github.com/rs/zerolog/log"
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
func (s *EmailService) SendEmail(recipient, subject, html string) error {
	to := recipient
	if s.env == "dev" && s.devEmailOverride != "" {
		log.Debug().Str("original", recipient).Str("override", s.devEmailOverride).Msg("email: dev override active")
		to = s.devEmailOverride
	}
	if s.host == "" {
		log.Warn().Str("to", to).Str("subject", subject).Msg("email skipped: SMTP_HOST not set")
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

	log.Debug().Str("to", to).Str("subject", subject).Msg("email sent")
	return nil
}
