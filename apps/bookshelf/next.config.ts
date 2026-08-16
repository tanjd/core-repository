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
};

const plugins = [withNx];

export default composePlugins(...plugins)(nextConfig);
