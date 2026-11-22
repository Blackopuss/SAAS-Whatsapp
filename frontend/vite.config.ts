import { defineConfig } from "vite";

// Tres entradas (tienda, panel, superadmin) + una hoja de estilos común. Go sirve web/static/app.
// shortcut: nombres fijos sin hash (Go no lee manifiesto); pasar a manifest + cache immutable si el tráfico lo pide.
export default defineConfig({
  root: ".",
  base: "/static/app/",
  publicDir: false,
  oxc: { jsx: { runtime: "automatic" } },
  build: {
    outDir: "../web/static/app",
    emptyOutDir: true,
    target: "es2022",
    cssCodeSplit: true,
    rollupOptions: {
      // "use client" de las librerías no aplica: no hay render en servidor.
      onLog(level, log, handler) {
        if (log.code !== "MODULE_LEVEL_DIRECTIVE") handler(level, log);
      },
      input: {
        styles: "src/styles.css",
        shop: "src/shop.tsx",
        panel: "src/panel.tsx",
        admin: "src/admin.tsx",
        ...Object.fromEntries(
          ["vitrina", "boutique", "barberia", "carniceria", "abarrotes", "papeleria"].map((s) => [`theme-${s}`, `src/themes/${s}/${s}.css`]),
        ),
      },
      output: {
        entryFileNames: "[name].js",
        chunkFileNames: "[name].js",
        assetFileNames: "[name][extname]",
      },
    },
  },
});
