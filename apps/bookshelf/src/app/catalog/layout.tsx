import { AuthGuard } from "@/components/auth/AuthGuard";

export default function CatalogLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AuthGuard>{children}</AuthGuard>;
}
