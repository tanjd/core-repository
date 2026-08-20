"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import {
  Search,
  ScanLine,
  ArrowRightLeft,
  Bell,
  ListChecks,
  Sparkles,
  BookOpen,
} from "lucide-react";
import { api } from "@/lib/api";
import type { Book } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const features = [
  {
    icon: Search,
    title: "Browse the catalogue",
    body: "Search every book the community has shared, filter to what's available right now, and see who's lending it.",
  },
  {
    icon: ScanLine,
    title: "Share a book in seconds",
    body: "Scan the barcode or search by title — Bookshelf fetches the cover, author, and description for you.",
  },
  {
    icon: ArrowRightLeft,
    title: "Request to borrow",
    body: "One tap sends a request to the owner. Once they accept, arrange hand-off directly with them.",
  },
  {
    icon: Sparkles,
    title: "Ask for what's missing",
    body: "Add a book to your wishlist even if no one's shared it yet — you're notified the moment it shows up.",
  },
  {
    icon: ListChecks,
    title: "Join the waitlist",
    body: "If a copy's already out, queue up for it and get notified the moment it's back on the shelf.",
  },
  {
    icon: Bell,
    title: "Never miss an update",
    body: "Notifications cover request replies and returns; the community's announcements show up right alongside them.",
  },
] as const;

const steps = [
  {
    title: "Create your account",
    body: "Register with your email — verifying it unlocks borrowing so the community knows you're reachable.",
  },
  {
    title: "Share or browse",
    body: "List a few books you're happy to lend, and see what everyone else in the community has to offer.",
  },
  {
    title: "Borrow and lend",
    body: "Request a copy, or accept a request for one of yours. Arrange the hand-off, and enjoy the book.",
  },
];

export function LandingPage() {
  const [recentBooks, setRecentBooks] = useState<Book[] | null>(null);
  const [totalBooks, setTotalBooks] = useState<number | null>(null);

  useEffect(() => {
    api
      .getRecentBooks(8)
      .then(setRecentBooks)
      .catch(() => setRecentBooks([]));
    api
      .getBooks({ page_size: 1 })
      .then((result) => setTotalBooks(result.total))
      .catch(() => setTotalBooks(null));
  }, []);

  return (
    <div className="flex flex-col gap-16">
      {/* Hero */}
      <div className="flex flex-col items-center gap-6 text-center pt-6">
        <div className="flex items-center gap-2 text-muted-foreground text-sm font-medium">
          <BookOpen className="size-4" />
          Bookshelf
        </div>
        <h1 className="text-3xl sm:text-4xl font-bold max-w-2xl text-balance">
          A free lending library, built by our community
        </h1>
        <p className="text-muted-foreground max-w-xl text-balance">
          Share the books on your shelf, borrow the ones on everyone
          else&apos;s. No fees, no couriers — just neighbours lending books to
          neighbours.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link href="/register">
            <Button size="lg">Join the community</Button>
          </Link>
          <Link href="/login">
            <Button size="lg" variant="outline">
              Sign in
            </Button>
          </Link>
        </div>
        {totalBooks !== null && totalBooks > 0 && (
          <p className="text-sm text-muted-foreground">
            {totalBooks} {totalBooks === 1 ? "book" : "books"} shared so far
          </p>
        )}
      </div>

      {/* Screenshots */}
      <div className="flex flex-col gap-6">
        <h2 className="text-xl font-semibold text-center">
          A look at the shelf
        </h2>
        <div className="relative mx-auto w-full max-w-4xl pb-0 sm:pb-10">
          <div className="overflow-hidden rounded-xl border shadow-lg">
            <div className="flex items-center gap-1.5 border-b bg-muted px-3 py-2">
              <span className="size-2.5 rounded-full bg-destructive/60" />
              <span className="size-2.5 rounded-full bg-primary/40" />
              <span className="size-2.5 rounded-full bg-success/60" />
            </div>
            {/* Shown opposite the page's current theme, as a peek at the
                other mode — the real toggle (top right) switches both. */}
            <Image
              src="/screenshots/catalog-desktop-dark.png"
              alt="Bookshelf's catalogue in dark mode, showing book covers shared by the community with availability badges"
              width={1440}
              height={960}
              className="block dark:hidden w-full h-auto"
              sizes="(max-width: 896px) 100vw, 896px"
            />
            <Image
              src="/screenshots/catalog-desktop.png"
              alt="Bookshelf's catalogue in light mode, showing book covers shared by the community with availability badges"
              width={1440}
              height={960}
              className="hidden dark:block w-full h-auto"
              sizes="(max-width: 896px) 100vw, 896px"
            />
          </div>
          <div className="hidden sm:block absolute -bottom-8 -right-6 w-[26%] overflow-hidden rounded-[1.25rem] border-4 border-background shadow-xl">
            <Image
              src="/screenshots/catalog-mobile-dark.png"
              alt="The same catalogue on a phone in dark mode, with the bottom tab bar for quick navigation"
              width={390}
              height={664}
              className="block dark:hidden w-full h-auto"
              sizes="26vw"
            />
            <Image
              src="/screenshots/catalog-mobile.png"
              alt="The same catalogue on a phone in light mode, with the bottom tab bar for quick navigation"
              width={390}
              height={664}
              className="hidden dark:block w-full h-auto"
              sizes="26vw"
            />
          </div>
        </div>
      </div>

      {/* Live teaser */}
      {recentBooks === null || recentBooks.length > 0 ? (
        <div className="flex flex-col gap-4">
          <h2 className="text-lg font-semibold text-center">
            Recently added to the shelf
          </h2>
          <div className="grid grid-cols-4 sm:grid-cols-8 gap-3">
            {recentBooks === null
              ? Array.from({ length: 8 }).map((_, i) => (
                  <Skeleton key={i} className="aspect-[2/3] rounded-md" />
                ))
              : recentBooks.map((book) => (
                  <div
                    key={book.id}
                    className="relative aspect-[2/3] w-full rounded-md bg-muted overflow-hidden"
                    title={book.title}
                  >
                    {book.cover_url ? (
                      <Image
                        src={book.cover_url}
                        alt={`Cover of ${book.title}`}
                        fill
                        className="object-cover"
                        sizes="(max-width: 640px) 25vw, 12vw"
                      />
                    ) : (
                      <div className="flex h-full items-center justify-center p-2">
                        <BookOpen className="size-6 text-muted-foreground/60" />
                      </div>
                    )}
                  </div>
                ))}
          </div>
        </div>
      ) : null}

      {/* Features */}
      <div className="flex flex-col gap-6">
        <h2 className="text-xl font-semibold text-center">
          Everything you need to lend and borrow
        </h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {features.map((feature) => (
            <Card key={feature.title}>
              <CardContent className="flex flex-col gap-2">
                <feature.icon className="size-5 text-primary" />
                <p className="font-medium text-sm">{feature.title}</p>
                <p className="text-sm text-muted-foreground">{feature.body}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      {/* How it works */}
      <div className="flex flex-col gap-6">
        <h2 className="text-xl font-semibold text-center">How it works</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
          {steps.map((step, i) => (
            <div key={step.title} className="flex flex-col gap-2">
              <div className="flex size-8 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-semibold">
                {i + 1}
              </div>
              <p className="font-medium text-sm">{step.title}</p>
              <p className="text-sm text-muted-foreground">{step.body}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Closing CTA */}
      <div className="flex flex-col items-center gap-4 text-center pb-4">
        <p className="text-muted-foreground text-sm">
          Ready to see what the community&apos;s got on the shelf?
        </p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link href="/register">
            <Button size="lg">Join the community</Button>
          </Link>
          <Link
            href="/about"
            className="text-sm text-primary underline-offset-2 hover:underline"
          >
            Read the FAQ
          </Link>
        </div>
      </div>
    </div>
  );
}
