import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
} from "@tanstack/react-router";
import type { FactionIdentifier } from "@eve-online-tools/eve-ship-tree";
import { WindowFrame } from "./components/window-frame";
import { Shell } from "./components/shell";
import { OverviewPage } from "./pages/overview";
import { CharactersPage } from "./pages/characters";
import { WelcomePage } from "./pages/welcome";
import { SettingsPage } from "./pages/settings";
import { charactersQuery, featuresQuery, queryClient, setupQuery, syncJobsQuery } from "./queries";

// Hash history: Wails serves one index.html. Every page sits in the frame (title bar, window controls).
const rootRoute = createRootRoute({ component: WindowFrame });

// Tool pages share the sidebar shell.
const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "layout",
  // The sidebar greys out entries by character features; loading them first avoids a flash of disabled entries.
  loader: () => Promise.all([queryClient.ensureQueryData(charactersQuery), queryClient.ensureQueryData(featuresQuery)]),
  component: Shell,
});

const indexRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/",
  beforeLoad: async () => {
    const characters = await queryClient.ensureQueryData(charactersQuery);
    throw redirect({ to: characters.length === 0 ? "/welcome" : "/overview" });
  },
});

// First run. Skips the shell so nothing else competes with the blocking steps.
const welcomeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/welcome",
  loader: () =>
    Promise.all([
      queryClient.ensureQueryData(setupQuery),
      queryClient.ensureQueryData(charactersQuery),
      queryClient.ensureQueryData(syncJobsQuery),
    ]),
  component: WelcomePage,
});

const overviewRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/overview",
  loader: () => queryClient.ensureQueryData(charactersQuery),
  component: OverviewPage,
});

const charactersRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/characters",
  loader: () => Promise.all([queryClient.ensureQueryData(charactersQuery), queryClient.ensureQueryData(featuresQuery)]),
  component: CharactersPage,
});

const shipTreeRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/ship-tree",
  validateSearch: (search: Record<string, unknown>): { character?: number; faction?: FactionIdentifier } => ({
    character: typeof search.character === "number" ? search.character : undefined,
    faction: typeof search.faction === "number" ? (search.faction as FactionIdentifier) : undefined,
  }),
  // Split out: the ship tree's art is most of the bundle.
  component: lazyRouteComponent(() => import("./pages/ship-tree"), "ShipTreePage"),
});

const settingsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/settings",
  component: SettingsPage,
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([welcomeRoute, layoutRoute.addChildren([indexRoute, overviewRoute, charactersRoute, shipTreeRoute, settingsRoute])]),
  history: createHashHistory(),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
