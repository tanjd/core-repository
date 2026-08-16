import Link from "next/link";
import Image from "next/image";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { Book } from "@/lib/types";

interface BookCardProps {
  book: Book;
}

export function BookCard({ book }: BookCardProps) {
  return (
    <Link href={`/catalog/${book.id}`} className="block group">
      <Card className="h-full overflow-hidden transition-shadow group-hover:shadow-md py-0 gap-0">
        <div className="relative aspect-[2/3] w-full bg-muted overflow-hidden">
          {book.cover_url ? (
            <Image
              src={book.cover_url}
              alt={`Cover of ${book.title}`}
              fill
              className="object-cover transition-transform group-hover:scale-105"
              sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 20vw"
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground text-sm px-4 text-center">
              No cover available
            </div>
          )}
        </div>
        <CardContent className="px-3 py-3 flex flex-col gap-1">
          <p className="font-medium text-sm leading-snug line-clamp-2">
            {book.title}
          </p>
          {book.author && (
            <p className="text-xs text-muted-foreground line-clamp-1">
              {book.author}
            </p>
          )}
          {typeof book.available_copies === "number" && (
            <div className="mt-1">
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
