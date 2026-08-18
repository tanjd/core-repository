package services

import (
	"context"

	"github.com/rs/zerolog"
)

// SMSService sends an OTP code to a phone number. The only implementation
// today is MockSMSService — there's no real SMS provider wired up yet — but
// handlers depend on this interface rather than the concrete type so a real
// provider (e.g. Twilio, SNS) can be substituted later without touching
// call sites.
type SMSService interface {
	SendOTP(ctx context.Context, phone, code string) error
}

// MockSMSService stands in for a real SMS provider. It never sends anything;
// callers that need the code to reach a human today return it directly in
// the API response (clearly labeled as mocked) rather than relying on actual
// delivery — this just logs for local traceability.
type MockSMSService struct{}

// NewMockSMSService creates a new MockSMSService.
func NewMockSMSService() *MockSMSService {
	return &MockSMSService{}
}

func (s *MockSMSService) SendOTP(ctx context.Context, phone, code string) error {
	zerolog.Ctx(ctx).Warn().Str("phone", phone).Str("code", code).
		Msg("sms skipped: no SMS provider configured — phone verification is mocked")
	return nil
}
