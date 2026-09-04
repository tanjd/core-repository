# Telegram notification preferences — spec

**Status:** Implemented (#109) · **Scope:** `apps/bookshelf` +
`apps/bookshelf-backend` · **Depends on:** `User.EmailNotificationsEnabled`,
[`telegram-bot-integration-spec.md`](./telegram-bot-integration-spec.md)
(`User.TelegramChatID`/`TelegramLinkedAt`)

Let a member independently turn Telegram push notifications on or off, the same way
`email_notifications_enabled` already lets them turn transactional email on or off — rather
than "linked" being the only on/off signal.

## Why

`telegram-bot-integration-spec.md` proposes `TelegramChatID != nil` as the sole gate for
sending a member Telegram notifications — linking _is_ opting in, with no way to pause
delivery short of unlinking. That's fine as a v1 simplification, but it's inconsistent with
every other notification channel in this app: `EmailNotificationsEnabled` and
`MonthlyDigestEnabled` are both independent, revisitable toggles in `ProfileForm.tsx`, not
tied to some other identity state. A member should be able to temporarily mute Telegram
(e.g. traveling, don't want phone buzzing) without losing the linked account and having to
redo the deep-link flow later.

## Goals

- A member who has linked Telegram gets a third independent toggle, **"Telegram
  notifications"**, alongside the existing "Email notifications" and "Monthly digest"
  switches in `ProfileForm.tsx`.
- Email and Telegram notifications are fully independent: a member can have both on, either
  on, or both off. No precedence/suppression logic between channels — matches how email and
  the in-app `Notification` bell already both fire today with no coordination between them.
- Linking Telegram defaults the new toggle **on** (the member just took an explicit action to
  connect it — defaulting off would make linking feel like it did nothing). Unlinking does
  _not_ require the toggle to be off first, and clears it back to the unset/default state.
- Linking/unlinking Telegram never touches `EmailNotificationsEnabled` — no silent side
  effects on an existing, unrelated setting.

## Non-goals

- No per-notification-type channel routing (e.g. "loan requests via Telegram, wishlist via
  email") — both new toggles proposed here, like the existing email one, are all-or-nothing
  across every notification type. Per-type preferences remain a `telegram-bot-integration-spec.md`
  v2 item, not introduced here either.
- No "preferred channel" concept or delivery precedence — see Goals above.

## Data model changes

Add to `User` (`internal/models/models.go`), same migration as
`telegram-bot-integration-spec.md`'s `000020_add_telegram_link` (this field is meaningless
without `TelegramChatID`, so bundling avoids a second migration touching the same feature):

```go
TelegramNotificationsEnabled bool `gorm:"column:telegram_notifications_enabled;not null;default:true" json:"telegram_notifications_enabled"`
```

Defaults `true` at the column level so it behaves sensibly even if ever read before a link
exists (irrelevant in practice, since the gate is always `TelegramChatID != nil &&
TelegramNotificationsEnabled`, but keeps the column's own default meaningful rather than
relying entirely on app-level set-on-link logic).

## Behavior

- **On link** (`POST /internal/telegram/confirm-link` handler, per the integration spec):
  set `TelegramChatID`, `TelegramLinkedAt`, and `TelegramNotificationsEnabled = true` in the
  same update.
- **On unlink** (`DELETE /profile/telegram/link`): clear `TelegramChatID`,
  `TelegramLinkedAt`, and reset `TelegramNotificationsEnabled` to `true` (its default) so a
  future re-link starts from the same "just connected" default rather than carrying forward a
  stale preference from a previous link.
- **Toggle while linked**: `PATCH /profile` gains
  `telegram_notifications_enabled *bool` alongside the existing
  `email_notifications_enabled *bool` field in `auth_profile.go`'s update body — same
  optional-pointer pattern, same handler, no new endpoint. Rejected (400) if the user has no
  `TelegramChatID` — there's nothing to toggle before linking, so the frontend hides/disables
  the switch until linked rather than relying solely on a backend error, but the backend still
  guards it since profile updates aren't otherwise validated against link state.
- **Notification gate**: every `w.telegram.NotifyAsync(...)` call site added per
  `telegram-bot-integration-spec.md`'s "Notification hook points" section changes its guard
  from `recipient.TelegramChatID != nil` to
  `recipient.TelegramChatID != nil && recipient.TelegramNotificationsEnabled` — mirrors the
  existing `if bookCopy.Owner.EmailNotificationsEnabled { ... }` shape exactly, just against
  the new field. The due-date reminder job's gate gets the same treatment.

## Frontend changes

`ProfileForm.tsx`: add a `telegramNotificationsEnabled` state (same `useState` +
`onCheckedChange` shape as `emailNotificationsEnabled`/`monthlyDigestEnabled`), rendered as a
third switch row directly below "Monthly digest". Visibility:

- **Not linked**: row hidden (or shown disabled with a "Connect Telegram first" hint —
  whichever this app's existing pattern favors for a setting gated on a prerequisite; check
  how `monthly_digest_enabled` behaves for an unverified/pending account, if there's a
  precedent, before inventing a new one).
- **Linked**: row shown, switch reflects `telegram_notifications_enabled`, saves through the
  existing `PATCH /profile` call in `ProfileForm.tsx` alongside the other two toggles (same
  submit handler, one more field in the payload).

## Files to touch

- `internal/models/models.go` — `User.TelegramNotificationsEnabled` (bundled into
  `telegram-bot-integration-spec.md`'s `000020_add_telegram_link` migration).
- `internal/handlers/auth_profile.go` — add `telegram_notifications_enabled *bool` to the
  profile update request body and apply-if-present logic, next to the existing
  `email_notifications_enabled` handling; add the has-`TelegramChatID` guard.
- `internal/handlers/` — the link/unlink handlers from the integration spec additionally
  set/reset `TelegramNotificationsEnabled` as described above.
- `internal/services/loan_workflow.go`, `internal/services/wishlist_workflow.go`,
  `internal/services/scheduler.go` — update the `TelegramChatID != nil` guards added by the
  integration spec to also check `TelegramNotificationsEnabled`.
- `apps/bookshelf/src/components/ProfileForm.tsx` — third switch row, state, payload field.

## Verification

- Backend: extend `auth_profile_test.go` (or equivalent) for: toggling
  `telegram_notifications_enabled` while linked succeeds; toggling while unlinked is
  rejected; link sets it `true`; unlink resets it to `true`.
- Backend: extend the fake-`TelegramNotifier` workflow tests from the integration spec with a
  case where `TelegramChatID` is set but `TelegramNotificationsEnabled` is `false` — assert no
  Telegram send, and (separately) that email still fires independently per the "no
  suppression" goal above.
- Frontend: `ProfileForm` test coverage (existing pattern for the other two switches) extended
  for the new switch's visibility (hidden/disabled pre-link) and save behavior.
