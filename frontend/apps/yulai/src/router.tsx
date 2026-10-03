import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";
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

const settingsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/settings",
  component: SettingsPage,
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([welcomeRoute, layoutRoute.addChildren([indexRoute, overviewRoute, charactersRoute, settingsRoute])]),
  history: createHashHistory(),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
