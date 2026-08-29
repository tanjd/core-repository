"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  api,
  emailLocalPart,
  validatePassword,
  type RegistrationResult,
} from "@/lib/api";
import { PasswordStrengthMeter } from "@/components/PasswordStrengthMeter";
import { PasswordInput } from "@/components/PasswordInput";
import { PasswordMatchIndicator } from "@/components/PasswordMatchIndicator";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

type Step = "details" | "verify-email";

// Mirrors ProfileForm's phone handling: a bare local number gets Singapore's
// +65 prefix; anything already starting with "+" is left as-is.
function toFullPhone(localPhone: string): string {
  const trimmed = localPhone.trim();
  return trimmed.startsWith("+") ? trimmed : `+65 ${trimmed}`;
}

export default function RegisterPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("details");

  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [phone, setPhone] = useState("");
  const [error, setError] = useState("");

  // Whether this community requires a phone number on file to *borrow*
  // (admin-toggleable `verification_requires_phone` setting). It never gates
  // signing up — it only changes this field's helper copy, so a registrant
  // knows to fill it in now rather than discovering the requirement at the
  // moment they try to borrow. Fails open to "don't mention it" both while
  // the check is in flight and if it errors out.
  const [requirePhone, setRequirePhone] = useState(false);
  const [phoneRequirementLoaded, setPhoneRequirementLoaded] = useState(false);

  const [emailOtpCode, setEmailOtpCode] = useState("");
  const [emailDebugCode, setEmailDebugCode] = useState("");

  // Invite-code banner state — see apps/bookshelf/docs/invite-code-spec.md.
  // inviteCode is only carried forward to send-email-otp when validation
  // confirmed it's still live; an invalid/revoked code is dropped rather
  // than submitted, so the registration just proceeds as a normal signup.
  const [inviteCode, setInviteCode] = useState("");
  const [inviterName, setInviterName] = useState("");
  const [inviteInvalid, setInviteInvalid] = useState(false);

  const [sendingEmailOtp, setSendingEmailOtp] = useState(false);
  const [verifyingEmailOtp, setVerifyingEmailOtp] = useState(false);
  // Debounces "Resend code" on the verify-email step so a user who doesn't
  // see the email within a few seconds can't flood their inbox (or trip the
  // backend rate limit silently) by tapping it repeatedly. Ticks down once
  // per second via a one-shot setTimeout re-armed on every change, rather
  // than setInterval, so there's nothing to leak on unmount.
  const [resendCooldown, setResendCooldown] = useState(0);
  // The magic-link path renders its own full-card state rather than the
  // details form, since there's nothing for the user to do while it runs and
  // nothing to fall back to: the form's state lives in whichever tab started
  // signup, which is usually not this one.
  const [verifyingLinkToken, setVerifyingLinkToken] = useState(false);
  const [linkError, setLinkError] = useState("");

  // Stores the session and leaves /register, for either verification path.
  // Both branches navigate away — there is no state in which a successful
  // verification drops the user back on a form.
  function completeRegistration(result: RegistrationResult) {
    if (result.status === "pending_approval") {
      toast.success(
        "Account created! An admin needs to approve it before you can sign in.",
      );
      router.push("/login");
      return;
    }
    localStorage.setItem("bookshelf_token", result.token);
    localStorage.setItem("bookshelf_user", JSON.stringify(result.user));
    toast.success("Account created! Welcome to Bookshelf.");
    router.push("/catalog");
  }

  // Magic link from the verification email (?verifyToken=...). The token is
  // all this page needs: the name/password/phone typed on the details step
  // were handed to the backend when the code was sent, so verifying finishes
  // signup outright — from this tab, another tab, or another device
  // entirely, which is the normal way people read email.
  //
  // Read via window.location on mount (rather than useSearchParams) to avoid
  // a server/client hydration mismatch and the Suspense boundary that hook
  // requires — same pattern as SharePage's ?q= prefill. The
  // setState-in-effect rule normally flags an effect that drives a render
  // branch, but there's no render-time alternative here: window.location
  // isn't available during SSR and verifying the token requires a network
  // call, so this genuinely has to run post-mount.
  const checkedVerifyTokenRef = useRef(false);
  useEffect(() => {
    if (checkedVerifyTokenRef.current) return;
    checkedVerifyTokenRef.current = true;
    const token = new URLSearchParams(window.location.search).get(
      "verifyToken",
    );
    if (!token) return;

    // eslint-disable-next-line react-hooks/set-state-in-effect
    setVerifyingLinkToken(true);
    api
      .verifyRegisterEmailOTP({ token })
      .then(completeRegistration)
      .catch((err) => {
        setLinkError(
          err instanceof Error
            ? err.message
            : "That verification link is invalid or expired",
        );
        setVerifyingLinkToken(false);
      });
    // completeRegistration is re-created every render but only ever reads
    // the result handed to it; this effect must run exactly once, on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    api
      .registrationRequirements()
      .then(({ require_phone }) => setRequirePhone(require_phone))
      .catch(() => setRequirePhone(false))
      .finally(() => setPhoneRequirementLoaded(true));
  }, []);

  // ?invite=<code> from a member's shared link. Read via window.location on
  // mount for the same reason as the verifyToken effect above (no SSR/
  // hydration mismatch, no Suspense boundary). Both params can coexist —
  // this reads independently of that one — but the invite check never
  // blocks the form: an invalid/revoked code just falls back to a normal
  // signup, per the spec's "brief notice, not a hard error".
  const checkedInviteRef = useRef(false);
  useEffect(() => {
    if (checkedInviteRef.current) return;
    checkedInviteRef.current = true;
    const code = new URLSearchParams(window.location.search).get("invite");
    if (!code) return;

    api
      .validateInviteCode(code)
      .then(({ valid, inviter_name }) => {
        if (valid) {
          setInviteCode(code);
          setInviterName(inviter_name);
        } else {
          setInviteInvalid(true);
        }
      })
      .catch(() => setInviteInvalid(true));
  }, []);

  useEffect(() => {
    if (resendCooldown <= 0) return;
    const id = setTimeout(() => setResendCooldown((s) => s - 1), 1000);
    return () => clearTimeout(id);
  }, [resendCooldown]);

  function startResendCooldown() {
    setResendCooldown(30);
  }

  // Sends (or resends) the verification email, handing the whole form to the
  // backend so either verification path can finish signup on its own.
  async function submitDetails() {
    const trimmedPhone = phone.trim();
    const { debug_code } = await api.sendRegisterEmailOTP({
      name,
      email,
      password,
      phone: trimmedPhone ? toFullPhone(trimmedPhone) : undefined,
      invite_code: inviteCode || undefined,
    });
    setEmailDebugCode(debug_code ?? "");
  }

  async function handleDetailsSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    const passwordError = validatePassword(password, [
      name,
      emailLocalPart(email),
    ]);
    if (passwordError) {
      setError(passwordError);
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    setSendingEmailOtp(true);
    try {
      await submitDetails();
      setStep("verify-email");
      startResendCooldown();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Could not send verification code",
      );
    } finally {
      setSendingEmailOtp(false);
    }
  }

  async function handleResendEmailOTP() {
    setSendingEmailOtp(true);
    try {
      await submitDetails();
      toast.success("Verification code sent");
      startResendCooldown();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to resend code");
    } finally {
      setSendingEmailOtp(false);
    }
  }

  async function handleVerifyEmailSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setVerifyingEmailOtp(true);
    try {
      const result = await api.verifyRegisterEmailOTP({
        email,
        code: emailOtpCode.trim(),
      });
      completeRegistration(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
      setVerifyingEmailOtp(false);
    }
  }

  // A magic-link click owns the whole card: nothing on the details form
  // applies, and this tab has no form state to return to on failure.
  if (verifyingLinkToken || linkError) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle className="text-2xl">
              {linkError ? "Link didn't work" : "Finishing your signup…"}
            </CardTitle>
            <CardDescription>
              {linkError
                ? linkError
                : "Verifying your email and creating your account."}
            </CardDescription>
          </CardHeader>
          {linkError && (
            <CardContent>
              <Button
                className="w-full"
                onClick={() => {
                  setLinkError("");
                  router.replace("/register");
                }}
              >
                Start again
              </Button>
            </CardContent>
          )}
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <Card className="w-full max-w-md">
        {step === "details" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Create account</CardTitle>
              <CardDescription>
                Join Bookshelf and start sharing books with your community
              </CardDescription>
            </CardHeader>
            <CardContent>
              {inviterName && (
                <Badge
                  variant="success"
                  className="mb-4 w-full justify-center rounded-md py-2 text-center whitespace-normal"
                >
                  Invited by {inviterName} — your account will be approved
                  automatically.
                </Badge>
              )}
              {inviteInvalid && (
                <Badge
                  variant="secondary"
                  className="mb-4 w-full justify-center rounded-md py-2 text-center whitespace-normal"
                >
                  This invite link is no longer valid — you can still register
                  normally.
                </Badge>
              )}
              <form
                onSubmit={handleDetailsSubmit}
                className="flex flex-col gap-4"
              >
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="name" className="text-sm font-medium">
                    Name
                  </label>
                  <Input
                    id="name"
                    type="text"
                    autoComplete="name"
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Your name"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="email" className="text-sm font-medium">
                    Email
                  </label>
                  <Input
                    id="email"
                    type="email"
                    autoComplete="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="password" className="text-sm font-medium">
                    Password
                  </label>
                  <PasswordInput
                    id="password"
                    autoComplete="new-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="At least 12 characters"
                  />
                  <PasswordStrengthMeter
                    password={password}
                    disallowed={[name, emailLocalPart(email)]}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label
                    htmlFor="confirm-password"
                    className="text-sm font-medium"
                  >
                    Confirm password
                  </label>
                  <PasswordInput
                    id="confirm-password"
                    autoComplete="new-password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Re-enter your password"
                  />
                  <PasswordMatchIndicator
                    password={password}
                    confirm={confirmPassword}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="phone" className="text-sm font-medium">
                    Phone number{" "}
                    <span className="text-muted-foreground font-normal">
                      (optional)
                    </span>
                  </label>
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-muted-foreground">+65</span>
                    <Input
                      id="phone"
                      type="tel"
                      autoComplete="tel-national"
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                      placeholder="9123 4567"
                    />
                  </div>
                  {phoneRequirementLoaded && requirePhone && (
                    <p className="text-sm text-muted-foreground">
                      You&apos;ll need a phone number on file to borrow books in
                      this community — you can also add it later from your
                      profile.
                    </p>
                  )}
                </div>
                {error && <p className="text-sm text-destructive">{error}</p>}
                <Button
                  type="submit"
                  disabled={sendingEmailOtp}
                  className="w-full"
                >
                  {sendingEmailOtp ? "Sending code…" : "Continue"}
                </Button>
              </form>
            </CardContent>
            <CardFooter className="justify-center">
              <p className="text-sm text-muted-foreground">
                Already have an account?{" "}
                <Link
                  href="/login"
                  className="text-primary hover:underline font-medium"
                >
                  Sign in
                </Link>
              </p>
            </CardFooter>
          </>
        )}

        {step === "verify-email" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Verify your email</CardTitle>
              <CardDescription>
                We sent a 6-digit code to <strong>{email}</strong>. Enter it
                here, or just tap the link in that email — either one finishes
                your signup, from any device.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={handleVerifyEmailSubmit}
                className="flex flex-col gap-4"
              >
                {emailDebugCode && (
                  <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground">
                    Dev mode — no SMTP configured, so here&apos;s the code
                    directly: <strong>{emailDebugCode}</strong>
                  </p>
                )}
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="email-otp" className="text-sm font-medium">
                    Verification code
                  </label>
                  <Input
                    id="email-otp"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    required
                    value={emailOtpCode}
                    onChange={(e) => setEmailOtpCode(e.target.value)}
                    placeholder="123456"
                  />
                </div>
                {error && <p className="text-sm text-destructive">{error}</p>}
                <Button
                  type="submit"
                  disabled={verifyingEmailOtp || emailOtpCode.length !== 6}
                  className="w-full"
                >
                  {verifyingEmailOtp
                    ? "Creating account…"
                    : "Verify and create account"}
                </Button>
                <div className="flex justify-between text-sm">
                  <button
                    type="button"
                    className="text-muted-foreground hover:underline"
                    onClick={() => setStep("details")}
                  >
                    ← Edit details
                  </button>
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
                </div>
              </form>
            </CardContent>
          </>
        )}
      </Card>
    </div>
  );
}
