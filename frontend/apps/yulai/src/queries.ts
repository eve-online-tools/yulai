import { QueryClient, queryOptions } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as Sync } from "@bindings/github.com/eve-online-tools/yulai/feature/sync";
import { ProgressService, SetupService as Setup } from "@bindings/github.com/eve-online-tools/yulai/app";
import type { ProgressEvent } from "@bindings/github.com/eve-online-tools/yulai/core/task";

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
  progress: (key: string) => ["progress", key] as const,
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

// Progress of the running task with that task.WithProgress key, null when it is not running.
// Fetched once, then kept current by the progress events.
export const progressQuery = (key: string) =>
  queryOptions({
    queryKey: keys.progress(key),
    queryFn: () => ProgressService.Get(key),
    staleTime: Infinity,
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
  const setProgress = (ev: ProgressEvent, running: boolean) =>
    queryClient.setQueryData(keys.progress(ev.key), running ? ev.progress : null);
  Events.On("progress:start", (e) => setProgress(e.data, true));
  Events.On("progress:update", (e) => setProgress(e.data, true));
  Events.On("progress:done", (e) => setProgress(e.data, false));
}
