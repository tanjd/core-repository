"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";

const faqs = [
  {
    id: "what-is-bookshelf",
    q: "What is Bookshelf?",
    a: "Bookshelf is a community book-lending app. Members share physical copies of books they own, and others can browse the catalogue and request to borrow them.",
  },
  {
    id: "who-is-it-for",
    q: "Who is Bookshelf for?",
    a: (
      <>
        <p>
          Bookshelf is open-source and free to self-host, so anyone can run
          their own instance for a small trust group — a building, an office, a
          church, a friend group, a book club.
        </p>
        <p>
          Within a community, it&apos;s for anyone with books to share and
          anyone who wants to read more — whether you&apos;ve got a shelf full
          of books gathering dust or you&apos;re just looking for your next
          read.
        </p>
      </>
    ),
  },
  {
    id: "share-a-book",
    q: "How do I share a book?",
    a: (
      <>
        <p>
          Use the &ldquo;Share a Book&rdquo; link in the navigation, then search
          by title, author, or ISBN and select it from the results to auto-fill
          the details. If nothing turns up, you can enter the book&apos;s
          details manually instead.
        </p>
        <p>
          On mobile, you can also tap &ldquo;Scan a barcode instead&rdquo; to
          scan a book&apos;s ISBN with your camera — handy for adding several
          books from your shelf in one go.
        </p>
        <p>
          Either way, submitting adds your copy to the catalogue for others to
          request.
        </p>
      </>
    ),
  },
  {
    id: "borrow-a-book",
    q: "How do I borrow a book?",
    a: (
      <>
        <p>
          Browse the Catalogue and click &ldquo;Request to Borrow&rdquo; on any
          available copy. The owner will receive a notification and can accept
          or decline. Once accepted, arrange collection directly with the owner.
        </p>
        <p>
          If every copy of a book is currently loaned out, you can join its
          waitlist instead and get notified when one comes back in.
        </p>
      </>
    ),
  },
  {
    id: "invite-a-member",
    q: "How do I invite someone to join?",
    a: (
      <>
        <p>
          Every verified member has their own personal invite link — find yours
          under &ldquo;Invite a member&rdquo; on the Profile tab of your profile
          page.
        </p>
        <p>
          Sharing it does more than just point someone at Bookshelf: signing up
          through your link skips this community&apos;s normal registration
          gate, even if the community has closed registration or requires admin
          approval for new members. Your invite is treated as the vouch.
        </p>
        <p>
          An admin can disable invite links community-wide, or revoke your link
          specifically — a revoked link stops working immediately, and
          you&apos;ll get a fresh one the next time you visit your profile.
        </p>
      </>
    ),
  },
  {
    id: "borrowing-requirements",
    q: "Why can't I request to borrow a book?",
    a: (
      <>
        <p>
          Some communities require you to meet certain trust requirements before
          you can request to borrow — for example a verified email, a verified
          phone number, or having shared a minimum number of books yourself.
          Which of these apply (if any) is set by your community, not fixed by
          the app.
        </p>
        <p>
          Check Profile → Integrations for a checklist of what&apos;s required
          and what you&apos;ve already completed.
        </p>
      </>
    ),
  },
  {
    id: "google-books-api-key-purpose",
    q: "What is the Google Books API key for?",
    a: (
      <>
        <p>
          When you share a book, Bookshelf looks up its metadata (cover art,
          description, etc.) from Open Library and BookBrainz automatically — no
          key needed. Google Books is an additional source that&apos;s only
          included when an API key is available: by default the app uses a
          shared key, but you can supply your own in Profile → Integrations to
          use your personal quota instead.
        </p>
      </>
    ),
  },
  {
    id: "google-books-api-key",
    q: "How do I get a Google Books API key?",
    a: (
      <ol className="list-decimal list-inside space-y-1.5">
        <li>
          Go to the{" "}
          <a
            href="https://console.cloud.google.com/"
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary underline-offset-2 hover:underline"
          >
            Google Cloud Console
          </a>{" "}
          and sign in.
        </li>
        <li>Create a new project (or select an existing one).</li>
        <li>
          In the left menu, go to <strong>APIs &amp; Services → Library</strong>
          .
        </li>
        <li>
          Search for <strong>&ldquo;Books API&rdquo;</strong> and click{" "}
          <strong>Enable</strong>.
        </li>
        <li>
          Go to <strong>APIs &amp; Services → Credentials</strong> and click{" "}
          <strong>Create Credentials → API key</strong>.
        </li>
        <li>
          Copy the generated key and paste it into{" "}
          <strong>Profile → Integrations → Google Books API Key</strong>.
        </li>
        <li>Optionally restrict the key to the Books API only for security.</li>
      </ol>
    ),
  },
  {
    id: "hardcover-api-key-purpose",
    q: "What is the Hardcover API key for?",
    a: (
      <>
        <p>
          Hardcover is another additional metadata source, alongside Google
          Books — it&apos;s only included when an API key is available. By
          default the app uses a shared key (if the self-hoster configured one),
          but you can supply your own in Profile → Integrations to use your
          personal quota instead.
        </p>
      </>
    ),
  },
  {
    id: "hardcover-api-key",
    q: "How do I get a Hardcover API key?",
    a: (
      <ol className="list-decimal list-inside space-y-1.5">
        <li>
          Go to{" "}
          <a
            href="https://hardcover.app/account/api"
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary underline-offset-2 hover:underline"
          >
            hardcover.app/account/api
          </a>{" "}
          and sign in (or create a free account).
        </li>
        <li>Copy the personal access token shown on that page.</li>
        <li>
          Paste it into{" "}
          <strong>Profile → Integrations → Hardcover API Key</strong>.
        </li>
      </ol>
    ),
  },
  {
    id: "telegram-notifications",
    q: "Can I get notifications on Telegram?",
    a: (
      <>
        <p>
          Yes — go to Profile → Integrations and click <strong>Connect</strong>{" "}
          next to Telegram. That opens Telegram to the Bookshelf bot, which
          finishes linking your account automatically — there&apos;s no code to
          copy or paste.
        </p>
        <p>
          Once connected, a toggle on the Profile tab turns on Telegram
          notifications for loan requests, approvals, and wishlist matches,
          alongside your regular in-app notifications. You can disconnect
          Telegram from Profile → Integrations at any time.
        </p>
      </>
    ),
  },
];

export default function AboutPage() {
  const [openItem, setOpenItem] = useState<string | undefined>(undefined);

  // Deep-link support: Profile → Integrations links to /about#google-books-api-key
  // and expects that FAQ entry to open and scroll into view. Guarded by a ref
  // (not a useState value in the deps array) so the effect only ever fires its
  // setState once, on mount — same pattern as share/page.tsx's prefill effect.
  const hashAppliedRef = useRef(false);
  useEffect(() => {
    if (hashAppliedRef.current) return;
    hashAppliedRef.current = true;
    const hash = window.location.hash.slice(1);
    const matchedId = faqs.find((faq) => faq.id === hash)?.id;
    setOpenItem(matchedId);
    if (matchedId)
      document.getElementById(matchedId)?.scrollIntoView({ block: "start" });
  }, []);

  return (
    <div className="max-w-2xl mx-auto flex flex-col gap-8">
      {/* About */}
      <div className="flex flex-col gap-3">
        <h1 className="text-2xl font-bold">About Bookshelf</h1>
        <p className="text-muted-foreground">
          Bookshelf is a free, self-hosted community lending library. Members
          donate their shelf space by listing physical books they are happy to
          lend out. Others browse the shared catalogue, request a copy, and
          arrange hand-off with the owner — no fees, no couriers, just
          community.
        </p>
      </div>

      <Separator />

      {/* FAQ */}
      <div className="flex flex-col gap-4">
        <h2 className="text-xl font-semibold">FAQ</h2>
        <Card>
          <CardContent className="p-0">
            <Accordion
              type="single"
              collapsible
              value={openItem}
              onValueChange={setOpenItem}
              className="px-5"
            >
              {faqs.map((faq) => (
                <AccordionItem
                  key={faq.id}
                  value={faq.id}
                  id={faq.id}
                  className="scroll-mt-20"
                >
                  <AccordionTrigger>{faq.q}</AccordionTrigger>
                  <AccordionContent>
                    {typeof faq.a === "string" ? (
                      <p>{faq.a}</p>
                    ) : (
                      <div className="flex flex-col gap-3">{faq.a}</div>
                    )}
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </CardContent>
        </Card>
      </div>

      <Separator />

      {/* Support */}
      <div className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold">Support the project</h2>
        <p className="text-muted-foreground text-sm">
          Bookshelf is free and open-source. If it&apos;s useful to your
          community, consider buying the developer a coffee — it keeps the
          project going.
        </p>
        <div>
          <Link
            href="https://buymeacoffee.com/tanjd"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-full bg-yellow-400 hover:bg-yellow-300 text-yellow-900 font-semibold text-sm px-5 py-2.5 transition-colors"
          >
            ☕ Buy me a coffee
          </Link>
        </div>
      </div>
    </div>
  );
}
