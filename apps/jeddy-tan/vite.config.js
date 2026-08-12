/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  root: __dirname,
  cacheDir: "../../node_modules/.vite/apps/jeddy-tan",
  plugins: [react()],
  server: {
    host: true,
  },
  build: {
    outDir: "../../dist/apps/jeddy-tan",
    emptyOutDir: true,
    reportCompressedSize: true,
  },
  test: {
    watch: false,
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/setupTests.js"],
    reporters: ["default"],
    coverage: {
      reportsDirectory: "../../coverage/apps/jeddy-tan",
      provider: "v8",
    },
  },
});
