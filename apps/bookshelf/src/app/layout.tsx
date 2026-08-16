import type { Metadata } from "next";
import { Geist } from "next/font/google";
import { Toaster } from "sonner";
import "./globals.css";
import { NavBar } from "@/components/layout/NavBar";
import { SetupGuard } from "@/components/auth/SetupGuard";

const geist = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Bookshelf",
  description: "Church community book lending",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body
        className={`${geist.variable} font-sans antialiased`}
        suppressHydrationWarning
      >
        <NavBar />
        <main className="max-w-6xl mx-auto px-4 py-6">
          <SetupGuard>{children}</SetupGuard>
        </main>
        <footer className="text-sm text-muted-foreground text-center py-6 flex items-center justify-center gap-3">
          <span>v{process.env.NEXT_PUBLIC_VERSION}</span>
          <span>·</span>
          <a href="/about" className="hover:underline underline-offset-2">
            About
          </a>
        </footer>
        <Toaster richColors position="bottom-right" />
      </body>
    </html>
  );
}
