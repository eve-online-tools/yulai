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
import { AddPage } from "./pages/Add";
import { charactersQuery, featuresQuery, queryClient } from "./queries";

// Hash history: the add-character window opens at "/#/add" and Wails serves one index.html.
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
  beforeLoad: () => {
    throw redirect({ to: "/characters" });
  },
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

// The add-character window. No chrome, its own small window.
const addRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/add",
  validateSearch: (s: Record<string, unknown>) => ({ error: typeof s.error === "string" ? s.error : undefined }),
  loader: () => queryClient.ensureQueryData(featuresQuery),
  component: AddPage,
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([layoutRoute.addChildren([indexRoute, charactersRoute, accountsRoute]), addRoute]),
  history: createHashHistory(),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
