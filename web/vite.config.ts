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
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
