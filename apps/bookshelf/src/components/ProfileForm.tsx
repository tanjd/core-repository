"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { CheckCircle2, XCircle } from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { api, emailLocalPart, validatePassword } from "@/lib/api";
import { PasswordStrengthMeter } from "@/components/PasswordStrengthMeter";
import { PasswordInput } from "@/components/PasswordInput";
import { PasswordMatchIndicator } from "@/components/PasswordMatchIndicator";
import { InviteLinkCard } from "@/components/InviteLinkCard";
import type { User, VerificationStatus } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

export function ProfileForm() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [verificationStatus, setVerificationStatus] =
    useState<VerificationStatus | null>(null);

  // Profile form
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [emailNotificationsEnabled, setEmailNotificationsEnabled] =
    useState(true);
  const [monthlyDigestEnabled, setMonthlyDigestEnabled] = useState(true);
  const [telegramUsername, setTelegramUsername] = useState("");
  const [whatsappUsername, setWhatsappUsername] = useState("");
  const [contactNote, setContactNote] = useState("");
  const [saving, setSaving] = useState(false);

  // Change password
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [pwError, setPwError] = useState("");
  const [changingPw, setChangingPw] = useState(false);

  // Google Books API key
  const [gbKey, setGbKey] = useState("");
  const [savingGbKey, setSavingGbKey] = useState(false);
  const [testingGbKey, setTestingGbKey] = useState(false);

  // OTP
  const [otpSent, setOtpSent] = useState(false);
  const [otpCode, setOtpCode] = useState("");
  const [sendingOtp, setSendingOtp] = useState(false);
  const [verifyingOtp, setVerifyingOtp] = useState(false);
  const [otpDebugCode, setOtpDebugCode] = useState("");

  // Pending email change confirmation
  const [emailChangePending, setEmailChangePending] = useState(false);
  const [pendingEmailTarget, setPendingEmailTarget] = useState("");
  const [emailChangeCode, setEmailChangeCode] = useState("");
  const [confirmingEmailChange, setConfirmingEmailChange] = useState(false);
  const [emailChangeDebugCode, setEmailChangeDebugCode] = useState("");

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) {
      router.push("/login");
      return;
    }
    Promise.all([api.me(), api.myVerificationStatus()])
      .then(([u, vs]) => {
        setUser(u);
        setVerificationStatus(vs);
        setName(u.name);
        setEmail(u.email);
        setPhone(
          u.phone?.startsWith("+65")
            ? u.phone.slice(3).trim()
            : (u.phone ?? ""),
        );
        setEmailNotificationsEnabled(u.email_notifications_enabled);
        setMonthlyDigestEnabled(u.monthly_digest_enabled);
        setTelegramUsername(u.telegram_username ?? "");
        setWhatsappUsername(u.whatsapp_username ?? "");
        setContactNote(u.contact_note ?? "");
        if (u.pending_email) {
          setEmailChangePending(true);
          setPendingEmailTarget(u.pending_email);
        }
      })
      .catch(() => router.push("/login"))
      .finally(() => setLoading(false));
  }, [router]);

  // Magic links from the email-verification / email-change-confirmation
  // emails (?verifyOtpToken=... / ?confirmEmailToken=...): auto-submit once
  // the authenticated user has loaded above, instead of making them retype
  // the code. Only works when opened in a browser that still holds this
  // account's bookshelf_token (the useEffect above already redirects to
  // /login otherwise) — the code is still shown in the email as a fallback.
  // Guarded by a ref (not a useState value in the deps array) so this only
  // ever fires once, on the render where `user` first becomes available.
  const autoVerifiedRef = useRef(false);
  useEffect(() => {
    if (!user || autoVerifiedRef.current) return;
    autoVerifiedRef.current = true;
    const params = new URLSearchParams(window.location.search);
    const verifyOtpToken = params.get("verifyOtpToken");
    const confirmEmailToken = params.get("confirmEmailToken");
    if (!user.verified && verifyOtpToken) {
      void submitOtpVerification({ token: verifyOtpToken });
    } else if (user.pending_email && confirmEmailToken) {
      void submitEmailChangeConfirmation({ token: confirmEmailToken });
    }
  }, [user]);

  async function handleSaveProfile(e?: FormEvent) {
    e?.preventDefault();
    setSaving(true);
    try {
      const localPhone = phone.trim();
      const fullPhone = localPhone
        ? localPhone.startsWith("+")
          ? localPhone
          : `+65 ${localPhone}`
        : undefined;
      const updated = await api.updateMe({
        name: name.trim() || undefined,
        email: email.trim() || undefined,
        phone: fullPhone,
        email_notifications_enabled: emailNotificationsEnabled,
        monthly_digest_enabled: monthlyDigestEnabled,
        telegram_username: telegramUsername.trim(),
        whatsapp_username: whatsappUsername.trim(),
        contact_note: contactNote.trim(),
      });
      if (updated.pending_email) {
        setUser(updated);
        setEmailChangePending(true);
        setPendingEmailTarget(updated.pending_email);
        setEmailChangeDebugCode(updated.pending_email_debug_code ?? "");
        toast.success(`Confirmation code sent to ${updated.pending_email}`);
      } else {
        const vs = await api.myVerificationStatus();
        setUser(updated);
        setVerificationStatus(vs);
        localStorage.setItem("bookshelf_user", JSON.stringify(updated));
        toast.success("Profile updated");
      }
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to update profile",
      );
    } finally {
      setSaving(false);
    }
  }

  async function handleChangePassword(e: FormEvent) {
    e.preventDefault();
    setPwError("");
    const validationError = validatePassword(newPassword, [
      user?.name ?? "",
      emailLocalPart(user?.email ?? ""),
    ]);
    if (validationError) {
      setPwError(validationError);
      return;
    }
    if (newPassword !== confirmNewPassword) {
      setPwError("New passwords do not match");
      return;
    }
    setChangingPw(true);
    try {
      await api.changePassword({
        current_password: currentPassword,
        new_password: newPassword,
        confirm_password: confirmNewPassword,
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmNewPassword("");
      toast.success("Password changed successfully");
    } catch (err) {
      setPwError(
        err instanceof Error ? err.message : "Failed to change password",
      );
    } finally {
      setChangingPw(false);
    }
  }

  async function handleSendOTP() {
    setSendingOtp(true);
    try {
      const { debug_code } = await api.sendOTP();
      setOtpSent(true);
      setOtpDebugCode(debug_code ?? "");
      toast.success("Verification code sent to your email");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to send code");
    } finally {
      setSendingOtp(false);
    }
  }

  async function handleTestGBKey() {
    setTestingGbKey(true);
    try {
      const result = await api.testGoogleBooksKey(gbKey.trim() || undefined);
      if (result.ok) {
        toast.success("API key is valid");
      } else {
        toast.error(result.message ?? "API key is invalid");
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Test failed");
    } finally {
      setTestingGbKey(false);
    }
  }

  async function handleSaveGBKey(e: FormEvent) {
    e.preventDefault();
    setSavingGbKey(true);
    try {
      const updated = await api.updateMe({
        google_books_api_key: gbKey.trim(),
      });
      setUser(updated);
      setGbKey("");
      toast.success(gbKey.trim() ? "API key saved" : "API key removed");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to save API key",
      );
    } finally {
      setSavingGbKey(false);
    }
  }

  async function handleRemoveGBKey() {
    setSavingGbKey(true);
    try {
      const updated = await api.updateMe({ google_books_api_key: "" });
      setUser(updated);
      toast.success("API key removed");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to remove API key",
      );
    } finally {
      setSavingGbKey(false);
    }
  }

  async function submitOtpVerification(
    data: { code: string } | { token: string },
  ) {
    setVerifyingOtp(true);
    try {
      const updated = await api.verifyOTP(data);
      const vs = await api.myVerificationStatus();
      setUser(updated);
      setVerificationStatus(vs);
      localStorage.setItem("bookshelf_user", JSON.stringify(updated));
      setOtpSent(false);
      setOtpCode("");
      setOtpDebugCode("");
      toast.success("Email verified!");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Invalid or expired code",
      );
    } finally {
      setVerifyingOtp(false);
    }
  }

  async function handleVerifyOTP(e: FormEvent) {
    e.preventDefault();
    await submitOtpVerification({ code: otpCode.trim() });
  }

  async function submitEmailChangeConfirmation(
    data: { code: string } | { token: string },
  ) {
    setConfirmingEmailChange(true);
    try {
      const updated = await api.confirmEmailChange(data);
      const vs = await api.myVerificationStatus();
      setUser(updated);
      setVerificationStatus(vs);
      localStorage.setItem("bookshelf_user", JSON.stringify(updated));
      setEmailChangePending(false);
      setPendingEmailTarget("");
      setEmailChangeCode("");
      setEmailChangeDebugCode("");
      setEmail(updated.email);
      toast.success("Email address updated");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Invalid or expired code",
      );
    } finally {
      setConfirmingEmailChange(false);
    }
  }

  async function handleConfirmEmailChange(e: FormEvent) {
    e.preventDefault();
    await submitEmailChangeConfirmation({ code: emailChangeCode.trim() });
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-6 max-w-2xl mx-auto">
        <div className="flex items-center gap-4 py-6">
          <div className="size-16 rounded-full bg-muted animate-pulse shrink-0" />
          <div className="flex flex-col gap-2">
            <div className="h-6 w-36 rounded bg-muted animate-pulse" />
            <div className="h-4 w-48 rounded bg-muted animate-pulse" />
            <div className="h-5 w-20 rounded-full bg-muted animate-pulse" />
          </div>
        </div>
        <div className="h-10 w-64 rounded-md bg-muted animate-pulse" />
        <div className="h-64 rounded-xl bg-muted animate-pulse" />
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="flex flex-col gap-6 max-w-2xl mx-auto">
      {/* Hero */}
      <div className="flex items-center gap-4 py-6">
        <div className="size-16 rounded-full bg-primary/10 flex items-center justify-center text-2xl font-bold text-primary select-none shrink-0">
          {user.name.charAt(0).toUpperCase()}
        </div>
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-bold">{user.name}</h1>
          <p className="text-sm text-muted-foreground">{user.email}</p>
          {verificationStatus ? (
            verificationStatus.eligible ? (
              <Badge variant="success" className="w-fit">
                Verified
              </Badge>
            ) : (
              <Badge variant="secondary" className="w-fit">
                Unverified
              </Badge>
            )
          ) : user.verified ? (
            <Badge variant="success" className="w-fit">
              Verified
            </Badge>
          ) : (
            <Badge variant="secondary" className="w-fit">
              Unverified
            </Badge>
          )}
        </div>
      </div>

      <Tabs defaultValue="profile">
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="security">Security</TabsTrigger>
          <TabsTrigger value="integrations">Integrations</TabsTrigger>
        </TabsList>

        {/* Profile tab */}
        <TabsContent value="profile" className="mt-4 flex flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Personal information</CardTitle>
              <CardDescription>
                Update your name, email, phone number, and messaging contact
                info
              </CardDescription>
            </CardHeader>
            <CardContent>
              {emailChangePending ? (
                <form
                  onSubmit={handleConfirmEmailChange}
                  className="flex flex-col gap-4"
                >
                  <p className="text-sm text-muted-foreground">
                    A code was sent to <strong>{pendingEmailTarget}</strong>.
                    Enter it below to confirm the change — your email won&apos;t
                    update until you do.
                  </p>
                  {emailChangeDebugCode && (
                    <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground">
                      Dev mode — no SMTP configured, so here&apos;s the code
                      directly: <strong>{emailChangeDebugCode}</strong>
                    </p>
                  )}
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="email-change-code"
                      className="text-sm font-medium"
                    >
                      Confirmation code
                    </label>
                    <Input
                      id="email-change-code"
                      type="text"
                      inputMode="numeric"
                      maxLength={6}
                      value={emailChangeCode}
                      onChange={(e) => setEmailChangeCode(e.target.value)}
                      placeholder="123456"
                    />
                  </div>
                  <div className="flex gap-2">
                    <Button
                      type="submit"
                      disabled={
                        confirmingEmailChange || emailChangeCode.length !== 6
                      }
                    >
                      {confirmingEmailChange ? "Confirming…" : "Confirm"}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      onClick={() => handleSaveProfile()}
                      disabled={saving}
                    >
                      {saving ? "Resending…" : "Resend code"}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      onClick={() => {
                        setEmailChangePending(false);
                        setPendingEmailTarget("");
                        setEmailChangeCode("");
                        setEmailChangeDebugCode("");
                        setEmail(user.email);
                      }}
                    >
                      Cancel
                    </Button>
                  </div>
                </form>
              ) : (
                <form
                  onSubmit={handleSaveProfile}
                  className="flex flex-col gap-4"
                >
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-name"
                      className="text-sm font-medium"
                    >
                      Name
                    </label>
                    <Input
                      id="profile-name"
                      type="text"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="Your name"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-email"
                      className="text-sm font-medium"
                    >
                      Email
                    </label>
                    <Input
                      id="profile-email"
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="you@example.com"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-phone"
                      className="text-sm font-medium"
                    >
                      Phone
                    </label>
                    <div className="flex rounded-md border border-input overflow-hidden focus-within:ring-1 focus-within:ring-ring">
                      <span className="flex items-center px-3 bg-muted text-muted-foreground text-sm border-r border-input select-none">
                        +65
                      </span>
                      <input
                        id="profile-phone"
                        type="tel"
                        className="flex-1 px-3 py-2 text-sm outline-none bg-background"
                        value={phone}
                        onChange={(e) => setPhone(e.target.value)}
                        placeholder="9123 4567"
                      />
                    </div>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-telegram"
                      className="text-sm font-medium"
                    >
                      Telegram username
                    </label>
                    <Input
                      id="profile-telegram"
                      type="text"
                      value={telegramUsername}
                      onChange={(e) => setTelegramUsername(e.target.value)}
                      placeholder="@yourusername"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-whatsapp"
                      className="text-sm font-medium"
                    >
                      WhatsApp username
                    </label>
                    <Input
                      id="profile-whatsapp"
                      type="text"
                      value={whatsappUsername}
                      onChange={(e) => setWhatsappUsername(e.target.value)}
                      placeholder="Your WhatsApp handle or number"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label
                      htmlFor="profile-contact-note"
                      className="text-sm font-medium"
                    >
                      Contact note
                    </label>
                    <Textarea
                      id="profile-contact-note"
                      value={contactNote}
                      onChange={(e) => setContactNote(e.target.value)}
                      placeholder="e.g. best days/times to meet, or a preferred meeting spot"
                      maxLength={500}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium">Email notifications</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Receive an email when someone requests to borrow your
                        book, your request is accepted, or a book on your
                        wishlist is added.
                      </p>
                    </div>
                    <Switch
                      checked={emailNotificationsEnabled}
                      onCheckedChange={setEmailNotificationsEnabled}
                      aria-label={
                        emailNotificationsEnabled
                          ? "Disable email notifications"
                          : "Enable email notifications"
                      }
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-medium">Monthly digest</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        A once-a-month email with new books and top recommended
                        books in the community.
                      </p>
                    </div>
                    <Switch
                      checked={monthlyDigestEnabled}
                      onCheckedChange={setMonthlyDigestEnabled}
                      aria-label={
                        monthlyDigestEnabled
                          ? "Disable monthly digest"
                          : "Enable monthly digest"
                      }
                    />
                  </div>
                  <div>
                    <Button type="submit" disabled={saving}>
                      {saving ? "Saving…" : "Save changes"}
                    </Button>
                  </div>
                </form>
              )}
            </CardContent>
          </Card>

          {/* An unverified member hasn't proved their own identity yet, so
              they can't hold an invite link — see requireEligibleInviter in
              internal/handlers/invite_codes.go. */}
          {user.verified && <InviteLinkCard />}
        </TabsContent>

        {/* Security tab */}
        <TabsContent value="security" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Change password</CardTitle>
              <CardDescription>
                Update your password. You must enter your current password to
                confirm.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={handleChangePassword}
                className="flex flex-col gap-4"
              >
                <div className="flex flex-col gap-1.5">
                  <label
                    htmlFor="current-password"
                    className="text-sm font-medium"
                  >
                    Current password
                  </label>
                  <PasswordInput
                    id="current-password"
                    autoComplete="current-password"
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    placeholder="Your current password"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="new-password" className="text-sm font-medium">
                    New password
                  </label>
                  <PasswordInput
                    id="new-password"
                    autoComplete="new-password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="At least 12 characters"
                  />
                  <PasswordStrengthMeter
                    password={newPassword}
                    disallowed={[
                      user?.name ?? "",
                      emailLocalPart(user?.email ?? ""),
                    ]}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label
                    htmlFor="confirm-new-password"
                    className="text-sm font-medium"
                  >
                    Confirm new password
                  </label>
                  <PasswordInput
                    id="confirm-new-password"
                    autoComplete="new-password"
                    value={confirmNewPassword}
                    onChange={(e) => setConfirmNewPassword(e.target.value)}
                    placeholder="Re-enter new password"
                  />
                  <PasswordMatchIndicator
                    password={newPassword}
                    confirm={confirmNewPassword}
                  />
                </div>
                {pwError && (
                  <p className="text-sm text-destructive">{pwError}</p>
                )}
                <div>
                  <Button
                    type="submit"
                    disabled={
                      changingPw ||
                      !currentPassword ||
                      !newPassword ||
                      !confirmNewPassword
                    }
                  >
                    {changingPw ? "Changing…" : "Change password"}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Integrations tab */}
        <TabsContent value="integrations" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Integrations</CardTitle>
              <CardDescription>
                Manage external services connected to your account.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-6">
              {/* Google Books API key */}
              <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">Google Books API Key</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Use your personal quota during book searches. The key is
                      stored encrypted and never exposed.
                    </p>
                    <Link
                      href="/about#google-books-api-key"
                      className="text-xs text-primary underline-offset-2 hover:underline mt-0.5 inline-block"
                    >
                      How to get a Google Books API key →
                    </Link>
                  </div>
                  {user.google_books_key_configured ? (
                    <Badge variant="success" className="shrink-0">
                      Configured
                    </Badge>
                  ) : (
                    <Badge variant="secondary" className="shrink-0">
                      Not configured
                    </Badge>
                  )}
                </div>
                <form
                  onSubmit={handleSaveGBKey}
                  className="flex flex-col gap-3"
                >
                  <PasswordInput
                    id="gb-api-key"
                    autoComplete="off"
                    value={gbKey}
                    onChange={(e) => setGbKey(e.target.value)}
                    placeholder={
                      user.google_books_key_configured
                        ? "Enter new key to replace"
                        : "Paste your API key"
                    }
                  />
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      disabled={
                        testingGbKey ||
                        savingGbKey ||
                        (!gbKey.trim() && !user.google_books_key_configured)
                      }
                      onClick={handleTestGBKey}
                    >
                      {testingGbKey ? "Testing…" : "Test"}
                    </Button>
                    <Button
                      type="submit"
                      disabled={savingGbKey || !gbKey.trim()}
                    >
                      {savingGbKey ? "Saving…" : "Save key"}
                    </Button>
                    {user.google_books_key_configured && (
                      <Button
                        type="button"
                        variant="outline"
                        disabled={savingGbKey}
                        onClick={handleRemoveGBKey}
                      >
                        Remove
                      </Button>
                    )}
                  </div>
                </form>
              </div>

              <Separator />

              {/* Email verification */}
              <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium">Email verification</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {user.verified
                        ? "Your email address has been verified."
                        : verificationStatus?.factors.some(
                              (factor) => factor.key === "email",
                            )
                          ? "Verify your email to unlock borrowing features."
                          : "Verifying your email helps other members trust that you're reachable."}
                    </p>
                  </div>
                  {user.verified ? (
                    <Badge variant="success" className="shrink-0">
                      Verified
                    </Badge>
                  ) : (
                    <Badge variant="secondary" className="shrink-0">
                      Unverified
                    </Badge>
                  )}
                </div>
                {!user.verified &&
                  (!otpSent ? (
                    verifyingOtp ? (
                      <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground w-fit">
                        Verifying your email…
                      </p>
                    ) : (
                      <Button
                        variant="outline"
                        onClick={handleSendOTP}
                        disabled={sendingOtp}
                        className="w-fit"
                      >
                        {sendingOtp ? "Sending…" : "Send verification code"}
                      </Button>
                    )
                  ) : (
                    <form
                      onSubmit={handleVerifyOTP}
                      className="flex flex-col gap-3"
                    >
                      <p className="text-sm text-muted-foreground">
                        A 6-digit code was sent to <strong>{user.email}</strong>
                        .
                      </p>
                      {otpDebugCode && (
                        <p className="text-sm rounded-md border border-dashed p-2 text-muted-foreground">
                          Dev mode — no SMTP configured, so here&apos;s the code
                          directly: <strong>{otpDebugCode}</strong>
                        </p>
                      )}
                      <div className="flex flex-col gap-1.5">
                        <label
                          htmlFor="otp-code"
                          className="text-sm font-medium"
                        >
                          Verification code
                        </label>
                        <Input
                          id="otp-code"
                          type="text"
                          inputMode="numeric"
                          maxLength={6}
                          value={otpCode}
                          onChange={(e) => setOtpCode(e.target.value)}
                          placeholder="123456"
                        />
                      </div>
                      <div className="flex gap-2">
                        <Button
                          type="submit"
                          disabled={verifyingOtp || otpCode.length !== 6}
                        >
                          {verifyingOtp ? "Verifying…" : "Verify"}
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          onClick={handleSendOTP}
                          disabled={sendingOtp}
                        >
                          Resend code
                        </Button>
                      </div>
                    </form>
                  ))}
              </div>

              {/* Verification requirements checklist */}
              {verificationStatus && verificationStatus.factors?.length > 0 && (
                <>
                  <Separator />
                  <div className="flex flex-col gap-3">
                    <div>
                      <p className="text-sm font-medium">
                        Borrowing requirements
                      </p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Complete all requirements to borrow books from the
                        community.
                      </p>
                    </div>
                    <ul className="flex flex-col gap-2">
                      {verificationStatus.factors.map((factor) => (
                        <li key={factor.key} className="flex items-start gap-2">
                          {factor.satisfied ? (
                            <CheckCircle2 className="size-4 text-green-600 mt-0.5 shrink-0" />
                          ) : (
                            <XCircle className="size-4 text-muted-foreground mt-0.5 shrink-0" />
                          )}
                          <div className="flex flex-col gap-0.5">
                            <span
                              className={`text-sm ${factor.satisfied ? "text-foreground" : "text-muted-foreground"}`}
                            >
                              {factor.key === "min_books_shared" &&
                              factor.target != null
                                ? `${factor.label} (${factor.current ?? 0}/${factor.target})`
                                : factor.label}
                            </span>
                            {!factor.satisfied && factor.key === "email" && (
                              <span className="text-xs text-muted-foreground">
                                Use the email verification section above
                              </span>
                            )}
                            {!factor.satisfied && factor.key === "phone" && (
                              <span className="text-xs text-muted-foreground">
                                Add your phone number in the{" "}
                                <button
                                  type="button"
                                  className="underline underline-offset-2 hover:text-foreground"
                                  onClick={() =>
                                    document
                                      .querySelector<HTMLElement>(
                                        '[data-value="profile"]',
                                      )
                                      ?.click()
                                  }
                                >
                                  Profile tab
                                </button>
                              </span>
                            )}
                            {!factor.satisfied &&
                              factor.key === "min_books_shared" && (
                                <Link
                                  href="/share"
                                  className="text-xs underline underline-offset-2 hover:text-foreground"
                                >
                                  Share a book →
                                </Link>
                              )}
                          </div>
                        </li>
                      ))}
                    </ul>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
