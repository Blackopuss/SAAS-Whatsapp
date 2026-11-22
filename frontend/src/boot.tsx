import { StrictMode, type ComponentType } from "react";
import { createRoot } from "react-dom/client";
import { Theme, type DefinedTheme } from "@astryxdesign/core/theme";
import { InternationalizationProvider } from "@astryxdesign/core/i18n";
import esES from "@astryxdesign/core/locales/es-ES.generated.js";

// Lo que Go escribe en <script id="page-data">: qué página pintar y sus datos ya formateados.
export type PageData<T = unknown> = { page: string; data: T };

export function pageData(): PageData<any> {
  const el = document.getElementById("page-data");
  return JSON.parse(el?.textContent || "{}");
}

// mount pinta la página que pide el servidor. Go ya validó y formateó todo; aquí solo se presenta.
// Cada entrada trae su tema (el panel el neutral, la tienda el de su estilo) para no cargar los demás.
export function mount(pages: Record<string, ComponentType<any>>, theme: DefinedTheme, mode: "system" | "light" | "dark" = "system") {
  const root = document.getElementById("root");
  if (!root) return;
  const { page, data } = pageData();
  const Page = pages[page];
  if (!Page) throw new Error(`página desconocida: ${page}`);
  createRoot(root).render(
    <StrictMode>
      <InternationalizationProvider locale="es-MX" messages={{ "es-MX": esES }}>
        <Theme theme={theme} mode={mode}>
          <Page {...(data as object)} />
        </Theme>
      </InternationalizationProvider>
    </StrictMode>,
  );
}
