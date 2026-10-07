import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Build output goes straight into the Go backend's embed directory so
// `npm run build` followed by `go build` produces one self-contained
// binary with no copy step in between.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/platform/embedweb/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": {
        // Not "localhost": Node may resolve that to ::1, and the API binds
        // IPv4 loopback by default.
        target: "http://127.0.0.1:8080",
        // Keep the browser Host/Origin so the API's same-origin check
        // matches Vite's proxy the same way a production single origin does.
        changeOrigin: false,
        ws: true,
      },
    },
  },
});
