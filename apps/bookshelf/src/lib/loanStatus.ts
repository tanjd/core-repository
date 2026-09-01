// Shared "is this loan overdue" check — an accepted loan request's
// expected_return_date has passed. Used anywhere a due date is shown
// (ReturnDateCell, CurrentlyBorrowedCard, LendingTab, My Books) so all of
// them agree on the same rule.
export function isOverdue(expectedReturnDate?: string | null): boolean {
  return !!expectedReturnDate && new Date(expectedReturnDate) < new Date();
}
