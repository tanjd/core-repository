package handlers

import (
	"testing"

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

func TestCreateLoanRequest(t *testing.T) {
	t.Run("borrower can request an available copy", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

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

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("copy that is already requested cannot be requested again", func(t *testing.T) {
		d := newLoanRequestHandler()
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
		require.NoError(t, d.copies.UpdateStatus(bookCopy.ID, "requested"))

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

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

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

		out, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "accepted", out.Body.Status)
		assert.NotNil(t, out.Body.RespondedAt)
	})

	t.Run("return date required but not supplied is rejected", func(t *testing.T) {
		d := newLoanRequestHandler()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Dated"}, Status: "available", ReturnDateRequired: true}
		require.NoError(t, d.copies.Create(bookCopy))

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID

		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("max active loans setting rejects a borrower at the limit", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("max_active_loans", "1"))
		_, borrower, bookCopy1 := seedOwnerAndBorrower(t, d)

		input1 := &createLoanRequestInput{}
		input1.Body.CopyID = bookCopy1.ID
		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input1)
		require.NoError(t, err)

		owner2 := &models.User{Name: "Owner2", Email: "owner2@example.com"}
		require.NoError(t, d.users.Create(owner2))
		bookCopy2 := &models.Copy{OwnerID: owner2.ID, Owner: *owner2, Book: models.Book{Title: "Second"}, Status: "available"}
		require.NoError(t, d.copies.Create(bookCopy2))

		input2 := &createLoanRequestInput{}
		input2.Body.CopyID = bookCopy2.ID
		_, err = d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input2)

		require.Error(t, err)
		assertStatus(t, err, 422)
	})

	t.Run("verified-email requirement rejects an unverified borrower", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("require_verified_to_borrow", "true"))
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
		assert.False(t, borrower.Verified)

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID
		_, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("min-books-shared requirement rejects a borrower who hasn't shared enough", func(t *testing.T) {
		d := newLoanRequestHandler()
		require.NoError(t, d.admin.UpsertSetting("verification_min_books_shared", "1"))
		_, borrower, bookCopy := seedOwnerAndBorrower(t, d)

		input := &createLoanRequestInput{}
		input.Body.CopyID = bookCopy.ID
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

	in1 := &createLoanRequestInput{}
	in1.Body.CopyID = bookCopy.ID
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

	input := &createLoanRequestInput{}
	input.Body.CopyID = bookCopy.ID
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

	input := &createLoanRequestInput{}
	input.Body.CopyID = bookCopy.ID
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

	createInput := &createLoanRequestInput{}
	createInput.Body.CopyID = bookCopy.ID
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

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "available", updatedCopy.Status)
}

func TestUpdateLoanRequest_InvalidTransition(t *testing.T) {
	d := newLoanRequestHandler()
	owner, borrower, bookCopy := seedOwnerAndBorrower(t, d)

	createInput := &createLoanRequestInput{}
	createInput.Body.CopyID = bookCopy.ID
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

	createInput := &createLoanRequestInput{}
	createInput.Body.CopyID = bookCopy.ID
	created, err := d.handler.createLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), createInput)
	require.NoError(t, err)

	getInput := &getLoanRequestInput{ID: created.Body.ID}
	pending, err := d.handler.getLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), getInput)
	require.NoError(t, err)
	assert.Empty(t, pending.Body.Copy.Owner.Email, "contact info must stay hidden before acceptance")

	acceptInput := &updateLoanRequestInput{ID: created.Body.ID}
	acceptInput.Body.Status = "accepted"
	_, err = d.handler.updateLoanRequest(fakeAuthedCtx(t, owner.ID, "user"), acceptInput)
	require.NoError(t, err)

	accepted, err := d.handler.getLoanRequest(fakeAuthedCtx(t, borrower.ID, "user"), getInput)
	require.NoError(t, err)
	assert.Equal(t, owner.Email, accepted.Body.Copy.Owner.Email, "contact info should be revealed once accepted")
}

func TestGetLoanRequest_AccessDeniedToUninvolvedUser(t *testing.T) {
	d := newLoanRequestHandler()
	_, borrower, bookCopy := seedOwnerAndBorrower(t, d)
	stranger := &models.User{Name: "Stranger", Email: "stranger@example.com"}
	require.NoError(t, d.users.Create(stranger))

	createInput := &createLoanRequestInput{}
	createInput.Body.CopyID = bookCopy.ID
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

	createInput := &createLoanRequestInput{}
	createInput.Body.CopyID = bookCopy.ID
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
