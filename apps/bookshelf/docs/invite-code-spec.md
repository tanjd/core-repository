# Invite-code registration — spec

**Status:** Implemented (#85; admin UX/discoverability polish in #93) · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `User`, `AppSetting`, `RegistrationVerification`, `PendingRegistrationData`,
`AuthHandler`, `AdminHandler`

A verified member has a personal invite link. Someone who registers via that link bypasses
the admin approval gate — the inviter has vouched for them. It fills the gap between fully-open
and fully-closed registration: an admin can keep `allow_registration = false` (no random signups)
while still letting members bring in specific, known people frictionlessly.

## Why now

- `require_registration_approval` creates friction even for trusted community members; an admin
  spending time approving someone a current member already vouched for is waste.
- Some communities want closed registration (no anonymous signups) but still need a way to
  onboard known members — invite links are the natural solution, with no new infrastructure.
- Members already expect this pattern from consumer apps (Dropbox, Revolut, group-chat
  apps) — the UX is a single shareable link, nothing more.

## Goals

- Every verified, active member has one permanent invite link they can share.
- Someone who registers via a valid invite link bypasses admin approval (`PendingApproval`
  stays false) even if `require_registration_approval` is on.
- Invite links also bypass `allow_registration = false` — this enables a "closed but
  controlled" onboarding mode: admin turns off open registration, members hand out their
  personal link as the sole entry point.
- The link is multi-use — the same link works for every new member the inviter brings in.
- A member can regenerate their link at any time (e.g., if they shared it too widely).
- Admins can see every member's link and revoke any one of them from the Users admin page.
- A global toggle is admin-configurable via the existing `AppSetting` mechanism.

## Non-goals (v1)

- **No per-member limit on signups via a link.** A member's link can create any number of
  accounts. If abuse is suspected, the admin revokes the link.
- **No expiry on invite links.** Links are permanent until revoked. The admin toggle
  (`allow_invite_codes`) is the circuit-breaker; per-link expiry adds complexity with
  little benefit at community scale.
- **No admin-generated links on behalf of a member.** Admins can use their own link
  (they're verified members), but cannot act as another member's inviter.
- **No email delivery of the invite.** The member copies the link and shares it however
  they like — the backend doesn't email it anywhere.
- **No "invited by" attribution on the invitee's public profile.** The relationship is
  tracked in `User.InvitedByID` for admin visibility, but there's no member-facing display.
- **No invite-code bypass of email OTP verification.** The invitee still goes through the
  normal email verification step. The link only removes the admin approval gate.
- **No account expiry for the resulting signup.** Once an account is created via an invite
  link, it lives on the same terms as any other account.
- **No notification to the inviter when their link is used.** The inviter can see that
  their link was used via the admin Users page; a push/email notification is deferred until
  there is evidence members want it.

## Who can create invite links

Eligible creators: members who are verified (`Verified = true`), not suspended
(`Suspended = false`), and not pending approval (`PendingApproval = false`). An unverified
member cannot hold a link — they haven't proved their own identity yet. Admins qualify
by virtue of being verified, non-suspended members with the `admin` role, same as any other
eligible member.

## Link lifecycle

1. Member opens their profile page → "Invite a member" section. The backend lazily creates
   their invite link on first access — subsequent visits always return the same link.
2. `GET /invite-code` (auth required) returns `{code, url}`. The member copies
   `{FRONTEND_ORIGIN}/register?invite={code}` and shares it however they like.
3. Invitee opens the URL. The registration page calls `GET /auth/invite/{code}` (public,
   no auth). If valid, a banner reads **"Invited by [Name] — your account will be approved
   automatically."**
4. Invitee fills in the registration form. `POST /auth/register/send-email-otp` body
   includes `invite_code`; the backend validates the code exists (is not revoked) and parks
   it in `PendingInviteCode` alongside the other pending fields.
5. Invitee verifies their email (OTP code or magic link — unchanged from today).
6. `POST /auth/register/verify-email-otp` re-validates the code (guarding against the
   inviter revoking or regenerating during the OTP window), creates the account with
   `PendingApproval = false` and `InvitedByID` set to the inviter's user ID.
7. The new account is live immediately — the invitee is signed in with a JWT. No approval
   queue.
8. The invite link remains active. The same link works for the next invitee.
9. If the member wants to invalidate the current link (e.g., they shared it too widely),
   they click "Regenerate link" → `POST /invite-code/regenerate` issues a new code. The
   old link stops working immediately.

**Code format:** 8-character lowercase alphanumeric (`a3kf92mx`), generated via `crypto/rand`.
36^8 ≈ 2.8 trillion combinations — effectively unguessable at community scale, short enough to
look clean in a shared URL. Consistent with Discord server invite links (all link-based, never
manually typed). A `uniqueIndex` on `code` handles the negligible collision probability.

## Admin controls (AppSettings)

| Key                  | Default  | Meaning                                                                                                                                                                                                                                                                                      |
| -------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `allow_invite_codes` | `"true"` | Global toggle. When false, no new links can be generated and `GET /invite-code` returns 403 for members who don't yet have one. Existing links already in use (code parked in the 15-minute OTP window) are not invalidated — disabling mid-session doesn't break an in-flight registration. |

`allow_invite_codes = false` is checked at link **access** only (creation and regeneration).
Validity checks at registration time (`send-email-otp` and `verify-email-otp`) do not
re-check this setting — a link that was legitimately issued remains valid while it exists.

## Registration flow changes

`PendingRegistrationData` gains one new field: `PendingInviteCode string` — the raw code
string stored alongside the other pending fields for the 15-minute OTP window.

**At `send-email-otp`:**

- If `invite_code` is present in the body: look up the code via `FindByCode`. If not found
  (revoked or never existed) → 400 "invite code is invalid or has already been revoked."
  If valid → park in `PendingInviteCode`.
- If `invite_code` is absent **and** `allow_registration = false` → 403, same as today.
- If `invite_code` is absent → proceed as today (no behavior change for non-invite
  registrations).

**At `verify-email-otp`:**

- If `PendingInviteCode != ""`: re-validate the code (the inviter may have regenerated or
  been suspended during the OTP window); if found → create the account with
  `PendingApproval = false` and `InvitedByID` set, regardless of
  `require_registration_approval`; if not found → 400.
- If `PendingInviteCode == ""`: proceed as today.

Neither `require_registration_approval` nor `allow_registration` is checked on the
invite-code registration path — both gates are bypassed when a valid code is present at
account-creation time.

## Backend changes

| Area                              | Change                                                                                                                                                             |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `InviteCode` model                | New struct in `models.go` (see below)                                                                                                                              |
| `User` model                      | New `InvitedByID *uint` field — permanent record of who invited this member, even after the code is regenerated or revoked                                         |
| `PendingRegistrationData`         | New `PendingInviteCode string` column (embedded in `RegistrationVerification`)                                                                                     |
| Migrations                        | `000017_create_invite_codes.{up,down}.sql`, `000018_add_pending_invite_code.{up,down}.sql`, `000019_add_invited_by_id.{up,down}.sql`                               |
| `InviteCodeRepository`            | New interface + GORM impl: `FindOrCreateByInviter`, `FindByCode`, `Regenerate`, `DeleteByInviter`, `ListAll`, `DeleteByID`                                         |
| `GET /invite-code`                | Auth required (verified, non-suspended, non-pending). Idempotent get-or-create. Returns `{code, url}`. 403 if `allow_invite_codes = false` and no code exists yet. |
| `POST /invite-code/regenerate`    | Auth required. Revokes current code and issues a new one. Returns `{code, url}`. 403 if `allow_invite_codes = false`.                                              |
| `GET /auth/invite/{code}`         | **Public, no auth.** Returns `{valid bool, inviter_name string}`. Does not modify anything. `inviter_name` is empty when `valid` is false.                         |
| `GET /admin/invite-codes`         | Admin. Lists every member who has a code: inviter name, code, created\_at. Not paginated — one row per member, bounded by community size.                          |
| `DELETE /admin/invite-codes/{id}` | Admin. Revokes a member's code by its ID. The member's next `GET /invite-code` will lazily issue a new one.                                                        |
| AppSettings (seeded)              | `allow_invite_codes`                                                                                                                                               |
| `send-email-otp` body             | New optional `invite_code string` field                                                                                                                            |
| `verify-email-otp`                | Re-validates `PendingInviteCode`; bypasses approval gate; sets `InvitedByID` on the new user                                                                       |
| `deleteUser` (admin)              | Revokes the deleted member's invite code so a removed member can't continue bringing in new signups                                                                |
| `suspendUser` (admin)             | Revokes the suspended member's invite code for the same reason                                                                                                     |

**`InviteCode` model:**

```go
type InviteCode struct {
    ID        uint      `gorm:"primarykey" json:"id"`
    Code      string    `gorm:"uniqueIndex;not null" json:"code"`
    InviterID uint      `gorm:"uniqueIndex;not null" json:"inviter_id"`
    Inviter   User      `json:"inviter,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}
```

The `uniqueIndex` on `InviterID` enforces one code per member at the database level.
There is no `ExpiresAt`, `UsedAt`, or `UsedByID` — the link is permanent and multi-use.
Who was invited by whom is tracked in `User.InvitedByID`, not on the code itself.

**New handler file:** `internal/handlers/invite_codes.go` — keeps invite-code logic out of
the already large `auth.go`. Route registration in `cmd/server/main.go`.

## Frontend changes

| Area                                                   | Change                                                                                                                                                                                                                                                                       |
| ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Profile page                                           | New "Invite a member" section: shows the member's invite link with a copy button and a "Regenerate link" action. On first load, `GET /invite-code` lazily creates the code. If `allow_invite_codes = false` and no code exists, shows "Invite links are currently disabled." |
| Registration page (`src/app/(auth)/register/page.tsx`) | Reads `?invite=` on mount; calls `GET /auth/invite/{code}`; if valid shows "Invited by [Name] — your account will be approved automatically"; passes `invite_code` in the `send-email-otp` request                                                                           |
| Admin Users page (`src/app/admin/users/page.tsx`)      | New "Invite links" section: table of all member codes (inviter name, created\_at); revoke button per row calling `DELETE /admin/invite-codes/{id}`                                                                                                                           |

The registration page banner appears only if `?invite=` is present **and** the code is
valid (`valid: true` from the public endpoint). An invalid or revoked code shows a brief
notice ("This invite link is no longer valid — you can still register normally") rather
than a hard error, since open registration may still be on.

## Testing

**Backend (TDD — write tests first):**

- `InviteCodeRepository`: `FindOrCreateByInviter` creates on first call and returns the
  same row on subsequent calls; `Regenerate` changes the code and invalidates the old one;
  `DeleteByInviter` removes the row; `ListAll` includes all members with codes.
- `GET /invite-code`: creates code on first call; returns same code on subsequent calls;
  403 when `allow_invite_codes = false` and no code exists; returns existing code even
  when `allow_invite_codes = false` (creation-only gate).
- `POST /invite-code/regenerate`: returns a new code; old code returns `{valid: false}`
  from the public endpoint; 403 when `allow_invite_codes = false`.
- `GET /auth/invite/{code}`: valid code → `{valid: true, inviter_name}`; revoked code →
  `{valid: false}`; unknown code → `{valid: false}`.
- Registration with invite code: valid code bypasses approval even with
  `require_registration_approval = true`; valid code enables registration with
  `allow_registration = false`; revoked code at `send-email-otp` → 400; code revoked
  between send and verify → 400 at `verify-email-otp`; no code + `allow_registration
= false` → 403, as today. Resulting user has `invited_by_id` set.
- Admin endpoints: list returns one row per member; admin can revoke any code; revoked
  member's next `GET /invite-code` creates a fresh code.
- `deleteUser`: removes the member's invite code; `suspendUser`: same.

**E2E (`apps/bookshelf-e2e`):**

- Full invite flow: logged-in member copies their invite link → opens it in an anonymous
  context → registration page shows the "Invited by" banner → registers → account is
  immediately active (no pending approval), confirmed via `GET /auth/me` and the admin
  Users page.
- Same flow with `require_registration_approval = true` to confirm the bypass holds.
- Member regenerates link → old URL shows "no longer valid" notice; new URL shows the
  banner again.
- Admin revokes a code via the Users page → that URL returns `{valid: false}`.

## Resolved decisions

- **Invite links bypass `allow_registration = false`:** This is the primary use case — a
  community that wants closed registration with controlled onboarding. If neither open
  registration nor invite links should work, the admin disables both
  (`allow_registration = false`, `allow_invite_codes = false`).
- **Invite links bypass `require_registration_approval`:** The whole point is to skip the
  approval queue for a vouched-for signup. A valid invite is the vouch; requiring further
  admin approval would negate it.
- **`allow_invite_codes` gates creation, not use:** Disabling invite codes prevents members
  from getting or regenerating links, but does not invalidate links already in circulation
  — the invitee who received a link yesterday shouldn't lose access because the admin
  flipped a toggle today.
- **One permanent multi-use link per member:** The commercial model (Dropbox, Revolut,
  Discord) — clicking "Invite others to join" returns the same link every time. No code
  list to manage, no status badges, no expiry. Abuse (link shared publicly) is handled by
  admin revocation, not by per-code complexity.
- **`InvitedByID` on `User`, not on `InviteCode`:** Tracking who invited whom on the code
  would lose the relationship as soon as the code is regenerated or the inviter deleted.
  A column on `User` is permanent and survives all code lifecycle events.
- **`allow_invite_codes = false` blocks get/regenerate, not use:** When the feature is
  disabled, members who already have a link can still see it (it still works for
  registration). Only creating or rotating a link is blocked. This mirrors the behaviour
  used for `allow_registration` vs. in-flight OTP sessions.
- **`deleteUser` and `suspendUser` revoke the member's code:** A removed or suspended
  community member should not continue bringing in new signups via an outstanding link.
  The revoke happens atomically alongside the user state change.
- **Public `GET /auth/invite/{code}` reveals the inviter's full name:** Deliberate UX
  choice (the banner is meaningless without a name), balanced by 36^8 ≈ 2.8 trillion
  combinations making guessing impractical at community scale.
- **New handler file `invite_codes.go`:** `auth.go` is already at its cognitive-complexity
  ceiling and invite-code concerns (link CRUD, admin visibility) are distinct from auth.
  Same rationale as the `unsubscribe.go` split in the monthly-digest plan.

## Implementation order

1. **Data layer** — `InviteCode` model, all three migrations, `InviteCodeRepository`
   interface + GORM impl, `PendingInviteCode` column on `RegistrationVerification`,
   `InvitedByID` on `User`, AppSettings seeded. No endpoints; tests confirm the repo layer.
2. **API** — `GET /invite-code`, `POST /invite-code/regenerate`, `GET /auth/invite/{code}`,
   admin `GET/DELETE /admin/invite-codes`. Manually testable via curl or the admin panel.
3. **Registration integration** — `send-email-otp` parks the code; `verify-email-otp`
   re-validates, bypasses the two gates, and sets `InvitedByID`.
4. **Frontend** — registration `?invite=` banner; profile "Invite a member" card;
   admin Users page invite links table.

## Housekeeping

Shipped in #85 (data layer, API, registration integration, frontend) with admin UX/
discoverability follow-ups in #93. `apps/bookshelf/TODO.md`'s "Next — approved, not yet
built" entry for this feature has been removed accordingly.
