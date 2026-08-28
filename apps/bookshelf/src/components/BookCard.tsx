import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { BookCover } from "@/components/BookCover";
import { RecommendButton } from "@/components/RecommendButton";
import type { Book } from "@/lib/types";

interface BookCardProps {
  book: Book;
  ownedByMe?: boolean;
  catalogHref?: string;
}

export function BookCard({ book, ownedByMe, catalogHref }: BookCardProps) {
  const detailHref = catalogHref
    ? `/catalog/${book.id}?from=${encodeURIComponent(catalogHref)}`
    : `/catalog/${book.id}`;
  return (
    <Link href={detailHref} className="block group">
      <Card className="h-full overflow-hidden transition-shadow group-hover:shadow-md py-0 gap-0">
        <div className="relative aspect-[2/3] w-full bg-muted overflow-hidden">
          {ownedByMe && (
            <Badge className="absolute top-2 left-2 z-10 shadow-sm">
              Yours
            </Badge>
          )}
          <BookCover
            title={book.title}
            author={book.author}
            coverUrl={book.cover_url}
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 20vw"
            className="transition-transform group-hover:scale-105"
          />
        </div>
        <CardContent className="px-3 py-3 flex flex-1 flex-col gap-1">
          <p className="font-medium text-sm leading-snug line-clamp-2">
            {book.title}
          </p>
          {book.author && (
            <p className="text-xs text-muted-foreground line-clamp-1">
              {book.author}
            </p>
          )}
          <div className="mt-auto pt-1 flex items-center justify-between gap-2">
            {typeof book.available_copies === "number" ? (
              <Badge
                variant={book.available_copies > 0 ? "success" : "secondary"}
              >
                {book.available_copies > 0
                  ? `${book.available_copies} available`
                  : "Unavailable"}
              </Badge>
            ) : (
              <span />
            )}
            {/* Count + toggle only — no facepile at card density, per
                docs/book-recommendations-spec.md's "Facepile stays off the
                card". Tapping recommends/un-recommends without leaving the
                catalog; RecommendButton stops the click from also
                triggering this card's <Link> navigation. */}
            <RecommendButton
              bookId={book.id}
              bookTitle={book.title}
              recommended={book.your_recommendation ?? false}
              count={book.recommendation_count ?? 0}
            />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
