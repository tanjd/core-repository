"use client";

import { useState } from "react";
import { ArrowUpDown } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import type { PositionItem } from "@/lib/types";
import { fmtUsd, fmtQty, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

type SortKey = keyof PositionItem;
type Dir = "asc" | "desc";

interface SortHeaderProps {
  col: SortKey;
  label: string;
  active: boolean;
  onSort: (col: SortKey) => void;
}

function SortHeader({ col, label, active, onSort }: SortHeaderProps) {
  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        "-ml-3 h-7 gap-1 text-xs font-medium",
        active && "text-foreground",
      )}
      onClick={() => onSort(col)}
    >
      {label}
      <ArrowUpDown className="h-3 w-3" />
    </Button>
  );
}

interface Props {
  positions: PositionItem[];
}

export function HoldingsTable({ positions }: Props) {
  const [sortKey, setSortKey] = useState<SortKey>("current_value");
  const [dir, setDir] = useState<Dir>("desc");

  const totals = {
    cost_basis: positions.reduce((s, p) => s + p.cost_basis, 0),
    current_value: positions.reduce((s, p) => s + p.current_value, 0),
    unrealized_pnl: positions.reduce((s, p) => s + p.unrealized_pnl, 0),
  };

  const sorted = [...positions].sort((a, b) => {
    const av = a[sortKey] as number | string;
    const bv = b[sortKey] as number | string;
    const cmp =
      typeof av === "string"
        ? av.localeCompare(bv as string)
        : (av as number) - (bv as number);
    return dir === "asc" ? cmp : -cmp;
  });

  const handleSort = (key: SortKey) => {
    if (key === sortKey) setDir((d) => (d === "asc" ? "desc" : "asc"));
    else {
      setSortKey(key);
      setDir("desc");
    }
  };

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>
              <SortHeader
                col="symbol"
                label="Symbol"
                active={sortKey === "symbol"}
                onSort={handleSort}
              />
            </TableHead>
            <TableHead className="hidden md:table-cell">Description</TableHead>
            <TableHead className="text-right">
              <SortHeader
                col="quantity"
                label="Qty"
                active={sortKey === "quantity"}
                onSort={handleSort}
              />
            </TableHead>
            <TableHead className="text-right">
              <SortHeader
                col="cost_price"
                label="Avg Cost"
                active={sortKey === "cost_price"}
                onSort={handleSort}
              />
            </TableHead>
            <TableHead className="text-right">
              <SortHeader
                col="close_price"
                label="Close"
                active={sortKey === "close_price"}
                onSort={handleSort}
              />
            </TableHead>
            <TableHead className="text-right">
              <SortHeader
                col="current_value"
                label="Market Value"
                active={sortKey === "current_value"}
                onSort={handleSort}
              />
            </TableHead>
            <TableHead className="text-right">
              <SortHeader
                col="unrealized_pnl"
                label="Unrealized P&L"
                active={sortKey === "unrealized_pnl"}
                onSort={handleSort}
              />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((pos) => {
            const pnlPct =
              pos.cost_basis !== 0
                ? (pos.unrealized_pnl / pos.cost_basis) * 100
                : 0;
            return (
              <TableRow key={`${pos.broker}-${pos.symbol}`}>
                <TableCell className="font-medium">{pos.symbol}</TableCell>
                <TableCell className="hidden max-w-[180px] truncate text-muted-foreground md:table-cell">
                  {pos.description}
                </TableCell>
                <TableCell className="text-right font-mono">
                  {fmtQty(pos.quantity)}
                </TableCell>
                <TableCell className="text-right font-mono">
                  {fmtUsd(pos.cost_price)}
                </TableCell>
                <TableCell className="text-right font-mono">
                  {fmtUsd(pos.close_price)}
                </TableCell>
                <TableCell className="text-right font-mono">
                  {fmtUsd(pos.current_value)}
                </TableCell>
                <TableCell
                  className={cn(
                    "text-right font-mono",
                    pnlColor(pos.unrealized_pnl),
                  )}
                >
                  {fmtUsd(pos.unrealized_pnl)}
                  <span className="ml-1 text-xs">
                    ({pnlPct >= 0 ? "+" : ""}
                    {pnlPct.toFixed(1)}%)
                  </span>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
        <TableFooter>
          <TableRow>
            <TableCell colSpan={5} className="font-semibold">
              Total
            </TableCell>
            <TableCell className="text-right font-mono font-semibold">
              {fmtUsd(totals.current_value)}
            </TableCell>
            <TableCell
              className={cn(
                "text-right font-mono font-semibold",
                pnlColor(totals.unrealized_pnl),
              )}
            >
              {fmtUsd(totals.unrealized_pnl)}
            </TableCell>
          </TableRow>
        </TableFooter>
      </Table>
    </div>
  );
}
