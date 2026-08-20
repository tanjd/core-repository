package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// CopyHandler holds dependencies for copy routes.
type CopyHandler struct {
	copies    repository.CopyRepository
	users     repository.UserRepository
	notifs    repository.NotificationRepository
	waitlists repository.WaitlistRepository
	admin     repository.AdminRepository
	books     repository.BookRepository
	wishlists repository.WishlistRequestRepository
	coversDir string
}

// NewCopyHandler creates a new CopyHandler.
func NewCopyHandler(
	copies repository.CopyRepository,
	users repository.UserRepository,
	notifs repository.NotificationRepository,
	waitlists repository.WaitlistRepository,
	admin repository.AdminRepository,
	books repository.BookRepository,
	wishlists repository.WishlistRequestRepository,
	coversDir string,
) *CopyHandler {
	return &CopyHandler{
		copies: copies, users: users, notifs: notifs, waitlists: waitlists, admin: admin,
		books: books, wishlists: wishlists, coversDir: coversDir,
	}
}

// --- Input / Output types ---

type createCopyInput struct {
	Body struct {
		BookID             uint   `json:"book_id" required:"true" minimum:"1" doc:"ID of the book"`
		Condition          string `json:"condition,omitempty" doc:"Physical condition: good, fair, or worn"`
		Notes              string `json:"notes,omitempty" doc:"Optional notes visible to borrowers"`
		AutoApprove        *bool  `json:"auto_approve,omitempty" doc:"Automatically accept the first request"`
		ReturnDateRequired *bool  `json:"return_date_required,omitempty" doc:"Require borrower to provide an expected return date"`
		HideOwner          *bool  `json:"hide_owner,omitempty" doc:"Hide your identity from borrowers (shown as anonymous)"`
	}
}

type createCopyOutput struct{ Body models.Copy }

type updateCopyInput struct {
	ID   uint `path:"id" doc:"Copy ID"`
	Body struct {
		Condition          *string `json:"condition,omitempty" doc:"Physical condition: good, fair, or worn"`
		Notes              *string `json:"notes,omitempty" doc:"Notes visible to borrowers"`
		Status             *string `json:"status,omitempty" doc:"Status: available or unavailable"`
		AutoApprove        *bool   `json:"auto_approve,omitempty" doc:"Automatically accept the first request"`
		ReturnDateRequired *bool   `json:"return_date_required,omitempty" doc:"Require borrower to provide an expected return date"`
		HideOwner          *bool   `json:"hide_owner,omitempty" doc:"Hide your identity from borrowers (shown as anonymous)"`
	}
}

type updateCopyOutput struct{ Body models.Copy }

type deleteCopyInput struct {
	ID uint `path:"id" doc:"Copy ID"`
}

type listMyCopiesOutput struct{ Body []models.Copy }

type listMyOwnedBookIDsOutput struct {
	Body struct {
		BookIDs []uint `json:"book_ids"`
	}
}

type transferCopyInput struct {
	ID   uint `path:"id" doc:"Copy ID"`
	Body struct {
		Email string `json:"email" required:"true" doc:"Email of the user to transfer the copy to"`
	}
}

type transferCopyOutput struct{ Body models.Copy }

// --- Route registration ---

// RegisterRoutes registers all copy routes on the given huma API.
func (h *CopyHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-my-copies",
		Method:      "GET",
		Path:        "/copies/mine",
		Tags:        []string{"copies"},
		Summary:     "List all copies owned by the authenticated user (with book info)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMyCopies)

	huma.Register(api, huma.Operation{
		OperationID: "list-my-owned-book-ids",
		Method:      "GET",
		Path:        "/copies/mine/book-ids",
		Tags:        []string{"copies"},
		Summary:     "List the distinct book IDs the authenticated user owns at least one copy of",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMyOwnedBookIDs)

	huma.Register(api, huma.Operation{
		OperationID: "export-my-copies",
		Method:      "GET",
		Path:        "/copies/mine/export",
		Tags:        []string{"copies"},
		Summary:     "Export the authenticated user's owned copies as JSON, YAML, or CSV",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.exportMyCopies)

	huma.Register(api, huma.Operation{
		OperationID:   "create-copy",
		Method:        "POST",
		Path:          "/copies",
		Tags:          []string{"copies"},
		Summary:       "Add a physical copy of a book",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createCopy)

	huma.Register(api, huma.Operation{
		OperationID: "update-copy",
		Method:      "PATCH",
		Path:        "/copies/{id}",
		Tags:        []string{"copies"},
		Summary:     "Update condition, notes, or status of a copy",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateCopy)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-copy",
		Method:        "DELETE",
		Path:          "/copies/{id}",
		Tags:          []string{"copies"},
		Summary:       "Delete a copy (only if not currently loaned or requested)",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.deleteCopy)

	huma.Register(api, huma.Operation{
		OperationID: "transfer-copy",
		Method:      "POST",
		Path:        "/copies/{id}/transfer",
		Tags:        []string{"copies"},
		Summary:     "Transfer ownership of a copy to another user",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.transferCopy)
}

// --- Handlers ---

func (h *CopyHandler) listMyCopies(ctx context.Context, _ *struct{}) (*listMyCopiesOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	copies, err := h.copies.ListByOwnerID(ownerID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch copies")
	}
	return &listMyCopiesOutput{Body: copies}, nil
}

func (h *CopyHandler) listMyOwnedBookIDs(ctx context.Context, _ *struct{}) (*listMyOwnedBookIDsOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	bookIDs, err := h.copies.ListOwnedBookIDs(ownerID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch owned book ids")
	}
	out := &listMyOwnedBookIDsOutput{}
	out.Body.BookIDs = bookIDs
	return out, nil
}

func (h *CopyHandler) createCopy(ctx context.Context, input *createCopyInput) (*createCopyOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	// Enforce max_copies_per_user (0 = unlimited).
	if maxStr, err := h.admin.GetSetting("max_copies_per_user"); err == nil && maxStr != "" && maxStr != "0" {
		var maxCopies int64
		if _, scanErr := fmt.Sscanf(maxStr, "%d", &maxCopies); scanErr == nil && maxCopies > 0 {
			count, countErr := h.copies.CountByOwnerID(ownerID)
			if countErr == nil && count >= maxCopies {
				return nil, huma.Error422UnprocessableEntity(
					fmt.Sprintf("you have reached the maximum of %d shared copy/copies", maxCopies),
				)
			}
		}
	}

	bookCopy := models.Copy{
		BookID:    input.Body.BookID,
		OwnerID:   ownerID,
		Condition: input.Body.Condition,
		Notes:     input.Body.Notes,
		Status:    "available",
	}
	if input.Body.AutoApprove != nil {
		bookCopy.AutoApprove = *input.Body.AutoApprove
	}
	if input.Body.ReturnDateRequired != nil {
		bookCopy.ReturnDateRequired = *input.Body.ReturnDateRequired
	}
	if input.Body.HideOwner != nil {
		bookCopy.HideOwner = *input.Body.HideOwner
	}
	if err := h.copies.Create(&bookCopy); err != nil {
		return nil, huma.Error500InternalServerError("could not create copy")
	}

	// Reload with associations.
	loaded, err := h.copies.GetByIDWithAssociations(bookCopy.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not reload copy")
	}
	return &createCopyOutput{Body: *loaded}, nil
}

func (h *CopyHandler) updateCopy(ctx context.Context, input *updateCopyInput) (*updateCopyOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.getOwnedCopy(ownerID, input.ID)
	if err != nil {
		return nil, err
	}

	if input.Body.Condition != nil {
		bookCopy.Condition = *input.Body.Condition
	}
	if input.Body.Notes != nil {
		bookCopy.Notes = *input.Body.Notes
	}
	if input.Body.Status != nil {
		if err := applyCopyStatusUpdate(bookCopy, *input.Body.Status); err != nil {
			return nil, err
		}
	}
	if input.Body.AutoApprove != nil {
		bookCopy.AutoApprove = *input.Body.AutoApprove
	}
	if input.Body.ReturnDateRequired != nil {
		bookCopy.ReturnDateRequired = *input.Body.ReturnDateRequired
	}
	if input.Body.HideOwner != nil {
		bookCopy.HideOwner = *input.Body.HideOwner
	}

	if err := h.copies.Save(bookCopy); err != nil {
		return nil, huma.Error500InternalServerError("could not update copy")
	}

	// Reload with associations.
	loaded, err := h.copies.GetByIDWithAssociations(bookCopy.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not reload copy")
	}
	return &updateCopyOutput{Body: *loaded}, nil
}

// applyCopyStatusUpdate validates and applies a status change, rejecting
// unknown statuses and preventing a status change on a copy that's currently
// loaned or requested (which would bypass the loan workflow).
func applyCopyStatusUpdate(bookCopy *models.Copy, newStatus string) error {
	allowed := map[string]bool{"available": true, "unavailable": true}
	if !allowed[newStatus] {
		return huma.Error400BadRequest("status must be 'available' or 'unavailable'")
	}
	if bookCopy.Status == "loaned" || bookCopy.Status == "requested" {
		return huma.Error400BadRequest("cannot change status of a copy that is currently loaned or requested")
	}
	bookCopy.Status = newStatus
	return nil
}

// getOwnedCopy fetches a copy by ID and verifies ownerID owns it. Shared by
// updateCopy, deleteCopy, and transferCopy.
func (h *CopyHandler) getOwnedCopy(ownerID, id uint) (*models.Copy, error) {
	bookCopy, err := h.copies.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID != ownerID {
		return nil, huma.Error403Forbidden("you do not own this copy")
	}
	return bookCopy, nil
}

func (h *CopyHandler) deleteCopy(ctx context.Context, input *deleteCopyInput) (*struct{}, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.getOwnedCopy(ownerID, input.ID)
	if err != nil {
		return nil, err
	}
	if bookCopy.Status == "loaned" || bookCopy.Status == "requested" {
		return nil, huma.Error400BadRequest("cannot delete a copy that is loaned or requested")
	}

	if err := h.copies.Delete(bookCopy); err != nil {
		return nil, huma.Error500InternalServerError("could not delete copy")
	}

	remaining, err := h.books.CountCopies(bookCopy.BookID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("book_id", bookCopy.BookID).Msg("could not count remaining copies after delete")
	} else if remaining == 0 {
		h.maybeDeleteOrphanedBook(ctx, bookCopy.BookID)
	}

	return nil, nil
}

// maybeDeleteOrphanedBook hard-deletes bookID's Book if it's keyless
// (carries no OLKey/GoogleBooksID/ISBN) — such a book can never be matched
// again by BookHandler.findExistingBook's dedup, so once its last Copy is
// gone it's permanent, invisible clutter. Books with an external key are
// left alone: a future re-share can still find and reuse them. Called after
// the copy is already deleted — every failure here is logged and swallowed,
// never surfaced as a copy-deletion error.
func (h *CopyHandler) maybeDeleteOrphanedBook(ctx context.Context, bookID uint) {
	log := zerolog.Ctx(ctx).With().Uint("book_id", bookID).Logger()

	book, err := h.books.GetByIDWithCopies(bookID)
	if err != nil {
		log.Warn().Err(err).Msg("could not fetch book for orphan cleanup")
		return
	}
	if book.OLKey != "" || book.GoogleBooksID != "" || book.ISBN != "" {
		return
	}

	if h.wishlists != nil {
		if err := h.wishlists.ClearFulfilledBookID(bookID); err != nil {
			log.Warn().Err(err).Msg("could not clear wishlist fulfilled_book_id before orphan delete")
		}
	}

	h.deleteCachedCover(ctx, book.CoverURL)

	if err := h.books.Delete(book); err != nil {
		log.Warn().Err(err).Msg("could not delete orphaned keyless book")
		return
	}
	log.Info().Msg("deleted orphaned keyless book")
}

// deleteCachedCover best-effort removes coverURL's locally-cached file, if
// it points inside h.coversDir (i.e. was downloaded via downloadCover —
// see internal/handlers/covers.go). A bare external URL, or any removal
// failure, is silently ignored.
func (h *CopyHandler) deleteCachedCover(ctx context.Context, coverURL string) {
	const localPrefix = "/api/covers/"
	if h.coversDir == "" || !strings.HasPrefix(coverURL, localPrefix) {
		return
	}
	filename := strings.TrimPrefix(coverURL, localPrefix)
	if err := os.Remove(filepath.Join(h.coversDir, filename)); err != nil && !os.IsNotExist(err) {
		zerolog.Ctx(ctx).Warn().Err(err).Str("filename", filename).Msg("could not delete cached cover for orphaned book")
	}
}

func (h *CopyHandler) transferCopy(ctx context.Context, input *transferCopyInput) (*transferCopyOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.getOwnedCopy(ownerID, input.ID)
	if err != nil {
		return nil, err
	}
	if bookCopy.Status == "loaned" || bookCopy.Status == "requested" {
		return nil, huma.Error400BadRequest("cannot transfer a copy that is currently loaned or requested")
	}

	target, err := h.users.FindByEmail(input.Body.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("no user found with that email address")
		}
		return nil, huma.Error500InternalServerError("could not find user")
	}
	if target.ID == ownerID {
		return nil, huma.Error400BadRequest("you already own this copy")
	}

	bookCopy.OwnerID = target.ID
	if err := h.copies.Save(bookCopy); err != nil {
		return nil, huma.Error500InternalServerError("could not transfer copy")
	}

	h.notifyTransfer(ctx, ownerID, target.ID)

	// Clear waitlist since ownership changed.
	if h.waitlists != nil {
		h.waitlists.DeleteByCopyID(bookCopy.ID) //nolint:errcheck,gosec
	}

	loaded, err := h.copies.GetByIDWithAssociations(bookCopy.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not reload copy")
	}
	return &transferCopyOutput{Body: *loaded}, nil
}

// notifyTransfer sends (non-fatal) notifications to both parties of a copy transfer.
func (h *CopyHandler) notifyTransfer(ctx context.Context, prevOwnerID, newOwnerID uint) {
	outN := models.Notification{RecipientID: prevOwnerID, Type: "copy_transferred_out"}
	inN := models.Notification{RecipientID: newOwnerID, Type: "copy_transferred_in"}
	if notifErr := h.notifs.Create(&outN); notifErr != nil {
		zerolog.Ctx(ctx).Warn().Err(notifErr).Msg("transfer out notification failed")
	}
	if notifErr := h.notifs.Create(&inN); notifErr != nil {
		zerolog.Ctx(ctx).Warn().Err(notifErr).Msg("transfer in notification failed")
	}
}
