/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// ditty's UI is served under a configurable --base-path (default "/") and
// talks to the Session's WebSocket at "<base-path>ws" (SPEC.md §4). In dev,
// proxy both the WS upgrade and the (future) REST API to a real backend —
// a fixture Hub started with `ditty run --fixture <name>` (SPEC.md §12 M3)
// — so `npm run dev` behaves like the embedded UI without a build step.
const DEV_BACKEND = process.env.DITTY_DEV_BACKEND ?? "http://127.0.0.1:7654";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/ws": { target: DEV_BACKEND, ws: true },
      "/api": { target: DEV_BACKEND },
      "/healthz": { target: DEV_BACKEND },
    },
  },
  build: {
    outDir: "dist",
    // No sourcemaps in the embedded build: they'd count toward the §10.4
    // bundle budget despite never being served, and embed.FS ships
    // everything under dist verbatim.
    sourcemap: false,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
});
