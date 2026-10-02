import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";
import { WindowFrame } from "./components/window-frame";
import { Shell } from "./components/shell";
import { CharactersPage } from "./pages/characters";
import { AccountsPage } from "./pages/accounts";
import { WelcomePage } from "./pages/welcome";
import { SettingsPage } from "./pages/settings";
import { charactersQuery, featuresQuery, queryClient, setupQuery } from "./queries";

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
    throw redirect({ to: characters.length === 0 ? "/welcome" : "/characters" });
  },
});

// First run. Skips the shell so nothing else competes with the blocking steps.
const welcomeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/welcome",
  loader: () => Promise.all([queryClient.ensureQueryData(setupQuery), queryClient.ensureQueryData(charactersQuery)]),
  component: WelcomePage,
});

const charactersRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/characters",
  loader: () => queryClient.ensureQueryData(charactersQuery),
  component: CharactersPage,
});

const accountsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/accounts",
  loader: () => Promise.all([queryClient.ensureQueryData(charactersQuery), queryClient.ensureQueryData(featuresQuery)]),
  component: AccountsPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/settings",
  component: SettingsPage,
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([welcomeRoute, layoutRoute.addChildren([indexRoute, charactersRoute, accountsRoute, settingsRoute])]),
  history: createHashHistory(),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
