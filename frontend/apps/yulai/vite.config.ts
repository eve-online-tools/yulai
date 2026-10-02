import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// main.go embeds frontend/dist.
const bindings = fileURLToPath(new URL("./bindings", import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: {
    alias: { "@bindings": bindings },
  },
  build: {
    outDir: "../../dist",
    emptyOutDir: true,
  },
  plugins: [react(), wails("@bindings")],
});
