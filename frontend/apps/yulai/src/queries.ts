import { QueryClient, queryOptions } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as ShipTree } from "@bindings/github.com/eve-online-tools/yulai/feature/shiptree";
import { Service as Skills } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { Service as SDE } from "@bindings/github.com/eve-online-tools/yulai/core/sde";
import { Service as StarMap } from "@bindings/github.com/eve-online-tools/yulai/feature/starmap";
import { Service as Presence } from "@bindings/github.com/eve-online-tools/yulai/feature/presence";
import { Consent } from "@bindings/github.com/eve-online-tools/yulai/identity/login";
import { SetupService as Setup, TaskService as Tasks } from "@bindings/github.com/eve-online-tools/yulai/app";
import type { ProgressEvent, Status } from "@bindings/github.com/eve-online-tools/yulai/core/task";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

// Keys start with the topic the backend sends in data:changed, so a change refetches every query under it.
// A query that also depends on other topics lists them in meta.topics.
export const keys = {
  characters: ["characters"] as const,
  features: ["features"] as const,
  tasks: ["tasks"] as const,
  setup: ["setup"] as const,
  sde: ["sde", "status"] as const,
  map: ["sde", "map"] as const,
  presence: ["presence"] as const,
  progress: (key: string) => ["progress", key] as const,
  skills: (characterID: number) => ["skills", characterID] as const,
  skillQueue: (characterID: number) => ["skills", characterID, "queue"] as const,
  shipTree: ["sde", "shipTree"] as const,
};

export const charactersQuery = queryOptions({
  queryKey: keys.characters,
  queryFn: () => Characters.List(),
});

export const featuresQuery = queryOptions({
  queryKey: keys.features,
  queryFn: () => Consent.Features(),
  staleTime: Infinity,
});

// Config is read once at boot, so this never changes while the app runs.
export const setupQuery = queryOptions({
  queryKey: keys.setup,
  queryFn: () => Setup.Status(),
  staleTime: Infinity,
});

// Status per task and subject, seen this session. Character tasks use subject "char:<id>".
export const tasksQuery = queryOptions({
  queryKey: keys.tasks,
  queryFn: () => Tasks.List(),
});

export const sdeQuery = queryOptions({
  queryKey: keys.sde,
  queryFn: () => SDE.Status(),
});

export const skillsQuery = (characterID: number) =>
  queryOptions({
    queryKey: keys.skills(characterID),
    queryFn: () => Skills.List(characterID),
  });

export const skillQueueQuery = (characterID: number) =>
  queryOptions({
    queryKey: keys.skillQueue(characterID),
    queryFn: () => Skills.Queue(characterID),
  });

export const characterTasks = (tasks: Status[], characterID: number) => tasks.filter((t) => t.subject === `char:${characterID}`);

// Go's zero time (RFC 3339 "0001-01-01T00:00:00Z") means the task has not run this session.
export const hasRun = (t: Status) => !t.lastRun.startsWith("0001-");

// Built from the SDE, so it only changes with it.
export const shipTreeQuery = queryOptions({
  queryKey: keys.shipTree,
  queryFn: () => ShipTree.Data(),
  staleTime: Infinity,
});

// Built from the SDE, so it only changes with it.
export const mapQuery = queryOptions({
  queryKey: keys.map,
  queryFn: () => StarMap.Data(),
  staleTime: Infinity,
});

export const presenceQuery = queryOptions({
  queryKey: keys.presence,
  queryFn: () => Presence.List(),
  // Removing a character drops its presence.
  meta: { topics: ["characters"] },
});

// Progress of the running task with that task.WithProgress key, null when it is not running.
// Fetched once, then kept current by the progress events.
export const progressQuery = (key: string) =>
  queryOptions({
    queryKey: keys.progress(key),
    queryFn: () => Tasks.Progress(key),
    staleTime: Infinity,
  });

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { topics?: string[] };
  }
}

// The backend sends data:changed with a topic when stored data changes; refetch the queries under it.
export function listenForBackendEvents() {
  Events.On("data:changed", (e) => {
    const topic = e.data;
    queryClient.invalidateQueries({
      predicate: (q) => q.queryKey[0] === topic || (q.meta?.topics?.includes(topic) ?? false),
    });
  });
  const setProgress = (ev: ProgressEvent, running: boolean) =>
    queryClient.setQueryData(keys.progress(ev.key), running ? ev.progress : null);
  Events.On("progress:start", (e) => setProgress(e.data, true));
  Events.On("progress:update", (e) => setProgress(e.data, true));
  Events.On("progress:done", (e) => setProgress(e.data, false));
}
