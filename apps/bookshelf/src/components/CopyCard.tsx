import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { Copy } from "@/lib/types";
import { cn } from "@/lib/utils";

interface CopyCardProps {
  copy: Copy;
  actions?: ReactNode;
}

const conditionLabel: Record<Copy["condition"], string> = {
  good: "Good",
  fair: "Fair",
  worn: "Worn",
};

const conditionVariant: Record<
  Copy["condition"],
  "default" | "secondary" | "outline"
> = {
  good: "default",
  fair: "secondary",
  worn: "outline",
};

const statusLabel: Record<Copy["status"], string> = {
  available: "Available",
  requested: "Requested",
  loaned: "On Loan",
  unavailable: "Unavailable",
};

const statusVariant: Record<
  Copy["status"],
  "success" | "secondary" | "destructive" | "outline"
> = {
  available: "success",
  requested: "secondary",
  loaned: "destructive",
  unavailable: "outline",
};

export function CopyCard({ copy, actions }: CopyCardProps) {
  return (
    <Card className={cn("py-4")}>
      <CardContent className="px-4 flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={conditionVariant[copy.condition]}>
            {conditionLabel[copy.condition]}
          </Badge>
          <Badge variant={statusVariant[copy.status]}>
            {statusLabel[copy.status]}
          </Badge>
          {copy.hide_owner ? (
            <span className="text-sm text-muted-foreground">
              Anonymous member
            </span>
          ) : copy.owner?.name ? (
            <span className="text-sm text-muted-foreground">
              Shared by{" "}
              <span className="font-medium text-foreground">
                {copy.owner.name}
              </span>
            </span>
          ) : (
            <span className="text-sm text-muted-foreground">
              Sign in and verify to see who&apos;s sharing
            </span>
          )}
        </div>
        {copy.notes && (
          <p className="text-sm text-muted-foreground italic">{copy.notes}</p>
        )}
        {actions && <div className="flex flex-wrap gap-2 mt-1">{actions}</div>}
      </CardContent>
    </Card>
  );
}
