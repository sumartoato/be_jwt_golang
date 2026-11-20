import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In dev, proxy /api to the Go backend so the browser sees a single origin.
// The build time, shown in the sidebar so a stale browser tab is easy to spot (Asia/Jakarta).
const buildTime = new Date().toLocaleString("sv-SE", { timeZone: "Asia/Jakarta" }).slice(0, 16);

export default defineConfig({
  plugins: [react()],
  define: { __BUILD_TIME__: JSON.stringify(buildTime) },
  server: {
    port: 5173,
    proxy: {
      "/api": process.env.VITE_API_PROXY ?? "http://localhost:8080",
    },
  },
});
