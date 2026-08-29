"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { api, emailLocalPart, validatePassword } from "@/lib/api";
import { PasswordStrengthMeter } from "@/components/PasswordStrengthMeter";
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

type Step = "request" | "reset";

export default function ForgotPasswordPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("request");

  const [email, setEmail] = useState("");
  const [debugCode, setDebugCode] = useState("");

  const [code, setCode] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [resetting, setResetting] = useState(false);

  // Magic link from the reset email (?resetToken=...): skip straight to the
  // "new password" fields, no code entry needed. Read via window.location on
  // mount (rather than useSearchParams) to avoid a server/client hydration
  // mismatch and the Suspense boundary that hook requires — same pattern as
  // SharePage's ?q= prefill. The setState-in-effect rule normally flags an
  // effect that drives a render branch, but there's no render-time
  // alternative here: window.location isn't available during SSR, so this
  // genuinely has to run post-mount, once, reading an external source (the
  // URL) exactly as the rule's own guidance allows.
  const [resetToken, setResetToken] = useState<string | null>(null);
  const checkedTokenRef = useRef(false);
  useEffect(() => {
    if (checkedTokenRef.current) return;
    checkedTokenRef.current = true;
    const token = new URLSearchParams(window.location.search).get("resetToken");
    if (token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setResetToken(token);
      setStep("reset");
    }
  }, []);

  async function handleRequestSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSending(true);
    try {
      const { debug_code } = await api.forgotPassword(email);
      setDebugCode(debug_code ?? "");
      setStep("reset");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Could not send reset code",
      );
    } finally {
      setSending(false);
    }
  }

  async function handleResendCode() {
    setSending(true);
    try {
      const { debug_code } = await api.forgotPassword(email);
      setDebugCode(debug_code ?? "");
      toast.success("If that email is registered, a new code was sent");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to resend code");
    } finally {
      setSending(false);
    }
  }

  async function handleResetSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    const passwordError = validatePassword(newPassword, [
      emailLocalPart(email),
    ]);
    if (passwordError) {
      // Checklist already shows which requirement is unmet — no prose error
      // needed.
      return;
    }
    if (newPassword !== confirmPassword) {
      // Live match indicator already shows the mismatch; just block
      // submission.
      return;
    }
    setResetting(true);
    try {
      await api.resetPassword({
        ...(resetToken ? { token: resetToken } : { email, code: code.trim() }),
        new_password: newPassword,
        confirm_password: confirmPassword,
      });
      toast.success("Password reset — you can now sign in");
      router.push("/login");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not reset password");
    } finally {
      setResetting(false);
    }
  }

  function handleUseCodeInstead() {
    setResetToken(null);
    setStep("request");
  }

  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <Card className="w-full max-w-md">
        {step === "request" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Forgot password</CardTitle>
              <CardDescription>
                Enter your account email and we&apos;ll send you a reset code
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={handleRequestSubmit}
                className="flex flex-col gap-4"
              >
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
                {error && <p className="text-sm text-destructive">{error}</p>}
                <Button type="submit" disabled={sending} className="w-full">
                  {sending ? "Sending code…" : "Send reset code"}
                </Button>
              </form>
            </CardContent>
            <CardFooter className="justify-center">
              <p className="text-sm text-muted-foreground">
                Remembered your password?{" "}
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

        {step === "reset" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Reset your password</CardTitle>
              <CardDescription>
                {resetToken ? (
                  "Enter a new password to complete your reset."
                ) : (
                  <>
                    If <strong>{email}</strong> is registered, a 6-digit code
                    was sent to it.
                  </>
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={handleResetSubmit}
                className="flex flex-col gap-4"
              >
                {!resetToken && debugCode && (
                  <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground">
                    Dev mode — no SMTP configured, so here&apos;s the code
                    directly: <strong>{debugCode}</strong>
                  </p>
                )}
                {!resetToken && (
                  <div className="flex flex-col gap-1.5">
                    <label htmlFor="reset-otp" className="text-sm font-medium">
                      Reset code
                    </label>
                    <Input
                      id="reset-otp"
                      type="text"
                      inputMode="numeric"
                      maxLength={6}
                      required
                      value={code}
                      onChange={(e) => setCode(e.target.value)}
                      placeholder="123456"
                    />
                  </div>
                )}
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="new-password" className="text-sm font-medium">
                    New password
                  </label>
                  <Input
                    id="new-password"
                    type="password"
                    autoComplete="new-password"
                    required
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="At least 12 characters"
                  />
                  <PasswordStrengthMeter
                    password={newPassword}
                    disallowed={[emailLocalPart(email)]}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label
                    htmlFor="confirm-new-password"
                    className="text-sm font-medium"
                  >
                    Confirm new password
                  </label>
                  <Input
                    id="confirm-new-password"
                    type="password"
                    autoComplete="new-password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Re-enter your new password"
                  />
                  <PasswordMatchIndicator
                    password={newPassword}
                    confirm={confirmPassword}
                  />
                </div>
                {error && <p className="text-sm text-destructive">{error}</p>}
                <Button
                  type="submit"
                  disabled={resetting || (!resetToken && code.length !== 6)}
                  className="w-full"
                >
                  {resetting ? "Resetting…" : "Reset password"}
                </Button>
                <div className="flex justify-between text-sm">
                  {resetToken ? (
                    <button
                      type="button"
                      className="text-muted-foreground hover:underline"
                      onClick={handleUseCodeInstead}
                    >
                      ← Enter a code instead
                    </button>
                  ) : (
                    <>
                      <button
                        type="button"
                        className="text-muted-foreground hover:underline"
                        onClick={() => setStep("request")}
                      >
                        ← Use a different email
                      </button>
                      <button
                        type="button"
                        className="text-primary hover:underline disabled:opacity-50"
                        disabled={sending}
                        onClick={handleResendCode}
                      >
                        {sending ? "Sending…" : "Resend code"}
                      </button>
                    </>
                  )}
                </div>
              </form>
            </CardContent>
          </>
        )}
      </Card>
    </div>
  );
}
