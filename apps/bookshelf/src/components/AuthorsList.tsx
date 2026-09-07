import Link from "next/link";
import type { AuthorSummary } from "@/lib/types";

// Alphabetical, sticky-letter-header list of distinct authors — the
// "Authors" view nested inside the Catalog page. Grouping is exact-string
// only (same rule as the per-author page itself); see
// apps/bookshelf/docs/author-view-spec.md's "Authors index page". Grouping
// happens client-side over whatever page of already-alphabetized authors
// the backend returned — a page never spans more than the current
// PAGE_SIZE, so this is just clustering consecutive same-letter rows, not
// a full re-sort.
export function AuthorsList({ authors }: { authors: AuthorSummary[] }) {
  const groups: { letter: string; authors: AuthorSummary[] }[] = [];
  for (const author of authors) {
    const letter = author.author.charAt(0).toUpperCase() || "#";
    const lastGroup = groups[groups.length - 1];
    if (lastGroup?.letter === letter) {
      lastGroup.authors.push(author);
    } else {
      groups.push({ letter, authors: [author] });
    }
  }

  return (
    <div className="flex flex-col">
      {groups.map((group) => (
        <div key={group.letter}>
          <div className="sticky top-0 z-10 bg-background/95 backdrop-blur-sm border-b px-1 py-1.5 text-xs font-semibold text-muted-foreground">
            {group.letter}
          </div>
          {group.authors.map((author) => (
            <Link
              key={author.author}
              href={`/authors/${encodeURIComponent(author.author)}`}
              className="flex items-center justify-between gap-3 px-1 py-3 border-b hover:bg-accent/50"
            >
              <span className="font-medium">{author.author}</span>
              <span className="text-sm text-muted-foreground shrink-0">
                {author.book_count} {author.book_count === 1 ? "book" : "books"}
              </span>
            </Link>
          ))}
        </div>
      ))}
    </div>
  );
}
