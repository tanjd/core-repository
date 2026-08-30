package handlers

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// LoanRequestHandler holds dependencies for loan-request routes.
type LoanRequestHandler struct {
	copies   repository.CopyRepository
	loanReqs repository.LoanRequestRepository
	admin    repository.AdminRepository
	users    repository.UserRepository
	workflow *services.LoanWorkflow
}

// NewLoanRequestHandler creates a new LoanRequestHandler.
func NewLoanRequestHandler(
	copies repository.CopyRepository,
	loanReqs repository.LoanRequestRepository,
	admin repository.AdminRepository,
	users repository.UserRepository,
	workflow *services.LoanWorkflow,
) *LoanRequestHandler {
	return &LoanRequestHandler{copies: copies, loanReqs: loanReqs, admin: admin, users: users, workflow: workflow}
}

// --- Input / Output types ---

type createLoanRequestInput struct {
	Body struct {
		CopyID             uint   `json:"copy_id" required:"true" minimum:"1" doc:"ID of the copy to borrow"`
		Message            string `json:"message,omitempty" maxLength:"500" doc:"Optional message to the owner"`
		ExpectedReturnDate string `json:"expected_return_date" required:"true" doc:"Expected return date (YYYY-MM-DD)"`
	}
}

type createLoanRequestOutput struct{ Body models.LoanRequest }

type getLoanRequestInput struct {
	ID uint `path:"id" doc:"Loan request ID"`
}

// safeUser redacts contact info when the loan has not yet been accepted.
type safeUser struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email,omitempty"`
	Phone            string `json:"phone,omitempty"`
	TelegramUsername string `json:"telegram_username,omitempty"`
	WhatsAppUsername string `json:"whatsapp_username,omitempty"`
	ContactNote      string `json:"contact_note,omitempty"`
}

type loanRequestCopyResponse struct {
	ID        uint        `json:"id"`
	BookID    uint        `json:"book_id"`
	OwnerID   uint        `json:"owner_id"`
	Condition string      `json:"condition"`
	Notes     string      `json:"notes"`
	Status    string      `json:"status"`
	Book      models.Book `json:"book,omitempty"`
	Owner     safeUser    `json:"owner,omitempty"`
}

type getLoanRequestBody struct {
	ID                          uint                    `json:"id"`
	CopyID                      uint                    `json:"copy_id"`
	BorrowerID                  uint                    `json:"borrower_id"`
	Message                     string                  `json:"message"`
	Status                      string                  `json:"status"`
	RequestedAt                 time.Time               `json:"requested_at"`
	RespondedAt                 *time.Time              `json:"responded_at"`
	LoanedAt                    *time.Time              `json:"loaned_at"`
	ReturnedAt                  *time.Time              `json:"returned_at"`
	ReturnedBy                  *uint                   `json:"returned_by,omitempty"`
	ExpectedReturnDate          time.Time               `json:"expected_return_date"`
	ExpectedReturnDateChangedBy *uint                   `json:"expected_return_date_changed_by,omitempty"`
	ExpectedReturnDateChangedAt *time.Time              `json:"expected_return_date_changed_at,omitempty"`
	Copy                        loanRequestCopyResponse `json:"copy"`
	Borrower                    safeUser                `json:"borrower"`
}

type getLoanRequestOutput struct{ Body getLoanRequestBody }

type listLoanRequestsInput struct {
	CopyID uint `query:"copy_id" minimum:"1" doc:"Copy ID to list requests for (owner only)"`
}

type listLoanRequestsOutput struct{ Body []getLoanRequestBody }

type listMineInput struct {
	Page     int    `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int    `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
	View     string `query:"view" doc:"Filter: current (pending+accepted) or history (returned+rejected+cancelled); omit for all"`
}

type listMineOutput struct {
	Body struct {
		Items      []getLoanRequestBody `json:"items"`
		Total      int64                `json:"total"`
		Page       int                  `json:"page"`
		PageSize   int                  `json:"page_size"`
		TotalPages int                  `json:"total_pages"`
	}
}

type listMineActiveOutput struct {
	Body struct {
		Items []getLoanRequestBody `json:"items"`
	}
}

type updateLoanRequestInput struct {
	ID   uint `path:"id" doc:"Loan request ID"`
	Body struct {
		Status       string `json:"status" required:"true" doc:"New status: accepted, rejected, returned, or cancelled. Submitting \"accepted\" while the loan is currently \"returned\" undoes the return (owner only)."`
		NewCondition string `json:"new_condition,omitempty" doc:"Updated copy condition on return: good, fair, or worn"`
	}
}

type updateLoanRequestOutput struct{ Body models.LoanRequest }

type updateExpectedReturnDateInput struct {
	ID   uint `path:"id" doc:"Loan request ID"`
	Body struct {
		ExpectedReturnDate string `json:"expected_return_date" required:"true" doc:"New agreed return date (YYYY-MM-DD)"`
	}
}

type updateExpectedReturnDateOutput struct{ Body models.LoanRequest }

// --- Route registration ---

// RegisterRoutes registers all loan-request routes on the given huma API.
func (h *LoanRequestHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-my-loan-requests",
		Method:      "GET",
		Path:        "/loan-requests/mine",
		Tags:        []string{"loan-requests"},
		Summary:     "List all loan requests made by the authenticated user (paginated)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMine)

	huma.Register(api, huma.Operation{
		OperationID: "list-my-lending-requests",
		Method:      "GET",
		Path:        "/loan-requests/mine/lending",
		Tags:        []string{"loan-requests"},
		Summary:     "List loan requests against copies owned by the authenticated user (paginated)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMineLending)

	huma.Register(api, huma.Operation{
		OperationID: "list-my-active-loans",
		Method:      "GET",
		Path:        "/loan-requests/mine/active",
		Tags:        []string{"loan-requests"},
		Summary:     "List the authenticated user's currently-held (accepted) loans",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMineActive)

	huma.Register(api, huma.Operation{
		OperationID: "list-loan-requests",
		Method:      "GET",
		Path:        "/loan-requests",
		Tags:        []string{"loan-requests"},
		Summary:     "List loan requests for a copy (owner only)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listLoanRequests)

	huma.Register(api, huma.Operation{
		OperationID:   "create-loan-request",
		Method:        "POST",
		Path:          "/loan-requests",
		Tags:          []string{"loan-requests"},
		Summary:       "Request to borrow a copy",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createLoanRequest)

	huma.Register(api, huma.Operation{
		OperationID: "get-loan-request",
		Method:      "GET",
		Path:        "/loan-requests/{id}",
		Tags:        []string{"loan-requests"},
		Summary:     "Get a loan request (contact info revealed only when accepted)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getLoanRequest)

	huma.Register(api, huma.Operation{
		OperationID: "update-loan-request",
		Method:      "PATCH",
		Path:        "/loan-requests/{id}",
		Tags:        []string{"loan-requests"},
		Summary:     "Update a loan request status (accept, reject, cancel, mark returned, or undo a return)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateLoanRequest)

	huma.Register(api, huma.Operation{
		OperationID: "update-loan-request-return-date",
		Method:      "PATCH",
		Path:        "/loan-requests/{id}/expected-return-date",
		Tags:        []string{"loan-requests"},
		Summary:     "Set or update the agreed return date for an accepted loan",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateExpectedReturnDate)
}

// --- Handlers ---

// revealedUser redacts a user's contact info to just ID/Name unless reveal
// is true, in which case every contact field (email, phone, messaging
// handles, and the free-text contact note) is included.
func revealedUser(u models.User, reveal bool) safeUser {
	s := safeUser{ID: u.ID, Name: u.Name}
	if reveal {
		s.Email = u.Email
		s.Phone = u.Phone
		s.TelegramUsername = u.TelegramUsername
		s.WhatsAppUsername = u.WhatsAppUsername
		s.ContactNote = u.ContactNote
	}
	return s
}

// buildContactPair returns the borrower/owner safeUser pair for a loan
// request, redacting contact info unless the loan is accepted. This is the
// single place that decides contact-reveal for every loan-request response
// (toGetLoanRequestBody, getLoanRequest, listLoanRequests) so the three
// endpoints can't drift out of sync on which fields get redacted.
func buildContactPair(lr models.LoanRequest) (borrower, owner safeUser) {
	reveal := lr.Status == "accepted"
	return revealedUser(lr.Borrower, reveal), revealedUser(lr.Copy.Owner, reveal)
}

// toGetLoanRequestBody maps a LoanRequest to its API response body,
// redacting borrower/owner contact info unless the loan is accepted.
func toGetLoanRequestBody(lr models.LoanRequest) getLoanRequestBody {
	borrowerResp, ownerResp := buildContactPair(lr)
	return getLoanRequestBody{
		ID:                          lr.ID,
		CopyID:                      lr.CopyID,
		BorrowerID:                  lr.BorrowerID,
		Message:                     lr.Message,
		Status:                      lr.Status,
		RequestedAt:                 lr.RequestedAt,
		RespondedAt:                 lr.RespondedAt,
		LoanedAt:                    lr.LoanedAt,
		ReturnedAt:                  lr.ReturnedAt,
		ReturnedBy:                  lr.ReturnedBy,
		ExpectedReturnDate:          lr.ExpectedReturnDate,
		ExpectedReturnDateChangedBy: lr.ExpectedReturnDateChangedBy,
		ExpectedReturnDateChangedAt: lr.ExpectedReturnDateChangedAt,
		Copy: loanRequestCopyResponse{
			ID:        lr.Copy.ID,
			BookID:    lr.Copy.BookID,
			OwnerID:   lr.Copy.OwnerID,
			Condition: lr.Copy.Condition,
			Notes:     lr.Copy.Notes,
			Status:    lr.Copy.Status,
			Book:      lr.Copy.Book,
			Owner:     ownerResp,
		},
		Borrower: borrowerResp,
	}
}

// statusesForView maps the "view" query param to a status filter set for
// listMine. Unknown/empty values return nil (no filter, i.e. every status).
func statusesForView(view string) []string {
	switch view {
	case "current":
		return []string{"pending", "accepted"}
	case "history":
		return []string{"returned", "rejected", "cancelled"}
	default:
		return nil
	}
}

func (h *LoanRequestHandler) listMine(ctx context.Context, input *listMineInput) (*listMineOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.loanReqs.ListByBorrowerIDPaginated(callerID, statusesForView(input.View), page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch loan requests")
	}

	bodies := make([]getLoanRequestBody, len(result.Items))
	for i, lr := range result.Items {
		bodies[i] = toGetLoanRequestBody(lr)
	}

	var out listMineOutput
	out.Body.Items = bodies
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *LoanRequestHandler) listMineLending(ctx context.Context, input *listMineInput) (*listMineOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.loanReqs.ListByOwnerIDPaginated(callerID, statusesForView(input.View), page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch lending history")
	}

	bodies := make([]getLoanRequestBody, len(result.Items))
	for i, lr := range result.Items {
		bodies[i] = toGetLoanRequestBody(lr)
	}

	var out listMineOutput
	out.Body.Items = bodies
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *LoanRequestHandler) listMineActive(ctx context.Context, _ *struct{}) (*listMineActiveOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	items, err := h.loanReqs.ListActiveByBorrowerID(callerID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch active loans")
	}

	bodies := make([]getLoanRequestBody, len(items))
	for i, lr := range items {
		bodies[i] = toGetLoanRequestBody(lr)
	}

	var out listMineActiveOutput
	out.Body.Items = bodies
	return &out, nil
}

func (h *LoanRequestHandler) listLoanRequests(ctx context.Context, input *listLoanRequestsInput) (*listLoanRequestsOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.copies.GetByID(input.CopyID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID != callerID {
		return nil, huma.Error403Forbidden("only the copy owner can list requests")
	}

	requests, err := h.loanReqs.ListByCopyID(input.CopyID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch loan requests")
	}

	bodies := make([]getLoanRequestBody, len(requests))
	for i, lr := range requests {
		bodies[i] = toGetLoanRequestBody(lr)
	}
	return &listLoanRequestsOutput{Body: bodies}, nil
}

func (h *LoanRequestHandler) createLoanRequest(ctx context.Context, input *createLoanRequestInput) (*createLoanRequestOutput, error) {
	borrowerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.getRequestableCopy(borrowerID, input.Body.CopyID)
	if err != nil {
		return nil, err
	}

	if err := h.checkBorrowerEligibility(borrowerID); err != nil {
		return nil, err
	}

	lr, err := buildLoanRequest(borrowerID, input)
	if err != nil {
		return nil, err
	}

	// Atomically create the loan request and mark the copy as requested,
	// preventing a TOCTOU race where two concurrent requests both pass the
	// availability check above and result in two active loan requests.
	if err := h.loanReqs.CreateAndMarkRequested(&lr); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error400BadRequest("copy is no longer available")
		}
		return nil, huma.Error500InternalServerError("could not create loan request")
	}

	// Load associations needed by the workflow.
	loaded, _ := h.loanReqs.GetByIDWithCopyOwnerAndBorrower(lr.ID)
	if loaded != nil {
		lr = *loaded
	}

	h.finalizeLoanRequest(ctx, &lr, bookCopy)

	return &createLoanRequestOutput{Body: lr}, nil
}

// getRequestableCopy fetches a copy and verifies it can be requested by borrowerID.
func (h *LoanRequestHandler) getRequestableCopy(borrowerID, copyID uint) (*models.Copy, error) {
	bookCopy, err := h.copies.GetByID(copyID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID == borrowerID {
		return nil, huma.Error400BadRequest("you cannot request your own copy")
	}
	if bookCopy.Status != "available" {
		return nil, huma.Error400BadRequest("copy is not available")
	}
	return bookCopy, nil
}

// checkBorrowerEligibility enforces the admin-configured borrowing limits and
// verification requirements (max active loans, verified email, phone on
// file, minimum books shared).
func (h *LoanRequestHandler) checkBorrowerEligibility(borrowerID uint) error {
	// Load all settings in a single query for the eligibility checks below.
	allSettings, err := h.admin.GetSettings()
	if err != nil {
		return huma.Error500InternalServerError("could not load settings")
	}
	sm := make(map[string]string, len(allSettings))
	for _, s := range allSettings {
		sm[s.Key] = s.Value
	}

	if err := h.checkMaxActiveLoans(borrowerID, sm); err != nil {
		return err
	}
	return h.checkVerificationRequirements(borrowerID, sm)
}

// checkMaxActiveLoans enforces the max_active_loans setting (0 = unlimited).
func (h *LoanRequestHandler) checkMaxActiveLoans(borrowerID uint, sm map[string]string) error {
	maxStr := sm["max_active_loans"]
	if maxStr == "" || maxStr == "0" {
		return nil
	}
	var maxLoans int64
	if _, scanErr := fmt.Sscanf(maxStr, "%d", &maxLoans); scanErr != nil || maxLoans <= 0 {
		return nil
	}
	activeCount, countErr := h.loanReqs.CountActiveLoansByBorrower(borrowerID)
	if countErr != nil || activeCount < maxLoans {
		return nil
	}
	return huma.Error422UnprocessableEntity(
		fmt.Sprintf("you have reached the maximum of %d active loan(s)", maxLoans),
	)
}

// checkVerificationRequirements enforces require_verified_to_borrow,
// verification_requires_phone, and verification_min_books_shared.
func (h *LoanRequestHandler) checkVerificationRequirements(borrowerID uint, sm map[string]string) error {
	// Load borrower once for all three checks below.
	borrower, borrowerErr := h.users.FindByID(borrowerID)

	if err := checkVerifiedEmailRequirement(sm, borrower, borrowerErr); err != nil {
		return err
	}
	if err := checkPhoneRequirement(sm, borrower, borrowerErr); err != nil {
		return err
	}
	return h.checkMinBooksSharedRequirement(borrowerID, sm)
}

// checkVerifiedEmailRequirement enforces require_verified_to_borrow, if enabled.
func checkVerifiedEmailRequirement(sm map[string]string, borrower *models.User, borrowerErr error) error {
	if sm["require_verified_to_borrow"] != "true" {
		return nil
	}
	if borrowerErr != nil || !borrower.Verified {
		return huma.Error403Forbidden("a verified email is required to borrow books")
	}
	return nil
}

// checkPhoneRequirement enforces verification_requires_phone, if enabled.
func checkPhoneRequirement(sm map[string]string, borrower *models.User, borrowerErr error) error {
	if sm["verification_requires_phone"] != "true" {
		return nil
	}
	if borrowerErr != nil || borrower.Phone == "" {
		return huma.Error403Forbidden("a phone number is required to borrow books")
	}
	return nil
}

// checkMinBooksSharedRequirement enforces verification_min_books_shared (0/unset = disabled).
func (h *LoanRequestHandler) checkMinBooksSharedRequirement(borrowerID uint, sm map[string]string) error {
	minStr := sm["verification_min_books_shared"]
	if minStr == "" || minStr == "0" {
		return nil
	}
	var minBooks int64
	if _, scanErr := fmt.Sscanf(minStr, "%d", &minBooks); scanErr != nil || minBooks <= 0 {
		return nil
	}
	sharedCount, countErr := h.copies.CountByOwnerID(borrowerID)
	if countErr != nil || sharedCount >= minBooks {
		return nil
	}
	return huma.Error403Forbidden(
		fmt.Sprintf("you must share at least %d book(s) before you can borrow", minBooks),
	)
}

// buildLoanRequest constructs a pending LoanRequest from the request body,
// parsing and validating the (always-required) expected return date.
func buildLoanRequest(borrowerID uint, input *createLoanRequestInput) (models.LoanRequest, error) {
	t, err := parseAndValidateReturnDate(input.Body.ExpectedReturnDate)
	if err != nil {
		return models.LoanRequest{}, err
	}

	return models.LoanRequest{
		CopyID:             input.Body.CopyID,
		BorrowerID:         borrowerID,
		Message:            input.Body.Message,
		Status:             "pending",
		RequestedAt:        time.Now(),
		ExpectedReturnDate: t,
	}, nil
}

// parseAndValidateReturnDate parses a YYYY-MM-DD date and rejects anything
// before today (UTC) — a borrower or owner can propose today's date (an
// immediate turnaround), but never backdate one, which would make a loan
// overdue the moment it's set.
func parseAndValidateReturnDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, huma.Error400BadRequest("expected_return_date must be in YYYY-MM-DD format")
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if t.Before(today) {
		return time.Time{}, huma.Error400BadRequest("expected_return_date must not be in the past")
	}
	return t, nil
}

// finalizeLoanRequest runs the post-create workflow: notify the owner, or
// auto-approve immediately if the copy has auto-approve enabled.
func (h *LoanRequestHandler) finalizeLoanRequest(ctx context.Context, lr *models.LoanRequest, bookCopy *models.Copy) {
	// Skip OnRequested when auto-approving to avoid sending a redundant
	// "someone wants to borrow your book" email that is immediately superseded.
	if !bookCopy.AutoApprove {
		if err := h.workflow.OnRequested(ctx, lr); err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("workflow.OnRequested failed")
		}
		return
	}

	now := time.Now()
	lr.Status = "accepted"
	lr.RespondedAt = &now
	if saveErr := h.loanReqs.Save(lr); saveErr != nil {
		zerolog.Ctx(ctx).Error().Err(saveErr).Msg("auto-approve save failed")
		return
	}
	if wErr := h.workflow.OnAccepted(ctx, lr); wErr != nil {
		zerolog.Ctx(ctx).Error().Err(wErr).Msg("workflow.OnAccepted failed for auto-approve")
	}
	if reloaded, relErr := h.loanReqs.GetByIDWithCopyOwnerAndBorrower(lr.ID); relErr == nil {
		*lr = *reloaded
	}
}

func (h *LoanRequestHandler) getLoanRequest(ctx context.Context, input *getLoanRequestInput) (*getLoanRequestOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	lr, err := h.loanReqs.GetByIDWithFullAssociations(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("loan request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch loan request")
	}

	ownerID := lr.Copy.OwnerID
	if lr.BorrowerID != callerID && ownerID != callerID {
		return nil, huma.Error403Forbidden("access denied")
	}

	// callerID is already guaranteed to be the borrower or the owner (the
	// access-denied check above), so reveal is decided by status alone —
	// same rule buildContactPair applies everywhere else.
	borrowerResp, ownerResp := buildContactPair(*lr)

	body := getLoanRequestBody{
		ID:                          lr.ID,
		CopyID:                      lr.CopyID,
		BorrowerID:                  lr.BorrowerID,
		Message:                     lr.Message,
		Status:                      lr.Status,
		RequestedAt:                 lr.RequestedAt,
		RespondedAt:                 lr.RespondedAt,
		LoanedAt:                    lr.LoanedAt,
		ReturnedAt:                  lr.ReturnedAt,
		ReturnedBy:                  lr.ReturnedBy,
		ExpectedReturnDate:          lr.ExpectedReturnDate,
		ExpectedReturnDateChangedBy: lr.ExpectedReturnDateChangedBy,
		ExpectedReturnDateChangedAt: lr.ExpectedReturnDateChangedAt,
		Copy: loanRequestCopyResponse{
			ID:        lr.Copy.ID,
			BookID:    lr.Copy.BookID,
			OwnerID:   lr.Copy.OwnerID,
			Condition: lr.Copy.Condition,
			Notes:     lr.Copy.Notes,
			Status:    lr.Copy.Status,
			Book:      lr.Copy.Book,
			Owner:     ownerResp,
		},
		Borrower: borrowerResp,
	}

	return &getLoanRequestOutput{Body: body}, nil
}

func (h *LoanRequestHandler) updateLoanRequest(ctx context.Context, input *updateLoanRequestInput) (*updateLoanRequestOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	lr, err := h.loanReqs.GetByIDWithCopyAndBorrower(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("loan request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch loan request")
	}

	ownerID := lr.Copy.OwnerID
	now := time.Now()

	var action string
	var transitionErr error
	switch input.Body.Status {
	case "accepted":
		action, transitionErr = h.acceptedTransition(lr, callerID, ownerID, now)
	case "rejected":
		transitionErr = h.acceptOrRejectLoan(lr, callerID, ownerID, "rejected", now)
		action = "rejected"
	case "returned":
		transitionErr = h.returnLoan(ctx, lr, callerID, ownerID, now, input.Body.NewCondition)
		action = "returned"
	case "cancelled":
		transitionErr = h.cancelLoan(lr, callerID)
		action = "cancelled"
	default:
		transitionErr = huma.Error400BadRequest("invalid status transition")
	}
	if transitionErr != nil {
		return nil, transitionErr
	}

	if err := h.loanReqs.Save(lr); err != nil {
		return nil, huma.Error500InternalServerError("could not update loan request")
	}

	h.runLoanWorkflowSideEffect(ctx, lr, action)

	return &updateLoanRequestOutput{Body: *lr}, nil
}

// acceptOrRejectLoan validates and applies an accept/reject transition; only
// the copy owner may act, and only on a pending request.
func (h *LoanRequestHandler) acceptOrRejectLoan(lr *models.LoanRequest, callerID, ownerID uint, status string, now time.Time) error {
	if callerID != ownerID {
		return huma.Error403Forbidden("only the copy owner can accept or reject")
	}
	if lr.Status != "pending" {
		return huma.Error400BadRequest("can only accept/reject pending requests")
	}
	lr.Status = status
	lr.RespondedAt = &now
	return nil
}

// acceptedTransition dispatches a "status: accepted" PATCH to either a fresh
// pending→accepted acceptance, or — if the loan is currently "returned" — an
// owner-only undo of that return. It returns the workflow action name for
// runLoanWorkflowSideEffect to fire on, since both paths leave lr.Status as
// "accepted" and can no longer be told apart afterwards.
func (h *LoanRequestHandler) acceptedTransition(lr *models.LoanRequest, callerID, ownerID uint, now time.Time) (string, error) {
	if lr.Status == "returned" {
		if err := h.undoReturn(lr, callerID, ownerID); err != nil {
			return "", err
		}
		return "return_undone", nil
	}
	if err := h.acceptOrRejectLoan(lr, callerID, ownerID, "accepted", now); err != nil {
		return "", err
	}
	return "accepted", nil
}

// undoReturn reverses a "returned" loan back to "accepted" because the return
// wasn't genuine. Only the copy owner may do this, and only while the copy is
// still exactly in the state OnReturned left it ("available") — i.e. nobody
// has requested, accepted, or otherwise touched the copy since. That guard
// prevents an undo from silently clobbering a different borrower's now-active
// loan on the same copy.
func (h *LoanRequestHandler) undoReturn(lr *models.LoanRequest, callerID, ownerID uint) error {
	if callerID != ownerID {
		return huma.Error403Forbidden("only the copy owner can undo a return")
	}
	if lr.Copy.Status != "available" {
		return huma.Error409Conflict(
			"this copy is no longer available — it may have been re-requested or loaned out since this was marked returned",
		)
	}
	lr.Status = "accepted"
	lr.ReturnedAt = nil
	lr.ReturnedBy = nil
	return nil
}

// returnLoan validates and applies a return transition, optionally updating
// the copy's condition; either the borrower or the copy owner may act, and
// only on an accepted loan.
func (h *LoanRequestHandler) returnLoan(
	ctx context.Context, lr *models.LoanRequest, callerID, ownerID uint, now time.Time, newCondition string,
) error {
	if callerID != ownerID && callerID != lr.BorrowerID {
		return huma.Error403Forbidden("only the borrower or the copy owner can mark this as returned")
	}
	if lr.Status != "accepted" {
		return huma.Error400BadRequest("can only mark accepted loans as returned")
	}
	lr.Status = "returned"
	lr.ReturnedAt = &now
	lr.ReturnedBy = &callerID

	if newCondition == "" {
		return nil
	}
	allowed := map[string]bool{"good": true, "fair": true, "worn": true}
	if !allowed[newCondition] {
		return huma.Error400BadRequest("new_condition must be good, fair, or worn")
	}
	lr.Copy.Condition = newCondition
	if saveErr := h.copies.Save(&lr.Copy); saveErr != nil {
		zerolog.Ctx(ctx).Error().Err(saveErr).Msg("failed to update copy condition on return")
	}
	return nil
}

// cancelLoan validates and applies a cancel transition; only the borrower may
// act, and only on a pending request.
func (h *LoanRequestHandler) cancelLoan(lr *models.LoanRequest, callerID uint) error {
	if callerID != lr.BorrowerID {
		return huma.Error403Forbidden("only the borrower can cancel")
	}
	if lr.Status != "pending" {
		return huma.Error400BadRequest("can only cancel pending requests")
	}
	lr.Status = "cancelled"
	return nil
}

// updateExpectedReturnDate lets either party (borrower or owner) set or
// change the agreed return date on an accepted loan — unlike at request
// creation, this can be filled in later once a date is actually agreed.
func (h *LoanRequestHandler) updateExpectedReturnDate(
	ctx context.Context, input *updateExpectedReturnDateInput,
) (*updateExpectedReturnDateOutput, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	lr, err := h.loanReqs.GetByIDWithCopyAndBorrower(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("loan request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch loan request")
	}

	if callerID != lr.Copy.OwnerID && callerID != lr.BorrowerID {
		return nil, huma.Error403Forbidden("only the borrower or the copy owner can update the return date")
	}
	if lr.Status != "accepted" {
		return nil, huma.Error400BadRequest("return date can only be changed while the loan is accepted")
	}

	t, err := parseAndValidateReturnDate(input.Body.ExpectedReturnDate)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	lr.ExpectedReturnDate = t
	lr.ExpectedReturnDateChangedBy = &callerID
	lr.ExpectedReturnDateChangedAt = &now

	if err := h.loanReqs.Save(lr); err != nil {
		return nil, huma.Error500InternalServerError("could not update return date")
	}

	if err := h.workflow.OnExpectedReturnDateChanged(ctx, lr, callerID); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("workflow.OnExpectedReturnDateChanged failed")
	}

	// Reload with Copy.Book/Copy.Owner — GetByIDWithCopyAndBorrower above only
	// preloads Copy itself, and returning that half-hydrated struct would wipe
	// the book title/owner name out of any frontend state it overwrites.
	if reloaded, relErr := h.loanReqs.GetByIDWithFullAssociations(lr.ID); relErr == nil {
		lr = reloaded
	}

	return &updateExpectedReturnDateOutput{Body: *lr}, nil
}

// runLoanWorkflowSideEffect fires the workflow callback matching the action
// just performed (non-fatal — failures are logged, not returned).
func (h *LoanRequestHandler) runLoanWorkflowSideEffect(ctx context.Context, lr *models.LoanRequest, action string) {
	var workflowErr error
	switch action {
	case "accepted":
		workflowErr = h.workflow.OnAccepted(ctx, lr)
	case "rejected":
		workflowErr = h.workflow.OnRejected(ctx, lr)
	case "cancelled":
		workflowErr = h.workflow.OnCancelled(ctx, lr)
	case "returned":
		workflowErr = h.workflow.OnReturned(ctx, lr)
	case "return_undone":
		workflowErr = h.workflow.OnReturnUndone(ctx, lr)
	}
	if workflowErr != nil {
		zerolog.Ctx(ctx).Error().Err(workflowErr).Str("action", action).Msg("workflow side-effect failed")
	}
}
