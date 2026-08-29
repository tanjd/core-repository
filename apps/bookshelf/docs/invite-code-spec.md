# Invite-code registration — spec

**Status:** Proposed · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `User`, `AppSetting` (`require_registration_approval`, `allow_registration`),
the email-magic-link registration flow (`docs/magic-link-registration-spec.md`)

Let an existing member generate a shareable invite link. Anyone who registers through it skips
the admin-approval queue entirely, regardless of the `require_registration_approval` setting.
Registering without a valid invite code behaves exactly as it does today.

## Why now

`require_registration_approval` (added in #39) is all-or-nothing: every sign-up sits in the queue
until an admin acts, even when the new member was personally invited by someone already using the
app. An invite link lets a vouched-for signup skip that queue without turning approval off for
everyone else.

## Goals

- A member can generate a shareable invite link from their own account (`POST /invites`). Anyone
  who registers through it skips admin approval.
- Registering without a code, or with one that's gone invalid, is unaffected by this feature:
  still gated by `allow_registration` and `require_registration_approval` as today.
- Admins can see every outstanding invite code across all members (creator, use count, expiry,
  revoked state), with the ability to revoke one.
- An invite code that's expired, revoked, or at its use cap fails registration with a clear error
  — never a silent fallback to the normal approval-gated path.

## Non-goals (v1)

- **Tagging members with a group/community label for future multi-instance bookkeeping.** An
  earlier draft of this spec added a `Community` table, `User.CommunityID`, and matching admin UI
  so members could be traced back to a group if this app were ever self-hosted separately for a
  different group. Cut for now — that demand is still hypothetical (see `TODO.md`'s
  "Someday/hold" multi-tenant entry), and carrying an admin-facing `Community` list with exactly
  one row for an indefinite period isn't worth it. If a second group ever concretely needs to run
  its own instance, add the label then, scoped to that need — `InviteCode.CreatedByID` already
  gives precise who-invited-whom provenance to backfill from if that happens.
- **Admin-created invite codes independent of a member.** Every code has a real `CreatedByID`.
  There's no separate admin-only code type.
- **Notifying the inviter when their code gets used.** Nice-to-have; not required here, and easy
  to add later on top of the existing `Notification`/`EmailService` plumbing.
- **A cap on how many outstanding invite codes a member can hold.** Considered and rejected —
  members are already vetted (via approval or a prior invite) at this app's trusted, small-
  community scale, and the admin oversight page already gives a human full visibility and revoke
  over every code from every member. A cap would add config surface for an adversarial threat
  model this app doesn't have.
- **`max_uses`/`expires_at` configuration in the member-facing UI.** The fields exist on the
  API/model (and the 30-day default expiry still applies) so they're available for future use,
  but exposing per-code configuration to members is speculative complexity this feature doesn't
  need yet — see "Invite generation" below.

## Data model

New migration under `internal/db/migrations/`:

```go
// InviteCode lets an existing member invite someone who skips admin
// approval on registration.
type InviteCode struct {
    ID          uint       `gorm:"primarykey" json:"id"`
    Code        string     `gorm:"uniqueIndex;not null" json:"code"`
    CreatedByID uint       `gorm:"not null" json:"created_by_id"`
    MaxUses     *int       `json:"max_uses,omitempty"` // nil = unlimited
    UseCount    int        `gorm:"not null;default:0" json:"use_count"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
    Revoked     bool       `gorm:"not null;default:false" json:"revoked"`
    CreatedAt   time.Time  `json:"created_at"`
}
```

- `User` gains `InviteCodeID *uint` (nullable — exactly which code they registered through; `nil`
  for approval-path, `/auth/setup`, and pre-migration users).
- `PendingRegistrationData` (the parked row behind email verification — see
  `magic-link-registration-spec.md`) gains `PendingInviteCode string`, following the same pattern
  as its other `Pending*` fields: captured at the first registration step, never itself
  serialized back to the client.
- `ExpiresAt` defaults to **30 days from creation** whenever `POST /invites` omits it — a code
  that bypasses admin approval shouldn't default to never expiring. A caller can still pass an
  explicit far-future (or shorter) date to override the default; omitting the field just means
  "use the default," not "forever." This matters more than it would otherwise because notifying
  the inviter when their code gets used is an explicit non-goal — an unlimited, never-expiring
  link that leaks (forwarded, screenshotted, bookmarked) would otherwise be a silent, permanent
  hole in the approval gate.

## Registration flow

Both entry points into `verifyRegisterEmailOTP` → `finalizeRegistration`
(`apps/bookshelf-backend/internal/handlers/auth.go`) already re-check `allow_registration` at
finalize time in case an admin changed it during the 15-minute verification window; invite-code
handling follows the same "check at send, re-check at finalize" shape:

- **`sendRegisterEmailOTP`** accepts an optional `invite_code` field. If present, resolve it and
  reject with 400 if it's unknown, revoked, expired, or already at `MaxUses` — same style as this
  handler's existing duplicate-email and weak-password checks. A valid code is stored on
  `PendingRegistrationData.PendingInviteCode`.
- **`finalizeRegistration`** re-resolves `PendingInviteCode` before creating the user (the code
  could have been revoked, expired, or exhausted by someone else in the intervening 15 minutes).
  - **Valid code:** set `user.InviteCodeID` from it, increment `InviteCode.UseCount`, and do
    **not** set `PendingApproval` — even if `require_registration_approval` is `"true"`.
    Invite-code registration always bypasses that gate; it does not bypass `allow_registration`
    (an admin's global kill switch still wins).
  - **No code, or it went stale:** unchanged from today — `require_registration_approval` applies
    as it already does.
- The check-then-increment on `UseCount` isn't wrapped in extra locking. This mirrors the app's
  existing tolerance for narrow check-then-act races on low-traffic, self-hosted-scale admin
  surfaces (e.g. `LoanRequestHandler.getRequestableCopy`'s availability check) — not worth new
  transaction machinery for the volume this app actually sees. Accepted risk, named here rather
  than silently ignored, matching this repo's habit of documenting known gaps (see
  `apps/bookshelf-backend/CLAUDE.md`'s "Known gaps" section for the pattern).
- **New public endpoint**, `GET /invites/{code}` (no auth): returns whether the code is currently
  redeemable, and nothing else (no inviter identity, no use count) — just enough for the register
  page to confirm "this invite is valid" before an account exists.

## Invite generation ("invite a friend")

New `InviteHandler`, mounted alongside the other `internal/handlers/*.go` handlers:

- `POST /invites` (authenticated, any member) — creates a code inheriting `CreatedByID` from the
  caller, with optional `max_uses`/`expires_at` fields in the body (not surfaced in the v1 UI —
  see "Non-goals"); `expires_at` defaults to 30 days out when omitted (see "Data model").
- `GET /invites/mine` (authenticated) — the caller's own codes plus use counts, for the frontend's
  "your invites" list and for the reuse check below.
- `POST /invites/{id}/revoke` (authenticated — owner or admin) — flips `Revoked`.

Frontend: a new "Invite a friend" section on the profile page (`apps/bookshelf/src/app/(auth)`'s
profile route), next to the existing contact/notification settings — not a new bottom-tab-bar
destination, given the fixed 5-slot mobile nav budget documented in `apps/bookshelf/CLAUDE.md`.
Tapping "Invite a friend" checks `GET /invites/mine` for an existing active code (not expired,
revoked, or at its use cap) and reuses it if found; `POST /invites` is only called when there is
no active code to show. This keeps a member at effectively one live link at a time rather than a
new code minted on every tap — a simpler mental model, and it sidesteps needing any UI to manage
a growing list of dead codes. The resulting `/register?invite=<code>` link is shared via
`navigator.share()` on mobile (the OS share sheet — Messages, WhatsApp, etc.), falling back to a
copy-to-clipboard button where the Web Share API isn't available. The section also lists the
member's own codes with use count and a revoke button — expected to rarely hold more than one or
two entries, given the reuse behavior above.

The register page (`apps/bookshelf/src/app/(auth)/register/page.tsx`) resolves `?invite=` against
`GET /invites/{code}` on page load, before the user has typed anything:

- **Valid:** shows a confirmation banner ("You've been invited — skip the approval wait") rather
  than a visible code field, and submits the code as part of registration.
- **Invalid** (expired, revoked, unknown): shows a dismissible notice ("This invite link isn't
  valid anymore — you can still sign up, but your account will need admin approval") and drops
  the code from the submission entirely, falling through to today's normal registration instead
  of surfacing a submit-time error after the user has filled out the whole form.

A code entered manually (no `?invite=` param) goes through a collapsed "Have an invite code?"
toggle on the details step rather than a default-visible field — most registrants arrive via the
link, not by typing a code, so the field shouldn't compete for attention with the rest of the
form.

## Admin surfaces

- New admin page (or a tab alongside an existing one) listing every outstanding `InviteCode`
  across all members — creator, use count/limit, expiry, revoked state — with an admin-level
  revoke action. Follows the existing Users/Backups/Jobs admin page conventions: a table plus row
  actions, reusing this app's existing four-variant badge vocabulary for the code's state:
  `success` (active), `outline` (expired), `secondary` (at its use cap), `destructive` (revoked).
  Revoked gets the loudest treatment since it's the only state reflecting a deliberate action;
  expiry and hitting the use cap are both benign natural-lifecycle outcomes.

## Backend changes

| Area                                            | Change                                                                                    |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `models.go`                                     | New `InviteCode` struct; `User.InviteCodeID`; `PendingRegistrationData.PendingInviteCode` |
| Migration                                       | New pair: create `invite_codes` table, add `invite_code_id` to `users`                    |
| `sendRegisterEmailOTP` / `finalizeRegistration` | Validate + park invite code; bypass `PendingApproval` on redemption                       |
| New public endpoint                             | `GET /invites/{code}` — validity only                                                     |
| New `InviteHandler`                             | `POST /invites`, `GET /invites/mine`, `POST /invites/{id}/revoke`                         |
| `AdminHandler`                                  | Cross-member invite-code listing/revoke                                                   |
| `repository` layer                              | `InviteCodeRepository`, following the existing repository interface pattern               |

## Frontend changes

| Area                                  | Change                                                                              |
| ------------------------------------- | ----------------------------------------------------------------------------------- |
| Register page                         | Reads `?invite=`, confirms validity, submits the code with registration             |
| Profile page                          | New "Invite a friend" section: generate a link, list own codes + use counts, revoke |
| New admin page                        | Invite-code oversight table with revoke                                             |
| `src/lib/api.ts` / `src/lib/types.ts` | New types/calls for `InviteCode` and the invite endpoints                           |

## Testing

- **Backend:** invite-code validation (valid, expired, revoked, at-cap, unknown) at both
  `sendRegisterEmailOTP` and `finalizeRegistration`; approval bypass on successful redemption;
  fallback to today's approval-gated behavior with no code; `UseCount` increments; revoke
  endpoint against owner, non-owner, and admin callers; `POST /invites` applies the 30-day
  default `ExpiresAt` when the field is omitted and respects an explicit override when it's
  provided.
- **E2E (`apps/bookshelf-e2e`):** register via a generated invite link end-to-end (skips the
  pending-approval screen); register with an expired/revoked code surfaces the right error;
  register with no code still hits the approval gate when `require_registration_approval` is on.
  The last case also closes an existing coverage gap — admin approval itself has no e2e coverage
  today.

## Resolved decisions

- **`Community`/group tagging cut from this spec.** See the "Non-goals" entry above — deferred
  until a second self-hosted group is an actual, not hypothetical, need.
- **Invite codes are member-generated, not admin-only.** The ask was specifically an "invite a
  friend" flow; admins get oversight/revoke over codes but don't hand-author them.
- **"Invite a friend" reuses an existing active code rather than minting a new one on every tap.**
  Keeps a member at effectively one live link at a time — a simpler mental model than a
  Discord-style "every share is a new code" pattern, and it avoids needing UI to manage a growing
  list of mostly-dead codes.
- **No `max_uses`/`expires_at` configuration in the v1 member-facing UI.** The API/model support
  it and the 30-day default still applies, but exposing per-code config to members is speculative
  complexity this feature doesn't need yet — see "Non-goals".
- **Per-code configurable `MaxUses`/`ExpiresAt`, not a fixed policy — but `ExpiresAt` defaults to
  30 days, never unlimited.** Matches this app's existing per-item configurability conventions
  (`WishlistRequest`, backup retention) rather than a single global invite policy, while avoiding
  an approval-bypassing link that silently lives forever if a member never revokes it — especially
  since redemption notifications to the inviter are out of scope (see "Non-goals").
- **Check-then-increment `UseCount` with no added locking.** Accepted risk at this app's actual
  self-hosted scale — see "Registration flow" above.

## Implementation order

1. `InviteCode` model + migration (new `invite_codes` table, `invite_code_id` column on `users`)
   — smallest independently-shippable slice.
2. Invite-code validation and redemption in the registration flow (`sendRegisterEmailOTP`,
   `finalizeRegistration`) + the public `GET /invites/{code}` lookup.
3. `InviteHandler` (create/list-mine/revoke) + the profile page's "Invite a friend" UI.
4. Register page invite-link handling (`?invite=` param, validity display).
5. Admin invite-code oversight page.
