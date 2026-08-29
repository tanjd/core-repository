"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";

type Status = "loading" | "success" | "error";

export default function UnsubscribePage() {
  const [status, setStatus] = useState<Status>("loading");
  const [email, setEmail] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const calledRef = useRef(false);

  // Read the token from the URL on mount and immediately call the backend.
  // useRef guards against double-invocation in React Strict Mode.
  useEffect(() => {
    if (calledRef.current) return;
    calledRef.current = true;

    const token = new URLSearchParams(window.location.search).get("token");
    if (!token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setErrorMessage("No unsubscribe token found in the link.");
      setStatus("error");
      return;
    }

    api
      .unsubscribeDigest(token)
      .then((res) => {
        setEmail(res.email);
        setStatus("success");
      })
      .catch((err: unknown) => {
        setErrorMessage(
          err instanceof Error
            ? err.message
            : "This unsubscribe link is invalid or has expired.",
        );
        setStatus("error");
      });
  }, []);

  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <Card className="w-full max-w-md">
        {status === "loading" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Unsubscribing…</CardTitle>
              <CardDescription>Just a moment.</CardDescription>
            </CardHeader>
          </>
        )}

        {status === "success" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">
                You&apos;re unsubscribed
              </CardTitle>
              <CardDescription>
                {email} will no longer receive the monthly digest.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                You can re-enable it any time from your profile settings.
              </p>
            </CardContent>
            <CardFooter>
              <Button asChild variant="outline" className="w-full">
                <Link href="/login">Go to Bookshelf</Link>
              </Button>
            </CardFooter>
          </>
        )}

        {status === "error" && (
          <>
            <CardHeader>
              <CardTitle className="text-2xl">Link not valid</CardTitle>
              <CardDescription>{errorMessage}</CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                If you want to unsubscribe from the monthly digest, you can
                toggle it off in your profile settings after signing in.
              </p>
            </CardContent>
            <CardFooter>
              <Button asChild variant="outline" className="w-full">
                <Link href="/login">Sign in</Link>
              </Button>
            </CardFooter>
          </>
        )}
      </Card>
    </div>
  );
}
