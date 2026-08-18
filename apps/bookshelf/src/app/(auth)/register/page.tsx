"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { api, emailLocalPart, validatePassword } from "@/lib/api";
import { PasswordStrengthMeter } from "@/components/PasswordStrengthMeter";
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

type Step = "details" | "verify-email" | "verify-phone";

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

  const [emailOtpCode, setEmailOtpCode] = useState("");
  const [emailDebugCode, setEmailDebugCode] = useState("");
  const [emailVerificationToken, setEmailVerificationToken] = useState("");

  const [phoneOtpCode, setPhoneOtpCode] = useState("");
  const [phoneMockCode, setPhoneMockCode] = useState("");
  const [fullPhone, setFullPhone] = useState("");

  const [sendingEmailOtp, setSendingEmailOtp] = useState(false);
  const [verifyingEmailOtp, setVerifyingEmailOtp] = useState(false);
  const [sendingPhoneOtp, setSendingPhoneOtp] = useState(false);
  const [verifyingPhoneOtp, setVerifyingPhoneOtp] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // Takes both tokens as parameters rather than reading them from state:
  // setEmailVerificationToken/setPhoneVerificationToken don't take effect
  // until the next render, so a caller that just set one and immediately
  // calls this in the same event handler would otherwise see a stale
  // (empty) value via closure.
  async function finalizeRegistration(
    emailToken: string,
    phoneToken: string | undefined,
    phoneForSubmit: string,
  ) {
    setSubmitting(true);
    try {
      const { token, user } = await api.register({
        name,
        email,
        password,
        phone: phoneForSubmit || undefined,
        email_verification_token: emailToken,
        phone_verification_token: phoneToken,
      });
      if (!token) {
        toast.success(
          "Account created! An admin needs to approve it before you can sign in.",
        );
        router.push("/login");
        return;
      }
      localStorage.setItem("bookshelf_token", token);
      localStorage.setItem("bookshelf_user", JSON.stringify(user));
      toast.success("Account created! Welcome to Bookshelf.");
      router.push("/catalog");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Registration failed");
    } finally {
      setSubmitting(false);
    }
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
      const { debug_code } = await api.sendRegisterEmailOTP(email);
      setEmailDebugCode(debug_code ?? "");
      setStep("verify-email");
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
      const { debug_code } = await api.sendRegisterEmailOTP(email);
      setEmailDebugCode(debug_code ?? "");
      toast.success("Verification code sent");
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
      const { verification_token } = await api.verifyRegisterEmailOTP(
        email,
        emailOtpCode.trim(),
      );
      setEmailVerificationToken(verification_token);

      const trimmedPhone = phone.trim();
      if (!trimmedPhone) {
        await finalizeRegistration(verification_token, undefined, "");
        return;
      }

      const full = toFullPhone(trimmedPhone);
      setFullPhone(full);
      setSendingPhoneOtp(true);
      try {
        const { mock_code } = await api.sendRegisterPhoneOTP(full);
        setPhoneMockCode(mock_code);
        setStep("verify-phone");
      } finally {
        setSendingPhoneOtp(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setVerifyingEmailOtp(false);
    }
  }

  async function handleResendPhoneOTP() {
    setSendingPhoneOtp(true);
    try {
      const { mock_code } = await api.sendRegisterPhoneOTP(fullPhone);
      setPhoneMockCode(mock_code);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to resend code");
    } finally {
      setSendingPhoneOtp(false);
    }
  }

  async function handleVerifyPhoneSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setVerifyingPhoneOtp(true);
    try {
      const { verification_token } = await api.verifyRegisterPhoneOTP(
        fullPhone,
        phoneOtpCode.trim(),
      );
      await finalizeRegistration(
        emailVerificationToken,
        verification_token,
        fullPhone,
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setVerifyingPhoneOtp(false);
    }
  }

  function handleSkipPhone() {
    setFullPhone("");
    setPhone("");
    void finalizeRegistration(emailVerificationToken, undefined, "");
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
                  <Input
                    id="password"
                    type="password"
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
                  <Input
                    id="confirm-password"
                    type="password"
                    autoComplete="new-password"
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Re-enter your password"
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
                A 6-digit code was sent to <strong>{email}</strong>.
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
                  disabled={
                    verifyingEmailOtp ||
                    sendingPhoneOtp ||
                    submitting ||
                    emailOtpCode.length !== 6
                  }
                  className="w-full"
                >
                  {verifyingEmailOtp || sendingPhoneOtp || submitting
                    ? "Verifying…"
                    : "Verify"}
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
                    className="text-primary hover:underline disabled:opacity-50"
                    disabled={sendingEmailOtp}
                    onClick={handleResendEmailOTP}
                  >
                    {sendingEmailOtp ? "Sending…" : "Resend code"}
                  </button>
                </div>
              </form>
            </CardContent>
          </>
        )}

        {step === "verify-phone" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Verify your phone</CardTitle>
              <CardDescription>
                A 6-digit code was sent to <strong>{fullPhone}</strong>.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={handleVerifyPhoneSubmit}
                className="flex flex-col gap-4"
              >
                <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground">
                  SMS delivery is mocked for now — this is where a real provider
                  would text the code. Your code:{" "}
                  <strong>{phoneMockCode}</strong>
                </p>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="phone-otp" className="text-sm font-medium">
                    Verification code
                  </label>
                  <Input
                    id="phone-otp"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    required
                    value={phoneOtpCode}
                    onChange={(e) => setPhoneOtpCode(e.target.value)}
                    placeholder="123456"
                  />
                </div>
                {error && <p className="text-sm text-destructive">{error}</p>}
                <Button
                  type="submit"
                  disabled={
                    verifyingPhoneOtp || submitting || phoneOtpCode.length !== 6
                  }
                  className="w-full"
                >
                  {verifyingPhoneOtp || submitting
                    ? "Creating account…"
                    : "Verify and create account"}
                </Button>
                <div className="flex justify-between text-sm">
                  <button
                    type="button"
                    className="text-muted-foreground hover:underline"
                    disabled={submitting}
                    onClick={handleSkipPhone}
                  >
                    Skip phone
                  </button>
                  <button
                    type="button"
                    className="text-primary hover:underline disabled:opacity-50"
                    disabled={sendingPhoneOtp}
                    onClick={handleResendPhoneOTP}
                  >
                    {sendingPhoneOtp ? "Sending…" : "Resend code"}
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
