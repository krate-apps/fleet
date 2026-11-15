import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// FLEET_UI_BASE must match the public path prefix (e.g. /trevor/fleet/) when
// Fleet is reverse-proxied behind Caddy path mode. Default keeps root installs.
const base = process.env.FLEET_UI_BASE || "/";

export default defineConfig({
  base,
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": process.env.FLEET_DEV_API || "http://127.0.0.1:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsDir: "assets",
  },
});
