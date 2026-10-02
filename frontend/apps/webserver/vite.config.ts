import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// main.go embeds frontend/dist; identity/login serves this build.
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: 9246,
    strictPort: true,
  },
  build: {
    outDir: "../../dist/webserver",
    emptyOutDir: true,
    // Serve every asset as a file; identity/login's CSP has no data: fonts.
    assetsInlineLimit: 0,
  },
  plugins: [react()],
});
