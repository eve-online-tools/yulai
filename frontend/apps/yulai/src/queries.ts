import { QueryClient, queryOptions } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as Sync } from "@bindings/github.com/eve-online-tools/yulai/feature/sync";
import { SetupService as Setup } from "@bindings/github.com/eve-online-tools/yulai/app";
import { Service as SDE } from "@bindings/github.com/eve-online-tools/yulai/core/sde";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

// One key per backend list. Features add their own here alongside a query and an event.
export const keys = {
  characters: ["characters"] as const,
  features: ["features"] as const,
  syncJobs: ["syncJobs"] as const,
  setup: ["setup"] as const,
  sde: ["sde"] as const,
};

export const charactersQuery = queryOptions({
  queryKey: keys.characters,
  queryFn: () => Characters.List(),
});

export const featuresQuery = queryOptions({
  queryKey: keys.features,
  queryFn: () => Characters.Features(),
  staleTime: Infinity,
});

// Config is read once at boot, so this never changes while the app runs.
export const setupQuery = queryOptions({
  queryKey: keys.setup,
  queryFn: () => Setup.Status(),
  staleTime: Infinity,
});

export const syncJobsQuery = queryOptions({
  queryKey: keys.syncJobs,
  queryFn: () => Sync.List(),
});

export const sdeQuery = queryOptions({
  queryKey: keys.sde,
  queryFn: () => SDE.Status(),
});

// Backend emits events when data changes; we invalidate the matching queries.
export function listenForBackendEvents() {
  Events.On("character:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.characters });
    queryClient.invalidateQueries({ queryKey: keys.syncJobs });
  });
  Events.On("sync:jobs:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.syncJobs });
  });
  // task:changed also carries progress, at most every 250ms.
  Events.On("sde:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.sde });
  });
  Events.On("task:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.sde });
  });
}
