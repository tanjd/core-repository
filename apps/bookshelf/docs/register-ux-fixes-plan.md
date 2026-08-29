# Registration UX fixes — implementation plan

**Status:** Approved for build · **Scope:** `apps/bookshelf` (frontend only) ·
**Depends on:** `PasswordStrengthMeter`, `Input`, `register/page.tsx`,
`forgot-password/page.tsx`, `ProfileForm.tsx`

Four friction points surfaced from user feedback on the registration flow. All four are
frontend-only fixes. No backend changes, no new API endpoints, no migrations.

---

## Fix 1 — No password visibility toggle

**Priority: high · Affects: all password fields site-wide**

### Problem

Both the password and confirm-password fields on the registration form (and every other
password field in the app — forgot-password, profile change-password, login, setup) are
permanently masked with no way to reveal what was typed. On mobile, where keyboard accuracy is
lower, a typo in either field has no escape hatch except clearing and retyping blind.

### Design decision: extract a `PasswordInput` component

Rather than adding `useState` + eye-icon markup at each of the six call sites that use
`type="password"` today, extract a single `src/components/PasswordInput.tsx` that wraps
`Input` in a relative container and overlays a `Ghost` `Button` with `Eye`/`EyeOff` icons from
`lucide-react`. Every call site swaps `<Input type="password" ...>` for
`<PasswordInput ...>` with no other changes — the visible/masked state is fully internal.

```tsx
// src/components/PasswordInput.tsx
"use client";

import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export function PasswordInput({
  className,
  ...props
}: React.ComponentProps<typeof Input>) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="relative">
      <Input
        {...props}
        type={visible ? "text" : "password"}
        className={cn("pr-10", className)}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon"
        tabIndex={-1}
        aria-label={visible ? "Hide password" : "Show password"}
        className="absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground"
        onClick={() => setVisible((v) => !v)}
      >
        {visible ? (
          <EyeOff className="size-4" aria-hidden />
        ) : (
          <Eye className="size-4" aria-hidden />
        )}
      </Button>
    </div>
  );
}
```

`tabIndex={-1}` keeps the eye button out of the tab order — the input itself is the
interactive element; the toggle is just a convenience affordance.

### Files touched

| File                                      | Change                                                                                              |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `src/components/PasswordInput.tsx`        | New component (above)                                                                               |
| `src/app/(auth)/register/page.tsx`        | Replace both `<Input type="password" ...>` with `<PasswordInput>`                                   |
| `src/app/(auth)/forgot-password/page.tsx` | Replace both `<Input type="password" ...>` in the reset step                                        |
| `src/app/(auth)/login/page.tsx`           | Replace the single `<Input type="password" ...>`                                                    |
| `src/app/setup/page.tsx`                  | Replace both `<Input type="password" ...>`                                                          |
| `src/components/ProfileForm.tsx`          | Replace all four `<Input type="password" ...>` (current, new, confirm new, and the admin-setup one) |

### Tests (write first)

**Unit (`src/components/PasswordInput.test.tsx`):**

- Renders an `input` with `type="password"` by default.
- Clicking the toggle button switches `type` to `"text"`.
- Clicking again switches back to `"password"`.
- `aria-label` is `"Show password"` when masked, `"Hide password"` when visible.
- Additional props (e.g. `placeholder`, `value`, `onChange`) are forwarded to the underlying
  `Input`.

---

## Fix 2 — Confirm-password mismatch only caught on submit

**Priority: high · Affects: register, forgot-password, ProfileForm change-password**

### Problem

`PasswordStrengthMeter` already gives live feedback on the primary password field. Its paired
confirm field gets nothing until the user clicks Continue and gets bounced back with a prose
error at the bottom of the form. The investment is the same as what was already made for the
strength meter — one more live indicator.

### Design decision: extract a `PasswordMatchIndicator` component

A tiny render-only component shown below the confirm field. Renders nothing when `confirm` is
empty (same "don't show red until the user has engaged" contract as `PasswordStrengthMeter`).
Once the user starts typing, it shows either a green "Matches" or a muted "Doesn't match yet"
— not red, to avoid a wall of red while they're still mid-typing.

```tsx
// src/components/PasswordMatchIndicator.tsx
import { Check, X } from "lucide-react";
import { cn } from "@/lib/utils";

export function PasswordMatchIndicator({
  password,
  confirm,
}: {
  password: string;
  confirm: string;
}) {
  if (!confirm) return null;

  const matches = password === confirm;
  return (
    <p
      className={cn(
        "flex items-center gap-1.5 text-xs",
        matches ? "text-success" : "text-muted-foreground",
      )}
    >
      {matches ? (
        <Check className="size-3.5 shrink-0" aria-hidden />
      ) : (
        <X className="size-3.5 shrink-0" aria-hidden />
      )}
      {matches ? "Matches" : "Doesn't match yet"}
    </p>
  );
}
```

"Doesn't match yet" (muted, not red) avoids false alarm styling while the user is still
typing — it turns green as soon as the two align. Red is reserved for a definitive error state
(see Fix 4 below on removing the submit-time prose duplicate).

### Files touched

| File                                        | Change                                                                                                 |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `src/components/PasswordMatchIndicator.tsx` | New component (above)                                                                                  |
| `src/app/(auth)/register/page.tsx`          | Add `<PasswordMatchIndicator password={password} confirm={confirmPassword} />` below the confirm input |
| `src/app/(auth)/forgot-password/page.tsx`   | Same, below confirm-new-password                                                                       |
| `src/components/ProfileForm.tsx`            | Same, below confirm-new-password in the change-password section                                        |

### Tests (write first)

**Unit (`src/components/PasswordMatchIndicator.test.tsx`):**

- Renders nothing when `confirm` is empty.
- Renders "Matches" with a `Check` icon when `password === confirm` (and confirm is non-empty).
- Renders "Doesn't match yet" with an `X` icon when they differ.
- `text-success` class applied when matching; `text-muted-foreground` when not.

---

## Fix 3 — No resend cooldown on the verification step

**Priority: medium · Affects: `register/page.tsx` only**

### Problem

"Resend code" on the verify-email step has no debounce or countdown. A user who doesn't see
the email within a few seconds is likely to tap it multiple times, which either floods their
inbox or trips the backend rate limit silently.

### Design decision: 30-second countdown, started immediately on entering the verify step

State: `resendCooldown: number` (seconds remaining). Initialized to `0`. Wired via a
`useEffect` cascade that fires once-per-second while non-zero, so no `setInterval` leak.

Start the cooldown in two places:

1. Right after `setStep("verify-email")` in `handleDetailsSubmit` — the code was just sent.
2. Right after `await submitDetails()` succeeds in `handleResendEmailOTP` — another was just
   sent.

Button label when cooling down: `"Resend in 0:30"` counting down to `"Resend in 0:01"`, then
re-enabled as `"Resend code"`. `String(resendCooldown).padStart(2, "0")` formats the seconds.

```tsx
// State addition to register/page.tsx
const [resendCooldown, setResendCooldown] = useState(0);

useEffect(() => {
  if (resendCooldown <= 0) return;
  const id = setTimeout(() => setResendCooldown((s) => s - 1), 1000);
  return () => clearTimeout(id);
}, [resendCooldown]);

function startResendCooldown() {
  setResendCooldown(30);
}
```

Wired at the two call sites:

```tsx
// in handleDetailsSubmit, success branch:
setStep("verify-email");
startResendCooldown(); // ← add

// in handleResendEmailOTP, success branch:
await submitDetails();
toast.success("Verification code sent");
startResendCooldown(); // ← add
```

Button:

```tsx
<button
  type="button"
  className="text-primary hover:underline disabled:opacity-50 disabled:cursor-not-allowed"
  disabled={sendingEmailOtp || resendCooldown > 0}
  onClick={handleResendEmailOTP}
>
  {resendCooldown > 0
    ? `Resend in 0:${String(resendCooldown).padStart(2, "0")}`
    : sendingEmailOtp
      ? "Sending…"
      : "Resend code"}
</button>
```

### Files touched

| File                               | Change                                                                                                                                                       |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `src/app/(auth)/register/page.tsx` | Add `resendCooldown` state + `useEffect` tick + `startResendCooldown`; update `handleDetailsSubmit` and `handleResendEmailOTP`; update button label/disabled |

### Tests (write first)

**Unit (`apps/bookshelf/src/app/(auth)/register/page.test.tsx` — extend or create):**

- After moving to the verify-email step, the "Resend code" button is disabled immediately.
- The button label reads `"Resend in 0:30"`.
- After advancing fake timers by 30 s, the button is re-enabled and reads `"Resend code"`.
- Clicking "Resend code" (when enabled) and having it succeed re-disables the button and
  restarts the countdown at 30.

**E2E (`apps/bookshelf-e2e/src/register-resend-cooldown.spec.ts`):**

- Start registration, reach the verify-email step.
- Assert the "Resend code" button is disabled immediately.
- Assert it contains countdown text ("Resend in 0:").
- After 30 s (or stub time via `page.clock.install`), assert the button is re-enabled.

> Note: the E2E uses Playwright's `page.clock` API to control time rather than waiting a real
> 30 s in CI. `page.clock.install({ time: Date.now() })` + `page.clock.tick(30_000)` is the
> pattern — no `waitForTimeout` required.

---

## Fix 4 — Submit-time prose error duplicates the strength-meter checklist

**Priority: medium · Affects: `register/page.tsx`, `forgot-password/page.tsx`,
`ProfileForm.tsx`**

### Problem

When `validatePassword` fails on submit, `setError(passwordError)` displays a red line at the
bottom of the form restating a requirement the `PasswordStrengthMeter` checklist already marks
with a red `X` above it. Two sources of truth for "what's wrong with your password" is harder
to scan than one. The fix is to let the checklist be the single source of truth and suppress the
bottom prose error.

### Design decision: stop calling `setError` for password-rule failures; add "not a common password" to the checklist

Two sub-problems to solve:

**4a.** All `validatePassword` rule failures (`< 12 chars`, `no uppercase`, `no lowercase`,
`no digit`, `name/email contains`) are already shown by the checklist's four `X`-marked
requirements. When the meter is visible (i.e. when `password` is non-empty — the condition
under which a submit-time failure can fire), simply `return` early without calling
`setError`.

**4b.** The checklist does _not_ cover `validatePassword`'s fifth rule: "too common password"
(`COMMON_PASSWORDS` set). If that check fails and we suppress the prose error, the user sees
all four checklist items green but submit still bounces — a confusing ghost failure. Fix: add a
fifth item to `requirementsFor` in `PasswordStrengthMeter`:

```ts
// In api.ts — add export to the existing helper:
export function isCommonPassword(password: string): boolean {
  return COMMON_PASSWORDS.has(password.toLowerCase());
}
```

```tsx
// In PasswordStrengthMeter.tsx — add fifth requirement:
{
  label: "Not a commonly used password",
  met: !isCommonPassword(password),
}
```

Import `isCommonPassword` from `@/lib/api`. This matches the existing pattern of importing
`MIN_PASSWORD_LENGTH` from there.

With both sub-problems resolved, `handleDetailsSubmit` in `register/page.tsx` becomes:

```tsx
const passwordError = validatePassword(password, [name, emailLocalPart(email)]);
if (passwordError) {
  // Checklist already shows which requirement is unmet — no prose error needed.
  return;
}
if (password !== confirmPassword) {
  // Live indicator (Fix 2) already shows the mismatch; just block submission.
  return;
}
```

The `setError` calls are removed for both cases. `error` state (and its render) remains for
backend errors (e.g. "email already registered") — those aren't shown anywhere else.

Apply the same suppression in `forgot-password/page.tsx`'s `handleResetSubmit` and
`ProfileForm.tsx`'s `handleChangePassword`: wherever a `validatePassword` check currently
calls `setError`, replace it with a bare `return`.

### Files touched

| File                                       | Change                                                                                        |
| ------------------------------------------ | --------------------------------------------------------------------------------------------- |
| `src/lib/api.ts`                           | Export new `isCommonPassword(password: string): boolean` helper                               |
| `src/components/PasswordStrengthMeter.tsx` | Add fifth `requirementsFor` item using `isCommonPassword`                                     |
| `src/app/(auth)/register/page.tsx`         | Remove `setError(passwordError)` and `setError("Passwords do not match")` from submit handler |
| `src/app/(auth)/forgot-password/page.tsx`  | Same — remove password-rule `setError` from reset submit                                      |
| `src/components/ProfileForm.tsx`           | Same — remove password-rule `setError` from change-password submit                            |

### Tests (write first)

**Unit (`src/lib/api.test.ts` — extend):**

- `isCommonPassword("password")` → `true`.
- `isCommonPassword("Ux7$kQp9mNv2")` → `false`.

**Unit (`src/components/PasswordStrengthMeter.test.tsx` — extend):**

- A common password (`"password"`) renders a red-X "Not a commonly used password" requirement.
- A non-common password that meets all other requirements renders it green.

**Unit (`src/app/(auth)/register/page.test.tsx` — extend):**

- Submitting with a too-short password does not render a prose error; the form stays on the
  details step.
- Submitting with a common password does not render a prose error; the form stays on the
  details step.
- A genuine backend error (mocked `api.sendRegisterEmailOTP` rejecting) still renders the
  prose error (the `error` state is still used for that path).

---

## Build order

1. **Fix 1** (PasswordInput) — extract the component, replace all call sites. Pure
   refactor + new component; zero behavior change. Easiest to review in isolation.
2. **Fix 2** (PasswordMatchIndicator) — new component, drop into three forms. No interaction
   with Fix 1.
3. **Fix 3** (resend cooldown) — isolated to `register/page.tsx`. No interaction with Fixes 1–2.
4. **Fix 4** (suppress duplicate error) — depends on Fix 2 being in first (otherwise removing
   the mismatch `setError` leaves no live feedback for the user). Depends on Fix 1 only
   incidentally (both touch the same file). Do last.

Each fix ships as its own PR — all four touch frontend only, CI green requires
`pnpm nx affected -t lint test e2e`.

---

## Cross-cutting notes

- **`PasswordInput` and the `Input` API contract.** `PasswordInput` uses
  `React.ComponentProps<typeof Input>` and spreads `...props` onto `Input`, so it accepts
  every prop `Input` does (`id`, `autoComplete`, `required`, `value`, `onChange`,
  `placeholder`, etc.) — no call-site diff beyond the component name and removing
  `type="password"` (which `PasswordInput` manages internally).
- **`tabIndex={-1}` on the toggle button.** The eye icon button is a visual convenience only;
  keeping it out of the tab order means keyboard users move straight from the input to the
  next field, which matches every major password manager and browser's own show/hide
  implementation.
- **`forgot-password/page.tsx` also has a resend path.** The "Resend code" button on the
  forgot-password `request` step shares the same pattern as registration's resend. It's
  out of scope for Fix 3 (the feedback was specifically about registration) but should get the
  same 30-second cooldown treatment in a follow-up — the implementation is identical.
- **`PasswordMatchIndicator` shows muted "Doesn't match yet", not red.** Red would be correct
  as soon as a mismatch is detected at submit time, but the live indicator fires from the
  first keystroke in the confirm field — "doesn't match" is the expected state while the user
  is mid-typing. Turning red immediately would train users to ignore it. A green checkmark is
  clear positive feedback; a muted mismatch state avoids false alarm without losing clarity.
- **No backend changes.** All four fixes are purely client-side state and UI. `validatePassword`
  is a client-side function that mirrors the backend's Go validation — the Go side is
  unchanged. The new `isCommonPassword` export is a thin wrapper over the existing
  `COMMON_PASSWORDS` set already present in `api.ts`.
