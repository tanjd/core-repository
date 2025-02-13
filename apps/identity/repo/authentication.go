package repo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (r *InMemoryRepo) StoreVerificationToken(userID uuid.UUID, token string, expiration time.Time) error {
	key := formatVerificationTokenKey(token)
	r.emailVerificationTokens[key] = EmailVerificationData{
		UserID:     userID,
		Expiration: expiration,
	}
	log.Info().
		Interface("VerificationTokens", r.emailVerificationTokens).
		Msg("Saved email verification token")
	return nil
}

func (r *InMemoryRepo) RetrieveVerificationToken(token string) (*EmailVerificationData, error) {
	key := formatVerificationTokenKey(token)
	data, exists := r.emailVerificationTokens[key]
	if !exists {
		return nil, ErrVerificationTokenNotFound
	}
	return &data, nil
}

func (r *InMemoryRepo) DeleteVerificationToken(token string) error {
	key := formatVerificationTokenKey(token)
	if _, exists := r.emailVerificationTokens[key]; !exists {
		return ErrVerificationTokenNotFound
	}
	delete(r.emailVerificationTokens, key)

	log.Debug().
		Interface("VerificationTokens", r.emailVerificationTokens).
		Msg("Delete Verification Token")
	return nil
}

func formatVerificationTokenKey(token string) string {
	return fmt.Sprintf("email_verification:%s", token)
}
