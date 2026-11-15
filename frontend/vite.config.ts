import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    // In dev the Go server runs separately (docker compose, port 8080).
    // Proxying keeps API calls same-origin, as they are in production.
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
