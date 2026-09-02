package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

type loanTestDeps struct {
	handler  *LoanRequestHandler
	copies   *repotest.CopyRepository
	loanReqs *repotest.LoanRequestRepository
	admin    *repotest.AdminRepository
	users    *repotest.UserRepository
	notifs   *repotest.NotificationRepository
}

func newLoanRequestHandler() *loanTestDeps {
	copies := repotest.NewCopyRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	loanReqs := repotest.NewLoanRequestRepository(copies, notifs, users)
	admin := repotest.NewAdminRepository()
	waitlists := repotest.NewWaitlistRepository()
	workflow := services.NewLoanWorkflow(copies, loanReqs, notifs, users, waitlists, noopEmail())
	handler := NewLoanRequestHandler(copies, loanReqs, admin, users, workflow)
	return &loanTestDeps{handler: handler, copies: copies, loanReqs: loanReqs, admin: admin, users: users, notifs: notifs}
}

// seedOwnerAndBorrower creates an owner + an available copy they own, and a
// separate borrower, returning both users and the copy.
func seedOwnerAndBorrower(t *testing.T, d *loanTestDeps) (owner, borrower *models.User, bookCopy *models.Copy) {
	t.Helper()
	owner = &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, d.users.Create(owner))
	borrower = &models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, d.users.Create(borrower))

	bookCopy = &models.Copy{
		OwnerID: owner.ID,
		Owner:   *owner,
		Book:    models.Book{Title: "Some Book"},
		Status:  "available",
	}
	require.NoError(t, d.copies.Create(bookCopy))
	return owner, borrower, bookCopy
}

// newCreateLoanRequestInput builds a createLoanRequestInput for copyID with a
// default valid expected return date — every request needs one now, so tests
// that don't care about its exact value still need one that parses and isn't
// in the past.
func newCreateLoanRequestInput(copyID uint) *createLoanRequestInput {
	in := &createLoanRequestInput{}
	in.Body.CopyID = copyID
	in.Body.ExpectedReturnDate = "2099-01-01"
	return in
}

func TestCreateLoanRequest(t *testing.T) {
	t.Run("borrower can request an available copy", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := newCreateLoanRequestInput(bookCopy.ID)

		out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "pending", out.Body.Status)
		assert.Equal(t, 1, d.notifs.Count(), "owner should be notified")

		updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
		require.NoError(t, findErr)
		assert.Equal(t, "requested", updatedCopy.Status)
	})

	t.Run("owner cannot request their own copy", func(t *testing.T) {
		d := newLoanRequestHandler()
		owner, _, bookCopy := seedOwnerAndBorrower(t, d)

		input := newCreateLoanRequestInput(bookCopy.ID)

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("copy that is already requested cannot be requested again", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
		require.NoError(t, d.copies.UpdateStatus(bookCopy.ID, "requested"))

		input := newCreateLoanRequestInput(bookCopy.ID)

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("auto-approve copy is immediately accepted", func(t *testing.T) {
		d := newLoanRequestHandler()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Auto"}, Status: "available", AutoApprove: true}
		require.NoError(t, d.copies.Create(bookCopy))

		input := newCreateLoanRequestInput(bookCopy.ID)

		out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "accepted", out.Body.Status)
		assert.NotNil(t, out.Body.RespondedAt)
	})

	t.Run("missing return date is rejected", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("return date in the past is rejected", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID
		input.Body.ExpectedReturnDate = "2000-01-01"

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("max active loans setting rejects a borrower at the limit", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("max_active_loans", "1"))
		_, borrower, bookCopy1 := seedOwnerAndBorrower(t, d)

		input1 := newCreateLoanRequestInput(bookCopy1.ID)
		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input1)
		require.NoError(t, err)

		owner2 := &models.User{Name: "Owner2", Email: "owner2@example.com"}
		require.NoError(t, d.users.Create(owner2))
		bookCopy2 := &models.Copy{OwnerID: owner2.ID, Owner: *owner2, Book: models.Book{Title: "Second"}, Status: "available"}
		require.NoError(t, d.copies.Create(bookCopy2))

		input2 := newCreateLoanRequestInput(bookCopy2.ID)
		_, err = d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input2)

		require.Error(t, err)
		assertStatus(t, err, 422)
	})

	t.Run("verified-email requirement rejects an unverified borrower", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("require_verified_to_borrow", "true"))
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
		assert.False(t, borrower.Verified)

		input := newCreateLoanRequestInput(bookCopy.ID)
		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("min-books-shared requirement rejects a borrower who hasn't shared enough", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("verification_min_books_shared", "1"))
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := newCreateLoanRequestInput(bookCopy.ID)
		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})
}

func TestUpdateLoanRequest_AcceptRejectsCompetingRequests(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower1, bookCopy := seedOwnerAndBorrower(t, d)
	borrower2 := &models.User{Name: "Borrower2", Email: "borrower2@example.com"}
	require.NoError(t, d.users.Create(borrower2))

	in1 := newCreateLoanRequestInput(bookCopy.ID)
	out1, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower1.ID, "user"), in1)
	require.NoError(t, err)

	// A second request against the same copy is blocked once it's "requested" —
	// simulate a competing pending request directly via the repo to exercise
	// the accept-side rejection path.
	competing := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower2.ID, Status: "pending"}
	require.NoError(t, d.loanReqs.Create(competing))

	updateInput := &updateLoanRequestInput{ID: out1.Body.ID}
	updateInput.Body.Status = "accepted"

	out, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), updateInput)

	require.NoError(t, err)
	assert.Equal(t, "accepted", out.Body.Status)

	reloadedCompeting, findErr := d.loanReqs.GetByID(competing.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "rejected", reloadedCompeting.Status, "the other pending request should be auto-rejected")

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "loaned", updatedCopy.Status)
}

func TestUpdateLoanRequest_OnlyOwnerCanAcceptOrReject(t *testing.T) {
	d := newLoanRequestHandler()
	_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	input := newCreateLoanRequestInput(bookCopy.ID)
	out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)
	require.NoError(t, err)

	updateInput := &updateLoanRequestInput{ID: out.Body.ID}
	updateInput.Body.Status = "accepted"

	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), updateInput)

	require.Error(t, err)
	assertStatus(t, err, 403)
}

func TestUpdateLoanRequest_CancelByBorrowerFreesTheCopy(t *testing.T) {
	d := newLoanRequestHandler()
	_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	input := newCreateLoanRequestInput(bookCopy.ID)
	out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)
	require.NoError(t, err)

	updateInput := &updateLoanRequestInput{ID: out.Body.ID}
	updateInput.Body.Status = "cancelled"

	result, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), updateInput)

	require.NoError(t, err)
	assert.Equal(t, "cancelled", result.Body.Status)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "available", updatedCopy.Status)
}

func TestUpdateLoanRequest_ReturnUpdatesConditionAndFreesCopy(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	returnInput := &updateLoanRequestInput{ID: created.Body.ID}
	returnInput.Body.Status = "returned"
	returnInput.Body.NewCondition = "worn"
	result, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), returnInput)

	require.NoError(t, err)
	assert.Equal(t, "returned", result.Body.Status)
	assert.Equal(t, "worn", result.Body.Copy.Condition)
	require.NotNil(t, result.Body.ReturnedBy)
	assert.Equal(t, owner.ID, *result.Body.ReturnedBy)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "available", updatedCopy.Status)
}

func TestUpdateLoanRequest_BorrowerCanMarkReturned(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	returnInput := &updateLoanRequestInput{ID: created.Body.ID}
	returnInput.Body.Status = "returned"
	result, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), returnInput)

	require.NoError(t, err)
	assert.Equal(t, "returned", result.Body.Status)
	require.NotNil(t, result.Body.ReturnedBy)
	assert.Equal(t, borrower.ID, *result.Body.ReturnedBy)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "available", updatedCopy.Status)
}

func TestUpdateLoanRequest_ReturnByStrangerIsForbidden(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	stranger := &models.User{Name: "Stranger", Email: "stranger@example.com"}
	require.NoError(t, d.users.Create(stranger))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	returnInput := &updateLoanRequestInput{ID: created.Body.ID}
	returnInput.Body.Status = "returned"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, stranger.ID, "user"), returnInput)

	require.Error(t, err)
	assertStatus(t, err, 403)
}

func TestUpdateLoanRequest_UndoReturn_OwnerOnly(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	returnInput := &updateLoanRequestInput{ID: created.Body.ID}
	returnInput.Body.Status = "returned"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), returnInput)
	require.NoError(t, err)

	undoInput := &updateLoanRequestInput{ID: created.Body.ID}
	undoInput.Body.Status = "accepted"

	t.Run("borrower cannot undo", func(t *testing.T) {
		_, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), undoInput)
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("owner can undo", func(t *testing.T) {
		result, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), undoInput)
		require.NoError(t, err)
		assert.Equal(t, "accepted", result.Body.Status)
		assert.Nil(t, result.Body.ReturnedAt)
		assert.Nil(t, result.Body.ReturnedBy)

		updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
		require.NoError(t, findErr)
		assert.Equal(t, "loaned", updatedCopy.Status)
	})
}

func TestUpdateLoanRequest_UndoReturn_BlockedIfCopyReloaned(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	borrower2 := &models.User{Name: "Borrower2", Email: "borrower2@example.com"}
	require.NoError(t, d.users.Create(borrower2))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	returnInput := &updateLoanRequestInput{ID: created.Body.ID}
	returnInput.Body.Status = "returned"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), returnInput)
	require.NoError(t, err)

	// A different borrower requests and gets accepted for the now-available copy.
	create2 := newCreateLoanRequestInput(bookCopy.ID)
	created2, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower2.ID, "user"), create2)
	require.NoError(t, err)
	accept2 := &updateLoanRequestInput{ID: created2.Body.ID}
	accept2.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), accept2)
	require.NoError(t, err)

	undoInput := &updateLoanRequestInput{ID: created.Body.ID}
	undoInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), undoInput)

	require.Error(t, err)
	assertStatus(t, err, 409)

	reloadedNewLoan, findErr := d.loanReqs.GetByID(created2.Body.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "accepted", reloadedNewLoan.Status, "undo must not disturb the new borrower's active loan")
}

func TestUpdateExpectedReturnDate_EitherPartyWhileAccepted(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	stranger := &models.User{Name: "Stranger", Email: "stranger@example.com"}
	require.NoError(t, d.users.Create(stranger))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	t.Run("cannot set date while pending", func(t *testing.T) {
		dateInput := &updateExpectedReturnDateInput{ID: created.Body.ID}
		dateInput.Body.ExpectedReturnDate = "2026-09-01"
		_, err := d.handler.updateExpectedReturnDate(fakeAuthedCtx(t, borrower.ID, "user"), dateInput)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	t.Run("stranger is forbidden", func(t *testing.T) {
		dateInput := &updateExpectedReturnDateInput{ID: created.Body.ID}
		dateInput.Body.ExpectedReturnDate = "2026-09-01"
		_, err := d.handler.updateExpectedReturnDate(fakeAuthedCtx(t, stranger.ID, "user"), dateInput)
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("bad date format is rejected", func(t *testing.T) {
		dateInput := &updateExpectedReturnDateInput{ID: created.Body.ID}
		dateInput.Body.ExpectedReturnDate = "not-a-date"
		_, err := d.handler.updateExpectedReturnDate(fakeAuthedCtx(t, borrower.ID, "user"), dateInput)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("borrower can set the date", func(t *testing.T) {
		newDate := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
		dateInput := &updateExpectedReturnDateInput{ID: created.Body.ID}
		dateInput.Body.ExpectedReturnDate = newDate
		result, err := d.handler.updateExpectedReturnDate(fakeAuthedCtx(t, borrower.ID, "user"), dateInput)
		require.NoError(t, err)
		assert.Equal(t, newDate, result.Body.ExpectedReturnDate.Format("2006-01-02"))
	})

	t.Run("owner can update the date", func(t *testing.T) {
		newDate := time.Now().AddDate(0, 0, 60).Format("2006-01-02")
		dateInput := &updateExpectedReturnDateInput{ID: created.Body.ID}
		dateInput.Body.ExpectedReturnDate = newDate
		result, err := d.handler.updateExpectedReturnDate(fakeAuthedCtx(t, owner.ID, "user"), dateInput)
		require.NoError(t, err)
		assert.Equal(t, newDate, result.Body.ExpectedReturnDate.Format("2006-01-02"))
	})
}

func TestUpdateLoanRequest_InvalidTransition(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	updateInput := &updateLoanRequestInput{ID: created.Body.ID}
	updateInput.Body.Status = "bogus"

	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), updateInput)

	require.Error(t, err)
	assertStatus(t, err, 400)
}

func TestGetLoanRequest_ContactInfoOnlyRevealedWhenAccepted(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	owner.TelegramUsername = "@owner"
	owner.WhatsAppUsername = "+15550100"
	owner.ContactNote = "prefer evenings"
	require.NoError(t, d.users.Save(owner))
	bookCopy.Owner = *owner
	require.NoError(t, d.copies.Save(bookCopy))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	getInput := &getLoanRequestInput{ID: created.Body.ID}
	pending, err := d.handler.getLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), getInput)
	require.NoError(t, err)
	assert.Empty(t, pending.Body.Copy.Owner.Email, "contact info must stay hidden before acceptance")
	assert.Empty(t, pending.Body.Copy.Owner.TelegramUsername, "telegram must stay hidden before acceptance")
	assert.Empty(t, pending.Body.Copy.Owner.WhatsAppUsername, "whatsapp must stay hidden before acceptance")
	assert.Empty(t, pending.Body.Copy.Owner.ContactNote, "contact note must stay hidden before acceptance")

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	accepted, err := d.handler.getLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), getInput)
	require.NoError(t, err)
	assert.Equal(t, owner.Email, accepted.Body.Copy.Owner.Email, "contact info should be revealed once accepted")
	assert.Equal(t, "@owner", accepted.Body.Copy.Owner.TelegramUsername, "telegram should be revealed once accepted")
	assert.Equal(t, "+15550100", accepted.Body.Copy.Owner.WhatsAppUsername, "whatsapp should be revealed once accepted")
	assert.Equal(t, "prefer evenings", accepted.Body.Copy.Owner.ContactNote, "contact note should be revealed once accepted")
}

func TestGetLoanRequest_AccessDeniedToUninvolvedUser(t *testing.T) {
	d := newLoanRequestHandler()
	_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	stranger := &models.User{Name: "Stranger", Email: "stranger@example.com"}
	require.NoError(t, d.users.Create(stranger))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	getInput := &getLoanRequestInput{ID: created.Body.ID}
	_, err = d.handler.getLoanRequest(fakeAuthedCtx(t, stranger.ID, "user"), getInput)

	require.Error(t, err)
	assertStatus(t, err, 403)
}

func TestListLoanRequests_OnlyOwnerCanList(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	listInput := &listLoanRequestsInput{CopyID: bookCopy.ID}

	t.Run("owner can list", func(t *testing.T) {
		out, err := d.handler.listLoanRequests(fakeAuthedCtx(t, owner.ID, "user"), listInput)
		require.NoError(t, err)
		assert.Len(t, out.Body, 1)
	})

	t.Run("non-owner cannot list", func(t *testing.T) {
		_, err := d.handler.listLoanRequests(fakeAuthedCtx(t, borrower.ID, "user"), listInput)
		require.Error(t, err)
		assertStatus(t, err, 403)
	})
}

func TestListLoanRequests_ContactInfoOnlyRevealedWhenAccepted(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	owner.TelegramUsername = "@owner"
	owner.WhatsAppUsername = "+15550100"
	owner.ContactNote = "prefer evenings"
	require.NoError(t, d.users.Save(owner))
	bookCopy.Owner = *owner
	require.NoError(t, d.copies.Save(bookCopy))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	listInput := &listLoanRequestsInput{CopyID: bookCopy.ID}

	pending, err := d.handler.listLoanRequests(fakeAuthedCtx(t, owner.ID, "user"), listInput)
	require.NoError(t, err)
	require.Len(t, pending.Body, 1)
	assert.Empty(t, pending.Body[0].Copy.Owner.Email, "contact info must stay hidden before acceptance")
	assert.Empty(t, pending.Body[0].Copy.Owner.TelegramUsername, "telegram must stay hidden before acceptance")
	assert.Empty(t, pending.Body[0].Copy.Owner.WhatsAppUsername, "whatsapp must stay hidden before acceptance")
	assert.Empty(t, pending.Body[0].Copy.Owner.ContactNote, "contact note must stay hidden before acceptance")

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	accepted, err := d.handler.listLoanRequests(fakeAuthedCtx(t, owner.ID, "user"), listInput)
	require.NoError(t, err)
	require.Len(t, accepted.Body, 1)
	assert.Equal(t, "@owner", accepted.Body[0].Copy.Owner.TelegramUsername, "telegram should be revealed once accepted")
	assert.Equal(t, "+15550100", accepted.Body[0].Copy.Owner.WhatsAppUsername, "whatsapp should be revealed once accepted")
	assert.Equal(t, "prefer evenings", accepted.Body[0].Copy.Owner.ContactNote, "contact note should be revealed once accepted")
}

func TestListMine_ViewFilter(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, copy1 := seedOwnerAndBorrower(t, d)

	makeCopy := func() *models.Copy {
		c := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Another Book"}, Status: "available"}
		require.NoError(t, d.copies.Create(c))
		return c
	}
	copy2, copy3, copy4, copy5 := makeCopy(), makeCopy(), makeCopy(), makeCopy()

	create := func(c *models.Copy) uint {
		in := newCreateLoanRequestInput(c.ID)
		out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), in)
		require.NoError(t, err)
		return out.Body.ID
	}
	accept := func(id uint) {
		in := &updateLoanRequestInput{ID: id}
		in.Body.Status = "accepted"
		_, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), in)
		require.NoError(t, err)
	}

	_ = create(copy1) // left pending
	acceptedID := create(copy2)
	accept(acceptedID)

	rejectedID := create(copy3)
	rejectIn := &updateLoanRequestInput{ID: rejectedID}
	rejectIn.Body.Status = "rejected"
	_, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), rejectIn)
	require.NoError(t, err)

	cancelledID := create(copy4)
	cancelIn := &updateLoanRequestInput{ID: cancelledID}
	cancelIn.Body.Status = "cancelled"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), cancelIn)
	require.NoError(t, err)

	returnedID := create(copy5)
	accept(returnedID)
	returnIn := &updateLoanRequestInput{ID: returnedID}
	returnIn.Body.Status = "returned"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), returnIn)
	require.NoError(t, err)

	t.Run("no view returns every status", func(t *testing.T) {
		out, err := d.handler.listMine(fakeAuthedCtx(t, borrower.ID, "user"), &listMineInput{})
		require.NoError(t, err)
		assert.Equal(t, int64(5), out.Body.Total)
	})

	t.Run("current view returns only pending and accepted", func(t *testing.T) {
		out, err := d.handler.listMine(fakeAuthedCtx(t, borrower.ID, "user"), &listMineInput{View: "current"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), out.Body.Total)
		for _, item := range out.Body.Items {
			assert.Contains(t, []string{"pending", "accepted"}, item.Status)
		}
	})

	t.Run("history view returns only returned, rejected, and cancelled", func(t *testing.T) {
		out, err := d.handler.listMine(fakeAuthedCtx(t, borrower.ID, "user"), &listMineInput{View: "history"})
		require.NoError(t, err)
		assert.Equal(t, int64(3), out.Body.Total)
		for _, item := range out.Body.Items {
			assert.Contains(t, []string{"returned", "rejected", "cancelled"}, item.Status)
		}
	})
}

func TestListMineLending_ViewFilter(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, copy1 := seedOwnerAndBorrower(t, d)

	makeCopy := func() *models.Copy {
		c := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Another Book"}, Status: "available"}
		require.NoError(t, d.copies.Create(c))
		return c
	}
	copy2, copy3, copy4, copy5 := makeCopy(), makeCopy(), makeCopy(), makeCopy()

	create := func(c *models.Copy) uint {
		in := newCreateLoanRequestInput(c.ID)
		out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), in)
		require.NoError(t, err)
		return out.Body.ID
	}
	accept := func(id uint) {
		in := &updateLoanRequestInput{ID: id}
		in.Body.Status = "accepted"
		_, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), in)
		require.NoError(t, err)
	}

	_ = create(copy1) // left pending
	acceptedID := create(copy2)
	accept(acceptedID)

	rejectedID := create(copy3)
	rejectIn := &updateLoanRequestInput{ID: rejectedID}
	rejectIn.Body.Status = "rejected"
	_, err := d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), rejectIn)
	require.NoError(t, err)

	cancelledID := create(copy4)
	cancelIn := &updateLoanRequestInput{ID: cancelledID}
	cancelIn.Body.Status = "cancelled"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), cancelIn)
	require.NoError(t, err)

	returnedID := create(copy5)
	accept(returnedID)
	returnIn := &updateLoanRequestInput{ID: returnedID}
	returnIn.Body.Status = "returned"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), returnIn)
	require.NoError(t, err)

	t.Run("no view returns every status", func(t *testing.T) {
		out, err := d.handler.listMineLending(fakeAuthedCtx(t, owner.ID, "user"), &listMineInput{})
		require.NoError(t, err)
		assert.Equal(t, int64(5), out.Body.Total)
	})

	t.Run("current view returns only pending and accepted", func(t *testing.T) {
		out, err := d.handler.listMineLending(fakeAuthedCtx(t, owner.ID, "user"), &listMineInput{View: "current"})
		require.NoError(t, err)
		assert.Equal(t, int64(2), out.Body.Total)
		for _, item := range out.Body.Items {
			assert.Contains(t, []string{"pending", "accepted"}, item.Status)
		}
	})

	t.Run("history view returns only returned, rejected, and cancelled", func(t *testing.T) {
		out, err := d.handler.listMineLending(fakeAuthedCtx(t, owner.ID, "user"), &listMineInput{View: "history"})
		require.NoError(t, err)
		assert.Equal(t, int64(3), out.Body.Total)
		for _, item := range out.Body.Items {
			assert.Contains(t, []string{"returned", "rejected", "cancelled"}, item.Status)
		}
	})

	t.Run("the borrower calling listMineLending sees nothing — they own no copies", func(t *testing.T) {
		out, err := d.handler.listMineLending(fakeAuthedCtx(t, borrower.ID, "user"), &listMineInput{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), out.Body.Total)
	})
}

func TestListMineActive(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	pendingCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Pending Book"}, Status: "available"}
	require.NoError(t, d.copies.Create(pendingCopy))

	createInput := newCreateLoanRequestInput(bookCopy.ID)
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	pendingInput := newCreateLoanRequestInput(pendingCopy.ID)
	_, err = d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), pendingInput)
	require.NoError(t, err)

	t.Run("only returns accepted loans, with contact info revealed", func(t *testing.T) {
		out, err := d.handler.listMineActive(fakeAuthedCtx(t, borrower.ID, "user"), &struct{}{})
		require.NoError(t, err)
		require.Len(t, out.Body.Items, 1)
		item := out.Body.Items[0]
		assert.Equal(t, "accepted", item.Status)
		assert.Equal(t, owner.Email, item.Copy.Owner.Email, "owner contact should be revealed for an accepted loan")
	})

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := d.handler.listMineActive(fakeAuthedCtxNone(), &struct{}{})
		assertStatus(t, err, 401)
	})
}
