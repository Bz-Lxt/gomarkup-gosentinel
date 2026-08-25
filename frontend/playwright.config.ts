import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  use: {
    baseURL: process.env.E2E_BASE || "http://127.0.0.1:31481",
    viewport: { width: 1280, height: 800 },
  },
  retries: 0,
  timeout: 30_000,
});
