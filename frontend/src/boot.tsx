import { StrictMode, type ComponentType } from "react";
import { createRoot } from "react-dom/client";
import { Theme } from "@astryxdesign/core/theme";
import { neutralTheme } from "@astryxdesign/theme-neutral/built";
import { InternationalizationProvider } from "@astryxdesign/core/i18n";
import esES from "@astryxdesign/core/locales/es-ES.generated.js";

// Lo que Go escribe en <script id="page-data">: qué página pintar y sus datos ya formateados.
export type PageData<T = unknown> = { page: string; data: T };

// mount pinta la página que pide el servidor. Go ya validó y formateó todo; aquí solo se presenta.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function mount(pages: Record<string, ComponentType<any>>, mode: "system" | "light" = "system") {
  const el = document.getElementById("page-data");
  const root = document.getElementById("root");
  if (!el || !root) return;
  const { page, data } = JSON.parse(el.textContent || "{}") as PageData;
  const Page = pages[page];
  if (!Page) throw new Error(`página desconocida: ${page}`);
  createRoot(root).render(
    <StrictMode>
      <InternationalizationProvider locale="es-MX" messages={{ "es-MX": esES }}>
        <Theme theme={neutralTheme} mode={mode}>
          <Page {...(data as object)} />
        </Theme>
      </InternationalizationProvider>
    </StrictMode>,
  );
}
