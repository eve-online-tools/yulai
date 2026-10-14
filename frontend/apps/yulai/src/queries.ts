import { QueryClient, queryOptions } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Service as ShipTree } from "@bindings/github.com/eve-online-tools/yulai/feature/shiptree";
import { Service as Skills } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { Service as SDE } from "@bindings/github.com/eve-online-tools/yulai/core/sde";
import { Service as Presence } from "@bindings/github.com/eve-online-tools/yulai/feature/presence";
import { Consent } from "@bindings/github.com/eve-online-tools/yulai/identity/login";
import { SetupService as Setup, TaskService as Tasks } from "@bindings/github.com/eve-online-tools/yulai/app";
import type { ProgressEvent, Status } from "@bindings/github.com/eve-online-tools/yulai/core/task";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

// One key per backend list. Features add their own here alongside a query and an event.
export const keys = {
  characters: ["characters"] as const,
  features: ["features"] as const,
  tasks: ["tasks"] as const,
  setup: ["setup"] as const,
  sde: ["sde"] as const,
  map: ["map"] as const,
  presence: ["presence"] as const,
  progress: (key: string) => ["progress", key] as const,
  allSkills: ["skills"] as const,
  skills: (characterID: number) => ["skills", characterID] as const,
  skillQueue: (characterID: number) => ["skills", characterID, "queue"] as const,
  shipTree: ["shipTree"] as const,
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

// Go's zero time means the task has not run this session.
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
  queryFn: () => SDE.Map(),
  staleTime: Infinity,
});

export const presenceQuery = queryOptions({
  queryKey: keys.presence,
  queryFn: () => Presence.List(),
});

// Progress of the running task with that task.WithProgress key, null when it is not running.
// Fetched once, then kept current by the progress events.
export const progressQuery = (key: string) =>
  queryOptions({
    queryKey: keys.progress(key),
    queryFn: () => Tasks.Progress(key),
    staleTime: Infinity,
  });

// Backend emits events when data changes; we invalidate the matching queries.
export function listenForBackendEvents() {
  Events.On("character:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.characters });
    queryClient.invalidateQueries({ queryKey: keys.presence });
  });
  Events.On("task:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.tasks });
  });
  Events.On("skills:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.allSkills });
  });
  Events.On("sde:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.sde });
    queryClient.invalidateQueries({ queryKey: keys.shipTree });
    queryClient.invalidateQueries({ queryKey: keys.map });
  });
  Events.On("presence:changed", () => {
    queryClient.invalidateQueries({ queryKey: keys.presence });
  });
  const setProgress = (ev: ProgressEvent, running: boolean) =>
    queryClient.setQueryData(keys.progress(ev.key), running ? ev.progress : null);
  Events.On("progress:start", (e) => setProgress(e.data, true));
  Events.On("progress:update", (e) => setProgress(e.data, true));
  Events.On("progress:done", (e) => setProgress(e.data, false));
}
