# Magic-Link Registration — spec

**Status:** Approved for build · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** the existing `/register` wizard (`details` → `verify-email` → `verify-phone`),
`AuthHandler`, `RegistrationVerificationRepository`

Make the emailed "Verify email" link a real magic link: clicking it (from any device, not just
the tab that started registration) finishes signup and signs the user in, instead of dropping them
back on a blank Name/Password form. While rebuilding this wizard's finalize logic, also drop the
phone-OTP step it currently forces every phone-providing registrant through — it doesn't verify
anything today, so it's pure friction with no offsetting trust benefit.

## Why now

Today, `sendRegisterEmailOTP` (`auth.go:758`) emails both a 6-digit code and a "Verify email"
button (`/register?verifyToken=<token>`), but the token only carries `{purpose, identifier: email,
code}` (`otp_link_token.go:32`) — never the Name/Password the user already typed on the `details`
step, because those only ever live in `register/page.tsx`'s React state. Clicking the link reloads
`/register` fresh: Email comes back pre-filled and marked verified, but Name and Password are
gone, and the user has to retype them and hit Continue again before an account (and session) is
even created (`register()`, `auth.go:674`, is the only place `issueToken` gets called for this
flow). Opened on a different device — the normal way people read email — the retype is unavoidable
and the "magic link" has saved nothing but 6 keystrokes. That mismatch between what a magic link
implies (one click, you're in) and what this one delivers (one click, please refill this form) is
the actual complaint.

Separately, while tracing this flow: the wizard's `verify-phone` step (`register/page.tsx:525`)
asks for a phone number and "verifies" it with a 6-digit code — but no SMS provider is configured
(`services.MockSMSService`), so that code is returned directly in the API response to whoever is
registering (`sendRegisterPhoneOTPOutput.MockCode`, `auth.go:329`), not texted anywhere. It doesn't
prove phone ownership; it just adds a screen. The real, already-existing borrow-time enforcement
(`checkPhoneRequirement`, `loan_requests.go:505`) only ever checked `Phone != ""`, never
`PhoneVerified` — so the backend has already implicitly settled on "phone on file," not "phone
verified," as what actually matters. Since fixing the magic link means rewriting this exact
finalize logic anyway, this is the natural point to stop pretending the OTP step proves anything,
rather than shipping it once more and ripping it out immediately after.

## Goals

- Clicking the emailed link, from any device or tab, never requires retyping Name or Password — it
  completes registration and signs the user in directly, every time.
- Typing the 6-digit code manually, in the same tab, produces the identical outcome as clicking
  the link — today it doesn't (see Open decision 1).
- No password material — plaintext or hash — ever travels through the email/URL channel. Only the
  existing OTP `code` does, same as every other flow in this file (reset, email-change).
- Registration is a single verification step (email) for every account — not two, and not
  conditionally two depending on whether a phone was entered.
- Phone becomes a plain optional field on the `details` step: collected (and, when the admin
  requires a phone to borrow, called out as needed for that later), never gated behind an OTP,
  never blocking account creation.

## Non-goals (v1)

- No change to the password-reset magic link (`/forgot-password?resetToken=`) — it already matches
  this shape: no name typed, only a new password, which is legitimate for a reset rather than a
  UX smell.
- No move off bearer-JWT-in-`localStorage` sessions to cookies — orthogonal.
- No change to `require_verified_to_borrow` (email) or `verification_min_books_shared` — untouched.
- No real SMS provider added. If phone-as-a-verified-trust-signal is wanted later, that's a
  prerequisite for reintroducing an OTP step, not something this spec does instead of it.
- `POST /auth/register` is removed, not deprecated-in-place. Once verify-email-otp finalizes the
  account itself, nothing calls it — keeping an unused endpoint around isn't a smaller change than
  deleting it.
- Not deleting the backend's phone-OTP machinery (`sendRegisterPhoneOTP`/`verifyRegisterPhoneOTP`
  handlers, routes, `MockSMSService`, `phoneOTPLimiter`) — the frontend stops calling any of it,
  but removing it cleanly is a larger, separate cleanup (see "Backend tech debt" below), not a
  frontend concern.

## How it works

**Design decision, stated up front:** the pending Name/Password/Phone move server-side at the
moment "Continue" is clicked on the `details` step — stored keyed by email, the same lifecycle
(15-minute TTL, single-use, deleted on verification) as the OTP code sitting next to them — rather
than being embedded in the magic-link token itself. The magic-link JWT embedded in the email URL
stays exactly as narrow as it is today (`{purpose, identifier, code}`); it's the one piece of this
system that transits an inherently insecure channel (email, URLs, browser history, referrer
headers, corporate link-scanners), so it stays minimal on purpose.

1. `details` step submit (`handleDetailsSubmit`) sends Name/Email/Password/Phone together to
   `POST /auth/register/send-email-otp`, not just Email. Phone is always optional at this layer
   now — the `require_phone` admin setting only changes the field's helper copy ("you'll need a
   phone number on file to borrow books"), not whether submission is blocked. The handler
   validates password complexity and email-not-already-registered exactly like today's
   `register()` does, then bcrypt-hashes the password once and stores
   `{Name, PasswordHash, Phone}` alongside the OTP `code`/`expires_at` on the existing
   `registration_verifications` row (`channel="email"`), via an extended `Upsert`. A resend
   (`handleResendEmailOTP`) re-submits the same fields, overwriting the pending row exactly as a
   resend overwrites the code today.
2. The email still contains the 6-digit code and the "Verify email" link, unchanged in shape.
3. Either path — typing the code (`handleVerifyEmailSubmit`) or clicking the link (the `useEffect`
   reading `?verifyToken=`) — hits the same `POST /auth/register/verify-email-otp`. On success it
   reads the pending row it just validated the code against and finalizes immediately: creates the
   `User` (reusing the already-computed password hash, no second bcrypt call; `Phone` stored as
   given, `PhoneVerified` always `false`), issues a session token (unless
   `require_registration_approval` applies), deletes the pending row, and returns
   `{status: "complete", token, user}` or `{status: "pending_approval", user}`. The frontend stores
   the token and redirects to `/catalog`, or shows the pending-approval message — never a return to
   `/register`.

No third branch, no hand-off token, no separate phone step — registration is done in one
verification round trip regardless of whether a phone was entered.

## API surface

| Change                                  | Where                                                                      | Behavior                                                                                                                                                                                                                                                                                                                                                                                       |
| --------------------------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Extended request                        | `POST /auth/register/send-email-otp`                                       | Body gains required `name`, `password`; optional `phone`. Validates and bcrypt-hashes password up front; stores both alongside the OTP code.                                                                                                                                                                                                                                                   |
| New dev-only field                      | `sendRegisterEmailOTPOutput`                                               | `debug_verify_link`, mirroring `forgotPasswordOutput.DebugResetLink` — the exact magic-link URL, for local dev without SMTP and for e2e coverage (none exists for this link today).                                                                                                                                                                                                            |
| Response becomes a discriminated result | `POST /auth/register/verify-email-otp`                                     | `{status: "complete", token, user}` \| `{status: "pending_approval", user}`, replacing today's bare `{verification_token, email}`. Always one of these two — no continuation branch.                                                                                                                                                                                                           |
| Removed                                 | `POST /auth/register`                                                      | Deleted — nothing calls it once verify-email-otp finalizes directly.                                                                                                                                                                                                                                                                                                                           |
| Removed                                 | `POST /auth/register/send-phone-otp`, `/verify-phone-otp` (frontend usage) | Frontend stops calling either — the `verify-phone` step is deleted. Endpoints themselves stay on the backend, unreachable from the app; see "Backend tech debt."                                                                                                                                                                                                                               |
| Corrected                               | `phoneOnFileFactor` (`auth.go:1606`)                                       | Label changes from "Verified phone number" to "Phone number on file"; `Satisfied` switches from `user.PhoneVerified` to `user.Phone != ""` — matching what `checkPhoneRequirement` already enforces. This one's not optional: once nothing ever sets `PhoneVerified` true again, leaving this check as-is would make the Profile checklist permanently unsatisfiable for every new registrant. |

## Open decisions

**Decision 1 — same-tab code entry gets the same outcome as the link.** Today, typing the 6-digit
code already auto-proceeds without a second click — only the magic-link path regresses to a blank
form. This spec makes both paths call the identical `verify-email-otp` logic, so there's no
special-casing to design here; it falls out of unifying the two entry points on one backend
response shape.

**Decision 2 — stale magic link after editing details.** If a user clicks Continue, then goes back
and edits Name/Password/Phone (triggering a resend), then clicks the _original_ email's link, the
token's embedded `code` no longer matches the just-overwritten `registration_verifications` row
(`Upsert` replaces, doesn't accumulate) — same "invalid verification code" rejection as today.
Unchanged, not a new edge case this design introduces.

**Decision 3 — bcrypt moves earlier.** Hashing now happens at `send-email-otp` time
(~150–300ms at cost 12) rather than at the old `register()` call. This endpoint is already
identifier-rate-limited (3 requests/5min, `registrationOTPRateLimitAttempts`) and already rejects
duplicate emails and bad passwords before doing any hashing, so this isn't a new abuse surface —
just moves an unavoidable cost earlier in the flow.

**Decision 4 — ambiguous concurrent submissions for one email.** Two tabs submitting different
Name/Password for the same not-yet-registered email race on the same `Upsert`; last write wins,
identical to how the OTP `code` itself already behaves under concurrent resends today. Not worth
extra locking for a self-hosted, single-community app.

**Decision 5 — existing users' `PhoneVerified=true` rows.** Left untouched. Nothing reads
`PhoneVerified` for enforcement after the `phoneOnFileFactor` fix above, so historical `true`
values become inert rather than incorrect — no backfill/migration needed.

## Settled during build

Four things this spec left implicit or self-contradictory, resolved while implementing it:

**The phone-OTP endpoints stay fully working.** The non-goals said "not deleting the backend's
phone-OTP machinery" while Build order step 2 said to delete
`registrationVerificationClaims`/`issueRegistrationVerificationToken`/`registrationPurposePhone` —
which is exactly what `verifyRegisterPhoneOTP` needs to produce a response. Kept working, on the
call that a real SMS provider may make them worth reviving; the tech-debt note now lives on
`sendRegisterPhoneOTP` in `auth.go`, listing what a clean removal would take. Only the genuinely
unreachable pieces went: `verifyRegistrationVerificationToken` and `registrationPurposeEmail`
(nothing validates either kind of token now), plus `register()`/`registerInput`/`resolveVerifiedPhone`.

**A fourth pending column, `pending_email`.** The row is keyed by the _normalized_ (lowercased)
email so the magic-link token can address it, but `users.email` is a case-sensitive lookup
(`FindByEmail`) — creating the account from the normalized form would leave anyone who typed
`John.Doe@…` unable to log in with what they typed. `pending_email` carries the casing as typed;
the other three columns are as specced.

**`allow_registration` is checked twice.** The spec's finalize helper had no home for the
`allow_registration` guard `register()` used to own. It's checked in `sendRegisterEmailOTP` (fail
fast, clear error) and again in `finalizeRegistration` (authoritative — an admin can flip the
setting during the 15 minutes a code is live).

**`registerLimiter` moved to `verify-email-otp` and was resized.** Account creation is what it
guarded, so it follows account creation to its new endpoint. Its old sizing (5 burst, 1 per 10
min) didn't survive the move: `ClientIP` degrades to a single shared bucket behind the frontend
proxy, so five signups per ~50 minutes is a community-wide cap — the e2e suite tripped it
immediately, and a real onboarding session would too. Now 20 burst, 1 per 30s. Reaching this
endpoint already requires a code delivered to an inbox you control, itself capped per address by
`registrationOTPRateLimitAttempts`.

## Backend tech debt (flagged, not addressed here)

Once the frontend stops calling them, these become dead code reachable only by direct API call,
not by anything in the app itself:

- `sendRegisterPhoneOTP`/`verifyRegisterPhoneOTP` handlers (`auth.go:813`, `auth.go:836`) and their
  routes (`/auth/register/send-phone-otp`, `/auth/register/verify-phone-otp`).
- `phoneOTPLimiter` (`auth.go:236`), `registrationChannelPhone` (`auth.go:90`).
- `services.SMSService`/`services.MockSMSService` (`internal/services/sms.go`) — nothing else in
  the codebase implements or calls this interface.
- `User.PhoneVerified` (`models.go:14`) and `applyPhoneUpdate`'s clearing of it (`auth.go:1159`) —
  the field itself, not just the OTP flow that used to set it true.

Left in place rather than deleted in this pass: removing them cleanly also means deciding whether
`PhoneVerified` disappears from the schema/API entirely (a migration, plus checking nothing else
reads `phone_verified` off the wire) or stays dormant for a future real-SMS integration to revive.
That's a genuinely separate decision from "stop asking users to sit through a fake verification,"
which is all this spec does. Worth revisiting either when a real SMS provider gets wired up
(verification becomes real, worth keeping and fixing) or as a deliberate dead-code removal pass.

## Build order

1. Backend: migration adding nullable `pending_name`, `pending_password_hash`, `pending_phone` to
   `registration_verifications`; `RegistrationVerificationRepository.Upsert` extended to accept
   them.
2. Backend: `AuthHandler` — extend `sendRegisterEmailOTPInput`/hash-and-store, add
   `debug_verify_link`; rewrite `verifyRegisterEmailOTP` to always finalize (extract a shared
   `finalizeRegistration(ctx, name, email, passwordHash, phone)` helper from today's `register()`
   body — creation, approval branch, `issueToken`, no `phoneVerified` parameter since it's always
   `false` now); delete `register()`, `registerInput`, its route, `resolveVerifiedPhone`,
   `registrationVerificationClaims`, `issueRegistrationVerificationToken`,
   `verifyRegistrationVerificationToken`, `registrationPurposeEmail`/`registrationPurposePhone`.
   Fix `phoneOnFileFactor`'s label/check per the API surface table.
3. Backend tests: cover finalize-on-verify-email (with and without a phone present), the corrected
   `phoneOnFileFactor` behavior. Remove/replace whatever `auth_test.go` coverage targeted the
   deleted `register()` handler and the old `verify-phone-otp`-in-registration path.
4. Frontend: `src/lib/api.ts` — new request/response types for `sendRegisterEmailOTP`/
   `verifyRegisterEmailOTP`; delete `sendRegisterPhoneOTP`/`verifyRegisterPhoneOTP` client
   functions (nothing calls them). `register/page.tsx` collapses to a two-step wizard
   (`details` → `verify-email`): delete the `verify-phone` step, `phoneOtpCode`/`phoneMockCode`/
   `fullPhone`/`sendingPhoneOtp`/`verifyingPhoneOtp` state, `handleVerifyPhoneSubmit`,
   `handleResendPhoneOTP`, `handleSkipPhone`, and `proceedAfterEmailVerified` (its
   phone-vs-no-phone branch no longer exists — `handleDetailsSubmit`/`handleVerifyEmailSubmit`/the
   magic-link effect all just call verify-email-otp and handle `complete`/`pending_approval`
   directly). `toFullPhone` stays — phone formatting is still needed for the plain field.
5. Frontend: `ProfileForm.tsx`'s verification-factor rendering (`~line 870`) picks up the
   `phoneOnFileFactor` label/copy change automatically once the backend field changes — spot-check
   the "Add your phone number in the [profile]..." hint text still reads correctly against the new
   label.
6. E2E: update `auth-helpers.ts`'s `registerTestUser` to the shorter two-call sequence (no more
   `POST /auth/register`, no phone-OTP calls). Add `register-magic-link.spec.ts` mirroring
   `password-reset-magic-link.spec.ts` — register via API, pull `debug_verify_link`, `page.goto()`
   it on a clean context, assert landing straight on `/catalog` already signed in (no form).
   Rewrite `register-phone-requirement.spec.ts`: its current premise (phone required ⇒ submission
   blocked) no longer holds, since phone never blocks registration now — replace its assertions
   with "the field's helper copy reflects the setting, and submission succeeds either way."

## How we'll know it's working

The literal case the user described — click the emailed link on your phone after starting signup
on a laptop — is exactly what `register-magic-link.spec.ts`'s `page.goto(debug_verify_link)` on a
fresh context reproduces (no cookies/state carried over, same as a different device). If that spec
passes and lands the user on `/catalog` already authenticated, the reported problem is fixed.
Separately: registering with `verification_requires_phone` on no longer shows a blocking error or
an OTP step — signup completes in one step regardless, and the phone requirement only surfaces
later, at the moment it's actually enforced (attempting to borrow).

## Implementation notes

**Backend** (`apps/bookshelf-backend`):

- `internal/db/migrations/000010_add_pending_registration.{up,down}.sql`: add
  `pending_name TEXT`, `pending_password_hash TEXT`, `pending_phone TEXT` to
  `registration_verifications`.
- `internal/models/models.go`: `RegistrationVerification` gains the three matching fields.
- `internal/repository/repository.go` / `gorm/registration_verification_repo.go`: `Upsert` gains a
  `pending models.PendingRegistrationData` (or equivalent) parameter.
- `internal/handlers/auth.go`:
  - `sendRegisterEmailOTPInput.Body` gains `Name`, `Password`, `Phone *string`; `sendRegisterEmailOTP`
    validates (`validatePasswordComplexity`, dup-email check — both already exist, just called
    earlier) then `bcrypt.GenerateFromPassword` before the `Upsert` call; `sendRegisterEmailOTPOutput`
    gains `DebugVerifyLink` alongside `DebugCode`.
  - `verifyRegisterEmailOTP` rewritten around the existing `resolveEmailAndCode` +
    `checkRegistrationOTP`-style validation, then always calls the new `finalizeRegistration`.
  - `phoneOnFileFactor`: `Label` → `"Phone number on file"`, `Satisfied` → `user.Phone != ""`.
  - Remove `register()`, `registerInput`, its route registration, `resolveVerifiedPhone`,
    `registrationVerificationClaims`, `issueRegistrationVerificationToken`,
    `verifyRegistrationVerificationToken`, `registrationPurposeEmail`/`registrationPurposePhone`.
- `internal/handlers/auth_test.go`: per Build order step 3.

**Frontend** (`apps/bookshelf`):

- `src/lib/api.ts`: type changes per API surface table above; delete the two phone-OTP client
  functions.
- `src/app/(auth)/register/page.tsx`: per Build order steps 4–5. The `verifyingLinkToken`/
  "✓ Email verified" banner on the `details` step becomes dead code too — a successful link click
  now navigates away from `/register` entirely (to `/catalog` or `/login`) rather than returning
  to this step — remove it along with the rest.

### Critical files

- `internal/handlers/auth.go`, `internal/handlers/auth_test.go`
- `internal/handlers/otp_link_token.go` (unchanged, referenced for context)
- `internal/repository/repository.go`, `internal/repository/gorm/registration_verification_repo.go`
- `internal/models/models.go`
- `src/lib/api.ts`, `src/app/(auth)/register/page.tsx`, `src/components/ProfileForm.tsx`
- `apps/bookshelf-e2e/src/auth-helpers.ts`, `register-phone-requirement.spec.ts`, new
  `register-magic-link.spec.ts`

### Verification

- Backend: `pnpm nx test bookshelf-backend`.
- Frontend: manual via `pnpm nx dev bookshelf` against a local backend in `ENV=dev` (no SMTP
  needed — `debug_code`/`debug_verify_link` are both returned).
- End-to-end: `pnpm nx e2e bookshelf-e2e`, specifically the new `register-magic-link.spec.ts`, the
  rewritten `register-phone-requirement.spec.ts`, and `login.spec.ts` for regressions.
- Full gate: `pnpm nx affected -t lint test e2e` before merging (this touches `bookshelf`,
  `bookshelf-backend`, and `bookshelf-e2e`, so `e2e` is affected too).
