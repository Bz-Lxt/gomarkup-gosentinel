import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:31482",
      "/ws": { target: "ws://127.0.0.1:31482", ws: true },
      "/healthz": "http://127.0.0.1:31482",
    },
  },
  test: {
    environment: "node",
  },
});
