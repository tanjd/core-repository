package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

type exportSettingsOutput struct {
	Body struct {
		Content string `json:"content"`
	}
}

// AdminHandler holds dependencies for admin routes.
type AdminHandler struct {
	admin              repository.AdminRepository
	copies             repository.CopyRepository
	loans              repository.LoanRequestRepository
	googleBooksKeyPool *services.GoogleBooksKeyPool
	registration       *services.RegistrationWorkflow
	// recommendations is optional (nil-safe), same reasoning as
	// BookHandler.recommendations — deleteUser clears the target's
	// recommendation rows before the user itself is deleted, so an
	// ex-member's thumbs-ups fall out of every book's count and facepile.
	// See docs/book-recommendations-spec.md's "Live-community signal".
	recommendations repository.RecommendationRepository
	// inviteCodes is optional (nil-safe), same reasoning as recommendations
	// above — deleteUser and a suspend transition both revoke the target's
	// invite code, so a removed or suspended member can't keep bringing in
	// new signups via an outstanding link. See docs/invite-code-spec.md.
	inviteCodes repository.InviteCodeRepository
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(admin repository.AdminRepository, copies repository.CopyRepository, loans repository.LoanRequestRepository, googleBooksKeyPool *services.GoogleBooksKeyPool, registration *services.RegistrationWorkflow, recommendations repository.RecommendationRepository, inviteCodes repository.InviteCodeRepository) *AdminHandler {
	return &AdminHandler{admin: admin, copies: copies, loans: loans, googleBooksKeyPool: googleBooksKeyPool, registration: registration, recommendations: recommendations, inviteCodes: inviteCodes}
}

// --- Input / Output types ---

type adminUsersInput struct {
	Page     int    `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int    `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 50)"`
	Search   string `query:"search" doc:"Filter by name or email (case-insensitive substring match)"`
	Role     string `query:"role" enum:"user,admin" doc:"Filter by role"`
	Status   string `query:"status" enum:"verified,unverified,pending_approval,suspended" doc:"Filter by status"`
}

type adminUsersOutput struct {
	Body struct {
		Items      []models.User `json:"items"`
		Total      int64         `json:"total"`
		Page       int           `json:"page"`
		PageSize   int           `json:"page_size"`
		TotalPages int           `json:"total_pages"`
	}
}

type adminUserIDInput struct {
	ID uint `path:"id" doc:"User ID"`
}

type updateAdminUserInput struct {
	ID   uint `path:"id" doc:"User ID"`
	Body struct {
		Role            *string `json:"role,omitempty" doc:"Role: user or admin"`
		Suspended       *bool   `json:"suspended,omitempty" doc:"Whether the user is suspended (cannot log in)"`
		PendingApproval *bool   `json:"pending_approval,omitempty" doc:"Whether the user is still awaiting admin approval (cannot log in until cleared)"`
	}
}

type adminUserOutput struct {
	Body models.User
}

type adminSettingsOutput struct {
	Body []models.AppSetting
}

type updateSettingsInput struct {
	Body []struct {
		Key   string `json:"key" required:"true" doc:"Setting key"`
		Value string `json:"value" required:"true" doc:"Setting value"`
	}
}

// MetadataProviderStatus reports reachability of a single metadata source.
type MetadataProviderStatus struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type metadataStatusOutput struct {
	Body []MetadataProviderStatus
}

type adminDashboardOutput struct {
	Body repository.DashboardStats
}

// --- Route registration ---

// RegisterRoutes registers all admin routes on the given huma API.
func (h *AdminHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "admin-dashboard-stats",
		Method:      "GET",
		Path:        "/admin/dashboard",
		Tags:        []string{"admin"},
		Summary:     "Get admin dashboard statistics",
		Security:    security,
	}, h.getDashboardStats)

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-users",
		Method:      "GET",
		Path:        "/admin/users",
		Tags:        []string{"admin"},
		Summary:     "List all users",
		Security:    security,
	}, h.listUsers)

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-user",
		Method:      "PATCH",
		Path:        "/admin/users/{id}",
		Tags:        []string{"admin"},
		Summary:     "Update a user's role or suspended status",
		Security:    security,
	}, h.updateUser)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-delete-user",
		Method:        "DELETE",
		Path:          "/admin/users/{id}",
		Tags:          []string{"admin"},
		Summary:       "Delete a user",
		Security:      security,
		DefaultStatus: 204,
	}, h.deleteUser)

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-settings",
		Method:      "GET",
		Path:        "/admin/settings",
		Tags:        []string{"admin"},
		Summary:     "Get all app settings",
		Security:    security,
	}, h.getSettings)

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-settings",
		Method:      "PATCH",
		Path:        "/admin/settings",
		Tags:        []string{"admin"},
		Summary:     "Upsert app settings",
		Security:    security,
	}, h.updateSettings)

	huma.Register(api, huma.Operation{
		OperationID: "admin-export-settings",
		Method:      "GET",
		Path:        "/admin/settings/export",
		Tags:        []string{"admin"},
		Summary:     "Export current settings as a bookshelf.yaml file",
		Security:    security,
	}, h.exportSettings)

	huma.Register(api, huma.Operation{
		OperationID: "admin-metadata-status",
		Method:      "GET",
		Path:        "/admin/metadata/status",
		Tags:        []string{"admin"},
		Summary:     "Check reachability of metadata providers",
		Security:    security,
	}, h.getMetadataStatus)
}

// --- Handlers ---

func (h *AdminHandler) getDashboardStats(ctx context.Context, _ *struct{}) (*adminDashboardOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	stats, err := h.admin.GetDashboardStats()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch dashboard stats")
	}

	return &adminDashboardOutput{Body: *stats}, nil
}

func (h *AdminHandler) listUsers(ctx context.Context, input *adminUsersInput) (*adminUsersOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	result, err := h.admin.ListUsersPaginated(page, pageSize, repository.UserListFilter{
		Search: input.Search,
		Role:   input.Role,
		Status: input.Status,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list users")
	}
	var out adminUsersOutput
	out.Body.Items = result.Items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *AdminHandler) updateUser(ctx context.Context, input *updateAdminUserInput) (*adminUserOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	callerID, _ := middleware.GetRequiredUserID(ctx)

	user, err := h.admin.FindUserByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if err := h.applyUserUpdates(ctx, user, callerID, input); err != nil {
		return nil, err
	}

	return &adminUserOutput{Body: *user}, nil
}

// applyUserUpdates applies the role/suspended/pendingApproval fields of an
// admin user-update request, persists the result, and fires the
// OnApproved notification when the update clears PendingApproval.
func (h *AdminHandler) applyUserUpdates(ctx context.Context, user *models.User, callerID uint, input *updateAdminUserInput) error {
	body := input.Body
	if body.Role != nil {
		if err := h.applyRoleUpdate(user, callerID, *body.Role); err != nil {
			return err
		}
	}
	if body.Suspended != nil {
		if err := h.applySuspendedUpdate(user, callerID, *body.Suspended); err != nil {
			return err
		}
	}
	wasPending := user.PendingApproval
	if body.PendingApproval != nil {
		if err := h.applyPendingApprovalUpdate(user, callerID, *body.PendingApproval); err != nil {
			return err
		}
	}

	if err := h.admin.SaveUser(user); err != nil {
		return huma.Error500InternalServerError("could not update user")
	}

	if wasPending && !user.PendingApproval {
		h.registration.OnApproved(ctx, user)
	}
	return nil
}

// applyRoleUpdate validates and applies a role change, enforcing that an admin
// cannot demote themselves or demote the last remaining admin.
func (h *AdminHandler) applyRoleUpdate(user *models.User, callerID uint, newRole string) error {
	if newRole != "admin" && newRole != "user" {
		return huma.Error400BadRequest("role must be 'admin' or 'user'")
	}
	// Prevent admins from demoting themselves.
	if user.ID == callerID && newRole != "admin" {
		return huma.Error400BadRequest("cannot demote yourself")
	}
	// Prevent demoting the last admin.
	if user.Role == "admin" && newRole != "admin" {
		count, err := h.admin.CountByRole("admin")
		if err != nil {
			return huma.Error500InternalServerError("could not check admin count")
		}
		if count <= 1 {
			return huma.Error400BadRequest("cannot demote the last admin")
		}
	}
	user.Role = newRole
	return nil
}

// applySuspendedUpdate validates and applies a suspension change, preventing
// an admin from suspending themselves.
func (h *AdminHandler) applySuspendedUpdate(user *models.User, callerID uint, suspended bool) error {
	if user.ID == callerID && suspended {
		return huma.Error400BadRequest("cannot suspend yourself")
	}
	user.Suspended = suspended
	// A suspended member can't keep bringing in new signups via an
	// outstanding invite link — revoke it alongside the state change. A
	// no-op if they have none.
	if suspended && h.inviteCodes != nil {
		if err := h.inviteCodes.DeleteByInviter(user.ID); err != nil {
			return huma.Error500InternalServerError("could not revoke user's invite code")
		}
	}
	return nil
}

// applyPendingApprovalUpdate validates and applies an approval-status change,
// preventing an admin from placing themselves back into pending approval.
func (h *AdminHandler) applyPendingApprovalUpdate(user *models.User, callerID uint, pending bool) error {
	if user.ID == callerID && pending {
		return huma.Error400BadRequest("cannot set your own account back to pending approval")
	}
	user.PendingApproval = pending
	return nil
}

func (h *AdminHandler) deleteUser(ctx context.Context, input *adminUserIDInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	callerID, _ := middleware.GetRequiredUserID(ctx)
	if uint(input.ID) == callerID {
		return nil, huma.Error400BadRequest("cannot delete yourself")
	}

	// Prevent deleting the last admin
	target, err := h.admin.FindUserByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}
	if err := h.guardLastAdmin(target); err != nil {
		return nil, err
	}
	if err := h.guardDeletableUser(input.ID); err != nil {
		return nil, err
	}

	if h.recommendations != nil {
		if err := h.recommendations.DeleteByRecommenderID(input.ID); err != nil {
			return nil, huma.Error500InternalServerError("could not clear user's recommendations")
		}
	}
	if h.inviteCodes != nil {
		if err := h.inviteCodes.DeleteByInviter(input.ID); err != nil {
			return nil, huma.Error500InternalServerError("could not revoke user's invite code")
		}
	}

	if err := h.admin.DeleteUser(input.ID); err != nil {
		return nil, huma.Error500InternalServerError("could not delete user")
	}

	return nil, nil
}

// guardLastAdmin rejects deleting target if they're the community's sole
// remaining admin.
func (h *AdminHandler) guardLastAdmin(target *models.User) error {
	if target.Role != "admin" {
		return nil
	}
	count, err := h.admin.CountByRole("admin")
	if err != nil {
		return huma.Error500InternalServerError("could not check admin count")
	}
	if count <= 1 {
		return huma.Error400BadRequest("cannot delete the last admin")
	}
	return nil
}

// guardDeletableUser rejects deleting userID while they still own copies or
// hold active loan requests — neither has an ON DELETE behavior enforced at
// the DB or ORM level, so deleting them first would orphan those rows.
func (h *AdminHandler) guardDeletableUser(userID uint) error {
	copyCount, err := h.copies.CountByOwnerID(userID)
	if err != nil {
		return huma.Error500InternalServerError("could not check owned copies")
	}
	if copyCount > 0 {
		return huma.Error409Conflict("cannot delete a user who still owns copies — transfer or remove them first")
	}
	activeLoanCount, err := h.loans.CountActiveLoansByBorrower(userID)
	if err != nil {
		return huma.Error500InternalServerError("could not check active loan requests")
	}
	if activeLoanCount > 0 {
		return huma.Error409Conflict("cannot delete a user with active loan requests — resolve them first")
	}
	return nil
}

func (h *AdminHandler) getSettings(ctx context.Context, _ *struct{}) (*adminSettingsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	settings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch settings")
	}

	return &adminSettingsOutput{Body: settings}, nil
}

func (h *AdminHandler) updateSettings(ctx context.Context, input *updateSettingsInput) (*adminSettingsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	for _, kv := range input.Body {
		if err := h.admin.UpsertSetting(kv.Key, kv.Value); err != nil {
			return nil, huma.Error500InternalServerError("could not save setting: " + kv.Key)
		}
	}

	settings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch settings")
	}

	return &adminSettingsOutput{Body: settings}, nil
}

func (h *AdminHandler) exportSettings(ctx context.Context, _ *struct{}) (*exportSettingsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	settings, err := h.admin.GetSettings()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch settings")
	}

	data, err := settingsToYAML(settings)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not serialise settings")
	}

	var out exportSettingsOutput
	out.Body.Content = string(data)
	return &out, nil
}

func (h *AdminHandler) getMetadataStatus(ctx context.Context, _ *struct{}) (*metadataStatusOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	type probe struct {
		name    string
		enabled bool
		// url is the actual request URL; it may contain secrets and must never be
		// surfaced in responses or logs.
		url string
	}

	googleBooksKey := h.googleBooksKeyPool.Key()
	probes := []probe{
		{
			name:    "openlibrary",
			enabled: true,
			url:     "https://openlibrary.org/search.json?q=test&limit=1",
		},
		{
			name:    "google_books",
			enabled: googleBooksKey != "",
			url:     "https://www.googleapis.com/books/v1/volumes?q=test&maxResults=1&key=" + googleBooksKey,
		},
		{
			name:    "bookbrainz",
			enabled: true,
			url:     "https://api.bookbrainz.org/1/search?q=test&type=edition&size=1",
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	statuses := make([]MetadataProviderStatus, len(probes))

	var wg sync.WaitGroup
	for i, p := range probes {
		statuses[i] = MetadataProviderStatus{Name: p.name, Enabled: p.enabled}
		if !p.enabled {
			continue
		}
		wg.Add(1)
		go func(idx int, p probe) {
			defer wg.Done()
			s := &statuses[idx]
			start := time.Now()
			resp, err := client.Get(p.url) //nolint:noctx,gosec
			s.LatencyMs = time.Since(start).Milliseconds()
			if err != nil {
				// Do not include the URL in the error — it may contain an API key.
				s.Error = "connection error"
				zerolog.Ctx(ctx).Warn().Err(err).Str("provider", p.name).Msg("metadata probe failed")
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode < 400 {
					s.Reachable = true
				} else {
					zerolog.Ctx(ctx).Warn().Str("provider", p.name).Int("status", resp.StatusCode).Msg("metadata probe failed")
					s.Error = probeFailureMessage(p.name, resp.StatusCode)
				}
			}
		}(i, p)
	}
	wg.Wait()

	return &metadataStatusOutput{Body: statuses}, nil
}

// probeFailureMessage returns the user-facing error message for a non-2xx metadata
// probe response. Google Books gets its richer status classification (rate limit vs.
// rejected key vs. transient 5xx, see googleBooksStatusError); other providers fall
// back to a generic "HTTP <StatusText>".
func probeFailureMessage(providerName string, status int) string {
	if providerName == "google_books" {
		return googleBooksStatusError(status).Error()
	}
	return "HTTP " + http.StatusText(status)
}

// adminError maps middleware sentinel errors to appropriate huma errors.
func adminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
