import type { Feature } from "@yulai/ui";

// Rendered by identity/login into the #page-data element. Keep in sync with its pageData types.
export type PageData =
  | { page: "picker"; csrf: string; features: Feature[] }
  | { page: "done"; id: number; name: string; csrf: string; features: string[] }
  | { page: "error"; message: string };

// Vite dev has no server behind it: ?page=done or ?page=error shows those pages.
const devPages: Record<string, PageData> = {
  picker: { page: "picker", csrf: "dev", features: [{ name: "Assets", scopes: ["esi-assets.read_assets.v1"] }] },
  done: { page: "done", id: 2112625428, name: "Dev Pilot", csrf: "dev", features: ["Assets"] },
  error: { page: "error", message: "The login was cancelled." },
};

export function readPageData(): PageData {
  const text = document.getElementById("page-data")?.textContent;
  if (text) return JSON.parse(text);
  if (import.meta.env.DEV) return devPages[new URLSearchParams(location.search).get("page") ?? "picker"] ?? devPages.picker;
  return { page: "error", message: "This page has no data. Start again from Yulai." };
}
