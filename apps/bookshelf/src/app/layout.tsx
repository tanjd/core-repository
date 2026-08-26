import type { Metadata } from "next";
import Link from "next/link";
import { Geist } from "next/font/google";
import { Toaster } from "sonner";
import "./globals.css";
import { NavBar } from "@/components/layout/NavBar";
import { SetupGuard } from "@/components/auth/SetupGuard";
import { ThemeProvider } from "@/components/theme-provider";

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
    <html lang="en" suppressHydrationWarning>
      <body
        className={`${geist.variable} font-sans antialiased min-h-screen flex flex-col`}
        suppressHydrationWarning
      >
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <NavBar />
          <main className="flex-1 w-full max-w-6xl mx-auto px-4 py-6 pb-24 md:pb-6">
            <SetupGuard>{children}</SetupGuard>
          </main>
          <footer className="shrink-0 text-sm text-muted-foreground text-center py-6 pb-24 md:pb-6 flex items-center justify-center gap-3">
            <Link
              href="/changelog"
              className="hover:underline underline-offset-2"
            >
              v{process.env.NEXT_PUBLIC_VERSION}
            </Link>
            <span>·</span>
            <a href="/about" className="hover:underline underline-offset-2">
              About
            </a>
          </footer>
          <Toaster richColors position="bottom-right" />
        </ThemeProvider>
      </body>
    </html>
  );
}
