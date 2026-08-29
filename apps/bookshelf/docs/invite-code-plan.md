# Invite-code registration — implementation plan

Companion to `invite-code-spec.md`. The spec is the behavior contract ("what and why").
This plan is the wiring against this specific codebase ("how"). If requirements shift, the spec
changes; if the codebase shifts, this plan changes.

**Prerequisite:** the spec's `Status:` header is Approved for build (it is).

## Design decisions the spec deliberately left open

### 1. Code generation: `generateInviteCode()` in `invite_codes.go`

The existing `generateOTPCode()` in `auth.go` loops over `big.NewInt(1_000_000)` for a 6-digit
numeric code. Invite codes need a wider, URL-safe alphabet:

```go
const inviteCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const inviteCodeLength   = 8

func generateInviteCode() (string, error) {
    b := make([]byte, inviteCodeLength)
    n := big.NewInt(int64(len(inviteCodeAlphabet)))
    for i := range b {
        idx, err := rand.Int(rand.Reader, n)
        if err != nil {
            return "", err
        }
        b[i] = inviteCodeAlphabet[idx.Int64()]
    }
    return string(b), nil
}
```

36^8 ≈ 2.8 trillion combinations; unguessable at community scale. Collision probability
with a table of a few hundred codes is negligible, but a `uniqueIndex` on `code` handles it
— on collision, `gorm.ErrDuplicatedKey` triggers a single retry.

### 2. One handler file: `internal/handlers/invite_codes.go`

All invite-code endpoints — member get/regenerate, public validation, admin list/revoke — live
in one file (`InviteCodeHandler`). `auth.go` and `admin.go` are both at their cognitive-complexity
ceiling. The `unsubscribeH.RegisterRoutes(api)` pattern in `main.go` already sets the precedent
for a non-`AuthHandler` owning a path that starts with `/auth/`.

`InviteCodeHandler` constructor fields:

```go
type InviteCodeHandler struct {
    inviteCodes repository.InviteCodeRepository
    admin       repository.AdminRepository
    users       repository.UserRepository
}
```

Added to `main.go` alongside the other handler constructions, then
`inviteCodeH.RegisterRoutes(api)`.

### 3. `AuthHandler` gains a nil-safe `inviteCodes` field

`sendRegisterEmailOTP` and `finalizeRegistration` live in `auth_registration.go`. To validate
invite codes there, `AuthHandler` needs a reference to `InviteCodeRepository`. Added as an
optional field:

```go
// inviteCodes is nil-safe — tests that construct AuthHandler without invite-code support
// keep working; when nil, any invite_code in the body is silently ignored.
inviteCodes repository.InviteCodeRepository
```

Threaded in via `NewAuthHandler`'s parameter list (added as the last parameter, matching the
pattern of other optional dependencies). `main.go` passes `inviteCodeRepo` as the new final
argument.

### 4. `PendingInviteCode` in `PendingRegistrationData` — single field, single migration

`PendingRegistrationData` (embedded in `RegistrationVerification`) gains one new field:

```go
PendingInviteCode string `gorm:"column:pending_invite_code" json:"-"`
```

Migration `000018_add_pending_invite_code.up.sql`:

```sql
ALTER TABLE registration_verifications ADD COLUMN pending_invite_code TEXT;
```

`RegistrationVerificationRepository.Upsert` takes `pending models.PendingRegistrationData` by
value, so the new field is picked up automatically with no signature change.

### 5. Registration bypass ordering in `sendRegisterEmailOTP` and `finalizeRegistration`

Both functions currently check `allow_registration` near the top. Invite registrations must
bypass that gate, so the order becomes:

1. If `invite_code` is present in the body and `h.inviteCodes != nil`: call
   `inviteCodes.FindByCode(code)`. If not found (revoked or unknown) → 400. If found →
   set `hasValidInvite = true`.
2. If `!hasValidInvite`: check `allow_registration` as today (403 if `"false"`).
3. Park `pending.PendingInviteCode = input.Body.InviteCode` (or `""`) with the other fields.

`finalizeRegistration` applies the same logic:

1. If `pending.PendingInviteCode != ""` and `h.inviteCodes != nil`: re-validate via
   `FindByCode` (guard against the code being revoked/regenerated during the 15-minute OTP
   window). If not found → 400.
2. Skip both `allow_registration` and `require_registration_approval` checks.
3. Create the user with `InvitedByID = &ic.InviterID` alongside the other fields.

With a multi-use permanent code, no atomic `ReserveCode` is needed — concurrent registrations
from the same link all succeed, which is the intended behavior.

### 6. `FindOrCreateByInviter` — handling the first-access race

`GET /invite-code` is idempotent: it returns the member's existing code or creates one. Two
concurrent first-access calls (rare but possible) are handled via GORM's `FirstOrCreate`:

```go
func (r *inviteCodeRepo) FindOrCreateByInviter(inviterID uint, code string) (*models.InviteCode, error) {
    ic := models.InviteCode{InviterID: inviterID}
    result := r.db.Where("inviter_id = ?", inviterID).FirstOrCreate(&ic, models.InviteCode{
        Code:      code,
        InviterID: inviterID,
    })
    return &ic, result.Error
}
```

The `uniqueIndex` on `inviter_id` means the losing concurrent INSERT returns
`gorm.ErrDuplicatedKey`; GORM's `FirstOrCreate` handles this by falling back to the SELECT,
so both callers get the same row. The `code` value passed in is only used if a new row is
created; it's generated by `generateInviteCode()` in the handler before calling the repo.

### 7. `Regenerate` — atomic revoke-and-reissue

`Regenerate(inviterID uint, newCode string) (*models.InviteCode, error)` does two things in
a single transaction:

```go
func (r *inviteCodeRepo) Regenerate(inviterID uint, newCode string) (*models.InviteCode, error) {
    var ic models.InviteCode
    err := r.db.Transaction(func(tx *gorm.DB) error {
        tx.Where("inviter_id = ?", inviterID).Delete(&models.InviteCode{})
        ic = models.InviteCode{Code: newCode, InviterID: inviterID}
        return tx.Create(&ic).Error
    })
    return &ic, err
}
```

The transaction ensures there is never a window where neither code exists. Old code is deleted
first so the uniqueIndex on `inviter_id` doesn't block the new INSERT.

### 8. `InvitedByID` on `User` — third migration, separate from the codes table

`User.InvitedByID` is a permanent record of who invited a member — it survives code
regeneration and deletion. Added to `models.go`:

```go
InvitedByID *uint `gorm:"column:invited_by_id" json:"invited_by_id,omitempty"`
```

Migration `000019_add_invited_by_id.up.sql`:

```sql
ALTER TABLE users ADD COLUMN invited_by_id INTEGER REFERENCES users(id);
```

Set in `finalizeRegistration` after `users.Create` succeeds:

```go
if usingInvite {
    user.InvitedByID = &ic.InviterID
    // best-effort update — if this fails the account still works, just lacks the attribution
    _ = h.users.Update(&user)
}
```

### 9. `appconfig.go` — one new YAML config field

`AppConfig` in `internal/handlers/appconfig.go` gets one new field:

```go
AllowInviteCodes string `yaml:"allow_invite_codes,omitempty"`
```

Added to `settingKeys`, `marshalAppConfig`, and `unmarshalAppConfig` — same mechanical
addition as each existing field.

### 10. `deleteUser` and `suspendUser` revoke the invite code

Both admin operations call `h.inviteCodes.DeleteByInviter(userID)` before (or alongside) the
user state change. `DeleteByInviter` is a no-op if the member has no code, so no guard is
needed. `AdminHandler` gains the same nil-safe `inviteCodes repository.InviteCodeRepository`
field pattern as `AuthHandler` (decision 3), wired via `NewAdminHandler`'s new final
parameter in Slice 2. The same `inviteCodeRepo` instance constructed in `main.go` is passed
to both handlers — no shared mutable state, no coupling between the two handlers.

---

## Sliced by the spec's Implementation order

Each slice is a single PR sized to land independently green through CI. Tests are written first
per the repo's TDD rule; every slice ends with `pnpm nx affected -t lint test e2e` clean.

---

### Slice 1 — Data layer + AppSettings

**Goal:** `InviteCode`, `InvitedByID`, and `PendingInviteCode` exist in the schema and are
backed by a repository. No endpoints or behavior change yet.

**Backend files touched:**

- `internal/models/models.go` — `InviteCode` struct; `InvitedByID *uint` on `User`;
  `PendingInviteCode string` on `PendingRegistrationData`.

- `internal/db/migrations/000017_create_invite_codes.{up,down}.sql`:

  ```sql
  -- up
  CREATE TABLE invite_codes (
      id         INTEGER PRIMARY KEY AUTOINCREMENT,
      code       TEXT    NOT NULL,
      inviter_id INTEGER NOT NULL REFERENCES users(id),
      created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );
  CREATE UNIQUE INDEX IF NOT EXISTS idx_invite_codes_code      ON invite_codes(code);
  CREATE UNIQUE INDEX IF NOT EXISTS idx_invite_codes_inviter_id ON invite_codes(inviter_id);
  ```

  ```sql
  -- down
  DROP TABLE IF EXISTS invite_codes;
  ```

- `internal/db/migrations/000018_add_pending_invite_code.{up,down}.sql`:

  ```sql
  -- up
  ALTER TABLE registration_verifications ADD COLUMN pending_invite_code TEXT;
  -- down: SQLite has no DROP COLUMN before 3.35.0; intentional no-op, same pattern
  -- as 000011_users_email_nocase_index.down.sql.
  ```

- `internal/db/migrations/000019_add_invited_by_id.{up,down}.sql`:

  ```sql
  -- up
  ALTER TABLE users ADD COLUMN invited_by_id INTEGER REFERENCES users(id);
  -- down: intentional no-op (same SQLite DROP COLUMN limitation).
  ```

- `internal/repository/repository.go` — new `InviteCodeRepository` interface:

  ```go
  type InviteCodeRepository interface {
      // FindOrCreateByInviter returns the member's existing code or inserts a new one.
      // code is only used if a new row is created (caller generates it first).
      FindOrCreateByInviter(inviterID uint, code string) (*models.InviteCode, error)
      FindByCode(code string) (*models.InviteCode, error)
      // Regenerate deletes the existing code and creates a new one in a single transaction.
      Regenerate(inviterID uint, newCode string) (*models.InviteCode, error)
      DeleteByInviter(inviterID uint) error
      DeleteByID(id uint) error
      ListAll() ([]models.InviteCode, error)
  }
  ```

- `internal/repository/gorm/invite_code_repo.go` — GORM implementation (see decisions 6
  and 7). `ListAll` preloads `Inviter`, orders by `created_at DESC`.

- `internal/db/db.go` `Seed()` — one new default:

  ```go
  {Key: "allow_invite_codes", Value: "true"},
  ```

- `internal/handlers/appconfig.go` — `AllowInviteCodes` field (see decision 9).

**Tests (write first):**

- `internal/repository/gorm/invite_code_repo_test.go`:
  - `FindOrCreateByInviter`: creates on first call; returns the same row on subsequent calls;
    two concurrent first calls yield the same row (unique constraint handled gracefully).
  - `FindByCode`: returns row when found; `ErrNotFound` when absent.
  - `Regenerate`: new code is returned; old code no longer findable via `FindByCode`.
  - `DeleteByInviter`: removes the row; is a no-op if no code exists.
  - `DeleteByID`: removes by primary key.
  - `ListAll`: returns all rows with `Inviter` preloaded; ordered newest first.

**Acceptance:** `pnpm nx test bookshelf-backend` green; all three migrations apply to a seeded
DB without error; `invite_codes` table, `users.invited_by_id`, and
`registration_verifications.pending_invite_code` all present with the correct schema.

---

### Slice 2 — API: member endpoints, public validation, admin endpoints

**Goal:** the full invite-code API is operational and manually testable. No registration
integration yet.

**Backend files touched:**

- `internal/handlers/invite_codes.go` (new):

  `InviteCodeHandler` + `generateInviteCode()` helper (decision 1). Endpoints:

  | Endpoint                          | Auth   | Notes                                                                                                                                                                                                                                         |
  | --------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | `GET /invite-code`                | Bearer | Checks caller eligibility (verified, non-suspended, non-pending); if `allow_invite_codes = false` and no code exists yet → 403; otherwise `FindOrCreateByInviter`; returns `{code, url}` where `url = h.email.URL("/register?invite="+code)`. |
  | `POST /invite-code/regenerate`    | Bearer | Checks eligibility; 403 if `allow_invite_codes = false`; calls `Regenerate`; returns `{code, url}`.                                                                                                                                           |
  | `GET /auth/invite/{code}`         | None   | `FindByCode`; returns `{valid: true, inviter_name}` or `{valid: false, inviter_name: ""}`.                                                                                                                                                    |
  | `GET /admin/invite-codes`         | Admin  | `ListAll`; returns the full list (no pagination — bounded by community size).                                                                                                                                                                 |
  | `DELETE /admin/invite-codes/{id}` | Admin  | `DeleteByID`; 404 if not found.                                                                                                                                                                                                               |

- `internal/handlers/admin.go` — `AdminHandler` gains a nil-safe `inviteCodes` field
  (same pattern as decision 3); `NewAdminHandler` accepts `inviteCodes repository.InviteCodeRepository`
  as a new optional final parameter; `deleteUser` and `suspendUser` each call
  `h.inviteCodes.DeleteByInviter(userID)` when the field is non-nil — a no-op if the
  member has no code, so no guard is needed.

- `cmd/server/main.go`:

  ```go
  inviteCodeRepo := gormrepo.NewInviteCodeRepository(database)
  // ...
  inviteCodeH := handlers.NewInviteCodeHandler(inviteCodeRepo, adminRepo, userRepo)
  adminH      := handlers.NewAdminHandler(..., inviteCodeRepo) // new final arg
  // ...
  inviteCodeH.RegisterRoutes(api)
  ```

**Tests (write first):**

- `internal/handlers/invite_codes_test.go`:
  - `GET /invite-code`: creates code on first call; returns same code on second call; 403
    when `allow_invite_codes = false` and no code exists; returns existing code when
    `allow_invite_codes = false` (existing holders are unaffected — creation is gated, not
    access); 400 if caller unverified / suspended / pending.
  - `POST /invite-code/regenerate`: returns a new code; old code not found via `FindByCode`;
    403 when `allow_invite_codes = false`.
  - `GET /auth/invite/{code}`: valid → `{valid:true, inviter_name}`; revoked/unknown →
    `{valid:false, inviter_name:""}`.
  - `GET /admin/invite-codes`: non-admin → 403; admin → list with inviter names.
  - `DELETE /admin/invite-codes/{id}`: non-admin → 403; found → 204; not found → 404.

- `internal/handlers/admin_test.go` (extend):
  - `deleteUser`: member's invite code is removed; subsequent `FindByCode` returns not found.
  - `suspendUser`: same assertion — code is gone after suspension.

**Acceptance:** `GET /invite-code` from a verified member returns a URL like
`https://…/register?invite=a3kf92mx`; `GET /auth/invite/{code}` returns the inviter's name;
the admin endpoint lists the code.

---

### Slice 3 — Registration integration

**Goal:** a valid invite code in `sendRegisterEmailOTP` propagates through the OTP flow,
bypasses both `allow_registration` and `require_registration_approval`, and sets
`InvitedByID` on the new user.

**Backend files touched:**

- `internal/handlers/auth.go` — `AuthHandler` struct gains the nil-safe `inviteCodes` field
  and `NewAuthHandler` gains a new final parameter (decision 3).

- `internal/handlers/auth_registration.go`:

  _`sendRegisterEmailOTPInput.Body` — new optional field:_

  ```go
  InviteCode string `json:"invite_code,omitempty" doc:"Invite code from a member's link — bypasses admin approval when valid"`
  ```

  _`sendRegisterEmailOTP` change (decision 5):_

  ```go
  hasValidInvite := false
  var pendingCode string
  if h.inviteCodes != nil && input.Body.InviteCode != "" {
      if _, err := h.inviteCodes.FindByCode(input.Body.InviteCode); err == nil {
          hasValidInvite = true
          pendingCode = input.Body.InviteCode
      } else {
          return nil, huma.Error400BadRequest("invite code is invalid or has been revoked")
      }
  }
  if !hasValidInvite {
      if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
          return nil, huma.Error403Forbidden("registration is currently disabled")
      }
  }
  // ... build pending struct ...
  pending.PendingInviteCode = pendingCode
  ```

  _`finalizeRegistration` change:_

  ```go
  usingInvite := h.inviteCodes != nil && pending.PendingInviteCode != ""

  if !usingInvite {
      if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
          return nil, huma.Error403Forbidden("registration is currently disabled")
      }
  }

  var ic *models.InviteCode
  if usingInvite {
      var err error
      ic, err = h.inviteCodes.FindByCode(pending.PendingInviteCode)
      if err != nil {
          return nil, huma.Error400BadRequest("invite link was revoked — please ask for a new one")
      }
  }

  user := models.User{
      Name:                      pending.PendingName,
      Email:                     pending.PendingEmail,
      Phone:                     pending.PendingPhone,
      Password:                  pending.PendingPasswordHash,
      Verified:                  true,
      EmailNotificationsEnabled: true,
      MonthlyDigestEnabled:      true,
  }
  if !usingInvite {
      if val, _ := h.admin.GetSetting("require_registration_approval"); val == "true" {
          user.PendingApproval = true
      }
  }
  if err := h.users.Create(&user); err != nil {
      return nil, huma.Error400BadRequest("email already registered")
  }

  if usingInvite && ic != nil {
      user.InvitedByID = &ic.InviterID
      _ = h.users.Update(&user) // best-effort; account is live regardless
  }
  ```

  Existing `OnPendingApproval` / `OnRegistered` / token-issue flow below is unchanged.

- `internal/handlers/admin.go` — the `listUsers` query gains a `.Preload("InvitedBy")`
  so the admin users table can show the "Invited by" column without a second request
  (the `InvitedBy *User` association is already declared on the model from Slice 1).

- `cmd/server/main.go` — pass `inviteCodeRepo` as the new final argument to
  `handlers.NewAuthHandler(...)`.

**Tests (write first):**

- Extend `internal/handlers/auth_registration_test.go`:
  - **Valid code, approval required:** seed `require_registration_approval = true` and an
    existing invite code; complete a registration with `invite_code` set; assert
    `User.PendingApproval == false` and `User.InvitedByID != nil`.
  - **Valid code, registration disabled:** seed `allow_registration = false` and an existing
    invite code; assert registration succeeds and `PendingApproval == false`.
  - **No code, registration disabled:** seed `allow_registration = false`, no invite code in
    body; `sendRegisterEmailOTP` returns 403.
  - **Revoked code at `send-email-otp`:** assert 400.
  - **Code revoked between send and verify:** park a code in `PendingInviteCode` then call
    `DeleteByInviter` to simulate revocation; call `verifyRegisterEmailOTP`; assert 400.
  - **No invite code:** existing tests unchanged.

**Acceptance:** manual end-to-end — with `require_registration_approval = true`, open
`/register?invite=<code>` in incognito; complete registration; confirm account is immediately
active (no pending-approval flag) and `invited_by_id` is set on the user row.

---

### Slice 4 — Frontend

**Goal:** registration detects `?invite=` and shows the inviter banner; the profile page has
a single "Invite a member" card; the admin Users page has an invite links table.

**Frontend files touched:**

_Registration page (`src/app/(auth)/register/page.tsx`):_

- On mount: read `searchParams.get("invite")`; if present call `api.validateInviteCode(code)`
  (`GET /auth/invite/{code}`). `valid: true` → show a success-variant `Badge` reading
  "Invited by [inviter\_name] — your account will be approved automatically." `valid: false`
  → show a secondary-variant notice "This invite link is no longer valid — you can still
  register normally." Form remains usable either way.
- When submitting `sendRegisterEmailOTP`, pass `invite_code` in the body if a valid code was
  found on mount.

_Profile form (`src/components/ProfileForm.tsx` or `src/app/profile/page.tsx`):_

- New "Invite a member" section.
- On load: call `GET /invite-code`. If success: show the link with a copy button and a
  "Regenerate link" button (calls `POST /invite-code/regenerate`, then refreshes).
  If 403 (feature disabled and no existing code): show "Invite links are currently disabled
  by the admin."

_Admin Users page (`src/app/admin/users/page.tsx`):_

- New "Invite links" section below the users table.
- Calls `GET /admin/invite-codes` (no pagination).
- Columns: Member, Link, Created. "Revoke" button per row → `DELETE /admin/invite-codes/{id}`
  → refetch.
- Existing users table gains an "Invited by" column: when a `User` row has `invited_by_id`
  set, show the inviter's name (the `GET /admin/users` response already includes full `User`
  structs — preload `InvitedBy` in the users query so the name is available without a
  second request). Rows where `invited_by_id` is null show an em-dash.

_New API client methods (`src/lib/api.ts`):_

```ts
getInviteCode(): Promise<{ code: string; url: string }>
regenerateInviteCode(): Promise<{ code: string; url: string }>
validateInviteCode(code: string): Promise<{ valid: boolean; inviter_name: string }>
listAdminInviteCodes(): Promise<InviteCode[]>
revokeAdminInviteCode(id: number): Promise<void>
```

_New/updated types (`src/lib/types.ts`):_

```ts
interface InviteCode {
  id: number;
  code: string;
  inviter_id: number;
  inviter?: User;
  created_at: string;
  url?: string; // only in GET /invite-code and POST /invite-code/regenerate responses
}

// User type gains:
//   invited_by_id?: number;
//   invited_by?: User;   // preloaded by GET /admin/users
```

**Tests (write first):**

- `apps/bookshelf-e2e`:
  - **Full invite flow:** logged-in verified member opens profile → copies invite link →
    opens it in a new browser context → "Invited by" banner appears → completes
    registration → `GET /auth/me` shows `pending_approval: false` even with
    `require_registration_approval: true` seeded.
  - **Regenerate:** member clicks "Regenerate link" → old URL shows "no longer valid" notice;
    new URL shows the banner again.
  - **Admin revoke:** admin revokes a code from the Users page → that URL returns
    `{valid: false}`; the member's profile reloads and `GET /invite-code` lazily creates a
    fresh code.
  - **Multi-use:** two different invitees register via the same link → both accounts active,
    both have `invited_by_id` set.

**Acceptance:** full invite flow works against real servers (mailhog for email verification);
admin can see and revoke links; profile card shows the correct link on every load.

---

## Cross-cutting notes

- **Migrations.** Slice 1 adds three migrations. The down migrations for `000018` and `000019`
  are intentional no-ops — SQLite before 3.35.0 doesn't support `DROP COLUMN`. Document in
  the files with a comment, matching the existing pattern in
  `000011_users_email_nocase_index.down.sql`.
- **CHANGELOG entries for each slice.** `nx release` picks these up on merge. All slices are
  member-visible or admin-visible — use `feat` throughout.
- **`TODO.md` housekeeping.** Remove the "Invite-code registration" entry from "Next —
  approved, not yet built" in Slice 4's PR once the feature is fully shipped.
- **`/register?invite=` and `?verifyToken=` coexist.** The magic-link token for email
  verification also uses a query param. Both can appear in the same URL and the registration
  page already reads them independently — no conflict.
- **`deleteUser` and `suspendUser` wiring (decision 10).** Placed in Slice 2 alongside the
  other `inviteCodeRepo` wiring. The repo instance is constructed once in `main.go` and
  passed to both `NewInviteCodeHandler` and `NewAdminHandler` — no coupling between the two
  handlers.
