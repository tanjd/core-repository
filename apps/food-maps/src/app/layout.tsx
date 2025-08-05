import "./global.css";

export const metadata = {
  title: "Food Maps",
  description: "My personal food location maps from Google Maps",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-gray-50">
        <nav className="bg-white shadow-sm">
          <div className="container mx-auto px-4 py-4">
            <a href="/" className="text-xl font-semibold text-gray-900">
              Food Maps
            </a>
          </div>
        </nav>
        {children}
      </body>
    </html>
  );
}
