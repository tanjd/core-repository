import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";

const faqs = [
  {
    q: "What is Bookshelf?",
    a: "Bookshelf is a community book-lending app. Members share physical copies of books they own, and others can browse the catalogue and request to borrow them.",
  },
  {
    q: "How do I share a book?",
    a: 'Use the "Share a Book" link in the navigation. Search for the book by title or author, select it from the results to auto-fill the details, and submit. Your copy will appear in the catalogue for others to request.',
  },
  {
    q: "How do I borrow a book?",
    a: 'Browse the Catalogue and click "Request to Borrow" on any available copy. The owner will receive a notification and can accept or decline. Once accepted, arrange collection directly with the owner.',
  },
  {
    q: "Why do I need a verified email to borrow?",
    a: "Email verification helps the community trust that borrowers are reachable. Once verified, all borrowing features are unlocked. You can verify your email in Profile → Integrations.",
  },
  {
    q: "What is the Google Books API key for?",
    a: "When you share a book, Bookshelf fetches metadata (cover art, description, etc.) from Google Books. By default the app uses a shared API key. You can supply your own key in Profile → Integrations to use your personal Google Books quota instead.",
  },
  {
    q: "How do I get a Google Books API key?",
    id: "google-books-api-key",
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
];

export default function AboutPage() {
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
            {faqs.map((faq, i) => (
              <div key={i}>
                {i > 0 && <Separator />}
                <div id={faq.id} className="flex flex-col gap-2 p-5">
                  <p className="text-sm font-semibold">{faq.q}</p>
                  {typeof faq.a === "string" ? (
                    <p className="text-sm text-muted-foreground">{faq.a}</p>
                  ) : (
                    <div className="text-sm text-muted-foreground">{faq.a}</div>
                  )}
                </div>
              </div>
            ))}
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
