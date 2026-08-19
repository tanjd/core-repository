import { createRequire } from "module";
import { composePlugins, withNx } from "@nx/next";
import type { WithNxOptions } from "@nx/next/plugins/with-nx";

const require = createRequire(import.meta.url);
const { version } = require("./package.json") as { version: string };

const nextConfig: WithNxOptions = {
  nx: {},
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "covers.openlibrary.org",
      },
      {
        protocol: "https",
        hostname: "books.google.com",
      },
    ],
  },
  // Required for the Docker image — generates .next/standalone + server.js
  output: "standalone",
  // API proxy is handled by src/app/api/[...path]/route.ts so that
  // BACKEND_URL is read at request time (runtime), not baked at build time.
  env: {
    NEXT_PUBLIC_VERSION: version,
  },
  // X-Frame-Options, X-Content-Type-Options, and Referrer-Policy are set by
  // Traefik's headers middleware in compose/docker-compose.bookshelf.yml
  // (which sits downstream of this origin in the response path, so app-layer
  // values here would just be overridden) — CSP has no simple Traefik
  // equivalent, so it stays here as the single source of truth.
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              // Cover images from the two metadata sources + Next/Image blur placeholders.
              "img-src 'self' https://covers.openlibrary.org https://books.google.com data:",
              // Next.js/Tailwind inline <style> injection needs 'unsafe-inline' short of a
              // nonce-based CSP — a deliberate starting point, not the end state.
              "style-src 'self' 'unsafe-inline'",
              // 'unsafe-inline' verified necessary (not just a precaution): App Router
              // embeds per-page inline <script> tags carrying RSC flight data
              // (self.__next_f.push(...)), a distinct hash every build/route — confirmed
              // via a Playwright run against the production standalone build that a
              // strict 'self'-only script-src blocks hydration on every route tested
              // (/, /login, /setup, /catalog, /register). Nonce-based CSP would remove
              // this relaxation but needs per-request middleware not present here.
              // 'unsafe-eval' is added in dev only — next dev's webpack HMR/react-refresh
              // runtime evals code to apply updates; the production standalone build
              // doesn't need it.
              `script-src 'self' 'unsafe-inline'${process.env.NODE_ENV === "production" ? "" : " 'unsafe-eval'"}`,
              // All backend calls are same-origin via src/app/api/[...path]/route.ts.
              "connect-src 'self'",
              "frame-ancestors 'none'",
              "base-uri 'self'",
              "form-action 'self'",
            ].join("; "),
          },
        ],
      },
    ];
  },
};

const plugins = [withNx];

export default composePlugins(...plugins)(nextConfig);
