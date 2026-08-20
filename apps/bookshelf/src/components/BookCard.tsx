import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { BookCover } from "@/components/BookCover";
import type { Book } from "@/lib/types";

interface BookCardProps {
  book: Book;
  ownedByMe?: boolean;
}

export function BookCard({ book, ownedByMe }: BookCardProps) {
  return (
    <Link href={`/catalog/${book.id}`} className="block group">
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
          {typeof book.available_copies === "number" && (
            <div className="mt-auto pt-1">
              <Badge
                variant={book.available_copies > 0 ? "success" : "secondary"}
              >
                {book.available_copies > 0
                  ? `${book.available_copies} available`
                  : "Unavailable"}
              </Badge>
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}
