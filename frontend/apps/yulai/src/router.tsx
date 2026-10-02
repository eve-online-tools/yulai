import {
  createHashHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { Root } from "./pages/Root";
import { CharactersPage } from "./pages/Characters";
import { AccountsPage } from "./pages/Accounts";
import { WelcomePage } from "./pages/Welcome";
import { charactersQuery, featuresQuery, queryClient, setupQuery } from "./queries";

// Hash history: Wails serves one index.html.
const rootRoute = createRootRoute({ component: Outlet });

// Main window pages share the header chrome.
const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "layout",
  component: Root,
});

const indexRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/",
  beforeLoad: async () => {
    const characters = await queryClient.ensureQueryData(charactersQuery);
    throw redirect({ to: characters.length === 0 ? "/welcome" : "/characters" });
  },
});

// First run. Chromeless so nothing else competes with the blocking steps.
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

export const router = createRouter({
  routeTree: rootRoute.addChildren([welcomeRoute, layoutRoute.addChildren([indexRoute, charactersRoute, accountsRoute])]),
  history: createHashHistory(),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
