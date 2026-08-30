"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { BookDown, BookUp } from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { BorrowingTab } from "./BorrowingTab";
import { LendingTab } from "./LendingTab";

export default function LoansPage() {
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) router.push("/login");
  }, [router]);

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-bold">Loans</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Everything you&apos;ve borrowed and everything you&apos;ve lent
        </p>
      </div>

      <Tabs defaultValue="borrowing">
        <TabsList>
          <TabsTrigger value="borrowing">
            <BookDown className="size-4" />
            Borrowing
          </TabsTrigger>
          <TabsTrigger value="lending">
            <BookUp className="size-4" />
            Lending
          </TabsTrigger>
        </TabsList>

        <TabsContent value="borrowing" className="flex flex-col gap-6">
          <BorrowingTab />
        </TabsContent>
        <TabsContent value="lending" className="flex flex-col gap-6">
          <LendingTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
