"use client";

import { useCallback, useState } from "react";
import { LineChart, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { uploadBenchmark } from "@/lib/api";
import { revalidateAll } from "@/hooks/useStatement";

export function BenchmarkUploadDialog() {
  const [open, setOpen] = useState(false);
  const [symbol, setSymbol] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);

  const reset = useCallback(() => {
    setSymbol("");
    setFile(null);
    setUploading(false);
  }, []);

  const handleOpenChange = useCallback(
    (val: boolean) => {
      setOpen(val);
      if (!val) reset();
    },
    [reset],
  );

  const handleUpload = useCallback(async () => {
    if (!file || !symbol.trim()) return;
    setUploading(true);
    try {
      const result = await uploadBenchmark(symbol.trim(), file);
      toast.success(`Imported ${result.symbol}`, {
        description: `${result.ingested} prices · ${result.first_date} → ${result.last_date}`,
      });
      revalidateAll();
      setOpen(false);
      reset();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }, [file, symbol, reset]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" className="gap-1.5">
          <LineChart className="h-3.5 w-3.5" />
          Add Benchmark
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Benchmark Prices</DialogTitle>
          <DialogDescription>
            Upload a (date, close) CSV for an index — e.g. an S&amp;P 500 export
            from Yahoo Finance or Stooq. Re-uploading the same symbol extends
            its coverage instead of replacing it.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="benchmark-symbol">Symbol</Label>
            <Input
              id="benchmark-symbol"
              placeholder="e.g. SPY"
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              disabled={uploading}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="benchmark-file">CSV file</Label>
            <Input
              id="benchmark-file"
              type="file"
              accept=".csv"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              disabled={uploading}
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={() => setOpen(false)}
            disabled={uploading}
          >
            Cancel
          </Button>
          <Button
            onClick={handleUpload}
            disabled={uploading || !file || !symbol.trim()}
          >
            {uploading ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                Uploading…
              </>
            ) : (
              "Upload"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
