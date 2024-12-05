package repo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func (r *InMemoryRepo) SaveVerificationToken(userID, token uuid.UUID, expiration time.Time) error {
	key := fmt.Sprintf("email_verification:%s", token)
	r.emailVerificationTokens[key] = EmailVerificationData{
		UserID:     userID,
		Expiration: expiration,
	}
	log.Info().
		Interface("emailVerificationList", r.emailVerificationTokens).
		Msg("Saved email verification token")
	return nil
}
func (r *InMemoryRepo) GetVerificationTokenData(token uuid.UUID) error {
	// unimplemented
	return nil
}
