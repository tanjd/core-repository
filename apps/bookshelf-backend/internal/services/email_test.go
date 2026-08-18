package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendEmail_SkipsWhenNoHostConfigured(t *testing.T) {
	// With no SMTP_HOST configured, SendEmail must no-op rather than attempt
	// a network connection — this is the seam that lets handler/workflow
	// tests exercise email-sending code paths without touching the network.
	svc := NewEmailService("", "", "", "", "from@example.com", "prod", "")

	err := svc.SendEmail(context.Background(), "to@example.com", "subject", "<p>body</p>")

	assert.NoError(t, err)
}

func TestSendEmail_SkipsEvenWithDevOverride(t *testing.T) {
	svc := NewEmailService("", "", "", "", "from@example.com", "dev", "override@example.com")

	err := svc.SendEmail(context.Background(), "to@example.com", "subject", "<p>body</p>")

	assert.NoError(t, err)
}

func TestSendEmail_RejectsCRLFInRecipient(t *testing.T) {
	// The guard must fire even with no SMTP_HOST configured (host=="" would
	// otherwise no-op before ever reaching the header construction), proving
	// it isn't skippable via the local/dev no-op path.
	svc := NewEmailService("", "", "", "", "from@example.com", "prod", "")

	err := svc.SendEmail(context.Background(), "to@example.com\r\nBcc: attacker@evil.com", "subject", "<p>body</p>")

	assert.Error(t, err)
}

func TestSendEmail_RejectsCRLFInSubject(t *testing.T) {
	svc := NewEmailService("", "", "", "", "from@example.com", "prod", "")

	err := svc.SendEmail(context.Background(), "to@example.com", "subject\r\nBcc: attacker@evil.com", "<p>body</p>")

	assert.Error(t, err)
}
