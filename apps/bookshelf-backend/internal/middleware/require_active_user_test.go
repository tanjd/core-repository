package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func TestRequireActiveUser(t *testing.T) {
	users := repotest.NewUserRepository()
	active := &models.User{Name: "Active", Email: "active@example.com", Role: "user"}
	require.NoError(t, users.Create(active))
	suspended := &models.User{Name: "Suspended", Email: "suspended@example.com", Role: "user", Suspended: true}
	require.NoError(t, users.Create(suspended))
	pending := &models.User{Name: "Pending", Email: "pending@example.com", Role: "user", PendingApproval: true}
	require.NoError(t, users.Create(pending))
	demotedAdmin := &models.User{Name: "Demoted", Email: "demoted@example.com", Role: "user"}
	require.NoError(t, users.Create(demotedAdmin))

	newCtx := func(userID uint, role string) context.Context {
		ctx := context.WithValue(context.Background(), userIDKey, userID)
		return context.WithValue(ctx, roleKey, role)
	}

	tests := []struct {
		name       string
		ctx        context.Context
		wantStatus int
		wantRole   string // only checked when wantStatus == 200
	}{
		{name: "unauthenticated request passes through", ctx: context.Background(), wantStatus: http.StatusOK},
		{name: "active user passes through", ctx: newCtx(active.ID, "user"), wantStatus: http.StatusOK, wantRole: "user"},
		{name: "suspended user is rejected", ctx: newCtx(suspended.ID, "user"), wantStatus: http.StatusForbidden},
		{name: "pending-approval user is rejected", ctx: newCtx(pending.ID, "user"), wantStatus: http.StatusForbidden},
		{name: "deleted user (stale token) is rejected", ctx: newCtx(9999, "user"), wantStatus: http.StatusUnauthorized},
		{
			// The JWT still carries the "admin" role from before the demotion,
			// but the DB record is now just "user" — the live role must win.
			name:       "demoted admin's stale JWT role is overridden by the live DB role",
			ctx:        newCtx(demotedAdmin.ID, "admin"),
			wantStatus: http.StatusOK, wantRole: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotRole string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotRole = GetUserRole(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(tt.ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			RequireActiveUser(users)(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusOK {
				assert.Equal(t, tt.wantRole, gotRole)
			}
		})
	}
}
