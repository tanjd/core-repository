import type { LoanRequest } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { BookCover } from "@/components/BookCover";

// A compact glance at one currently-held loan (status "accepted" guaranteed
// by the caller) — cover, title, author, and due date. Full detail (message,
// owner contact) stays in the expandable row of the table below rather than
// being duplicated here.
export function CurrentlyBorrowedCard({ request }: { request: LoanRequest }) {
  const book = request.copy?.book;

  return (
    <Card className="overflow-hidden py-0 gap-0">
      <CardContent className="p-3 flex gap-3">
        <div className="relative w-14 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
          <BookCover
            title={book?.title ?? "Unknown book"}
            author={book?.author}
            coverUrl={book?.cover_url}
            sizes="56px"
          />
        </div>
        <div className="flex flex-col gap-1 min-w-0 flex-1">
          <p className="text-sm font-medium leading-snug line-clamp-2">
            {book?.title ?? "Unknown book"}
          </p>
          {book?.author && (
            <p className="text-xs text-muted-foreground line-clamp-1">
              {book.author}
            </p>
          )}
          <p className="text-xs text-muted-foreground mt-1">
            {request.expected_return_date
              ? `Due ${new Date(request.expected_return_date).toLocaleDateString()}`
              : "No return date agreed"}
          </p>
          {request.copy?.owner?.name && (
            <p className="text-xs text-muted-foreground">
              from {request.copy.owner.name}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
