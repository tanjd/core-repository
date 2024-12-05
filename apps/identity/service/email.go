package service

import "github.com/rs/zerolog/log"

type SimulatedEmailSender struct {
}

func (s *SimulatedEmailSender) SendEmailVerification(to string, token string) error {
	subject := "Verify Your Account"
	body := "Thank you for creating an account. Please verify your email by clicking the link below:\n\n" +
		"http://localhost:8888/verify?token=" + token + "\n\n" +
		"If you did not request this, please ignore this email."

	log.Info().
		Str("to", to).
		Str("subject", subject).
		Str("body", body).
		Msg("Simulated email sent")
	return nil
}
