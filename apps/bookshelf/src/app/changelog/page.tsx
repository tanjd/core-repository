import Link from "next/link";
import Markdown from "react-markdown";

import { ChangelogRuntimeInfo } from "@/app/changelog/ChangelogRuntimeInfo";
import { CHANGELOG_ENTRIES } from "@/lib/changelog.generated";

export const metadata = {
  title: "Changelog — Bookshelf",
  description: "Release notes for Bookshelf",
};

export default function ChangelogPage() {
  const appVersion = process.env.NEXT_PUBLIC_VERSION ?? "unknown";

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-8">
      <div className="flex flex-col gap-2">
        <h1 className="text-3xl font-bold">Changelog</h1>
        <p className="text-muted-foreground">
          Release notes for Bookshelf v{appVersion}.
        </p>
        <ChangelogRuntimeInfo />
      </div>

      <div className="flex flex-col gap-10">
        {CHANGELOG_ENTRIES.map((entry) => (
          <section key={entry.version} className="flex flex-col gap-3">
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h2 className="text-xl font-semibold">v{entry.version}</h2>
              {entry.date ? (
                <time
                  dateTime={entry.date}
                  className="text-sm text-muted-foreground"
                >
                  {entry.date}
                </time>
              ) : null}
            </div>
            <div className="prose prose-neutral dark:prose-invert max-w-none text-sm">
              <Markdown
                components={{
                  a: ({ href, children }) => (
                    <Link
                      href={href ?? "#"}
                      className="text-primary underline-offset-2 hover:underline"
                      target={href?.startsWith("http") ? "_blank" : undefined}
                      rel={
                        href?.startsWith("http")
                          ? "noopener noreferrer"
                          : undefined
                      }
                    >
                      {children}
                    </Link>
                  ),
                }}
              >
                {entry.body}
              </Markdown>
            </div>
          </section>
        ))}
      </div>

      <p className="text-sm text-muted-foreground">
        <Link href="/catalog" className="hover:underline underline-offset-2">
          Back to catalog
        </Link>
      </p>
    </div>
  );
}
