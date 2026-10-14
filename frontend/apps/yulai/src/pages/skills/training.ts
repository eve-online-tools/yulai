import type { Skill, SkillQueue } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";

export const maxLevel = 5;

const roman = ["0", "I", "II", "III", "IV", "V"];
export const romanLevel = (level: number) => roman[level] ?? String(level);

// Skill points needed to reach a level: 250 × rank × √32^(level-1).
export function levelSP(rank: number, level: number) {
  if (level <= 0) return 0;
  // The epsilon keeps float error in √32^n from rounding exact values up.
  return Math.ceil(250 * rank * Math.pow(Math.sqrt(32), level - 1) - 1e-6);
}

// Per skill: its level now and the levels still to come from the queue.
export type SkillState = {
  level: number;
  sp: number;
  injected: boolean;
  training?: number;
  queued: number[];
};

// The stored skills lag the queue until the next fetch, so levels whose finish date has passed count as trained.
export function skillStates(skills: Skill[], queue: SkillQueue[], now: number) {
  const states = new Map<number, SkillState>();
  for (const s of skills) {
    states.set(s.skillId, { level: s.activeSkillLevel, sp: s.skillpointsInSkill, injected: true, queued: [] });
  }
  const active = activeEntry(queue, now);
  for (const e of queue) {
    const st = states.get(e.skillId) ?? { level: 0, sp: 0, injected: false, queued: [] };
    states.set(e.skillId, st);
    if (e.finishDate && Date.parse(e.finishDate) <= now) {
      st.level = Math.max(st.level, e.finishedLevel);
      if (e.levelEndSp != null) st.sp = Math.max(st.sp, e.levelEndSp);
    } else if (e === active) {
      st.training = e.finishedLevel;
    } else {
      st.queued.push(e.finishedLevel);
    }
  }
  return states;
}

// The entry training now: started and not yet finished. A paused queue has no dates.
export function activeEntry(queue: SkillQueue[], now: number) {
  return queue.find((e) => e.startDate && e.finishDate && Date.parse(e.startDate) <= now && Date.parse(e.finishDate) > now);
}

export const isPaused = (queue: SkillQueue[]) => queue.length > 0 && queue.every((e) => !e.finishDate);

// Entries not yet finished, in order.
export const pending = (queue: SkillQueue[], now: number) =>
  queue.filter((e) => !e.finishDate || Date.parse(e.finishDate) > now);

// Fraction of the level done at now, from the SP at the start of the run interpolated to the level's end.
export function entryProgress(e: SkillQueue, now: number) {
  if (e.levelStartSp == null || e.levelEndSp == null || e.levelEndSp <= e.levelStartSp) return 0;
  const sp = entrySP(e, now) ?? e.levelStartSp;
  return Math.min(1, Math.max(0, (sp - e.levelStartSp) / (e.levelEndSp - e.levelStartSp)));
}

function entrySP(e: SkillQueue, now: number) {
  if (!e.startDate || !e.finishDate || e.levelEndSp == null) return undefined;
  const from = e.trainingStartSp ?? e.levelStartSp ?? 0;
  const start = Date.parse(e.startDate);
  const end = Date.parse(e.finishDate);
  if (now <= start) return from;
  if (now >= end) return e.levelEndSp;
  return from + ((e.levelEndSp - from) * (now - start)) / (end - start);
}

// SP per hour of an entry with dates, from its own SP and duration, so implants and boosters are included.
export function entryRate(e: SkillQueue) {
  if (!e.startDate || !e.finishDate || e.levelEndSp == null) return undefined;
  const from = e.trainingStartSp ?? e.levelStartSp;
  if (from == null) return undefined;
  const hours = (Date.parse(e.finishDate) - Date.parse(e.startDate)) / 3_600_000;
  return hours > 0 ? (e.levelEndSp - from) / hours : undefined;
}

// Largest three units, like the client: "2d 3h 15m".
export function formatDuration(ms: number) {
  const s = Math.max(0, Math.floor(ms / 1000));
  const parts = [
    [Math.floor(s / 86400), "d"],
    [Math.floor(s / 3600) % 24, "h"],
    [Math.floor(s / 60) % 60, "m"],
    [s % 60, "s"],
  ] as const;
  const first = parts.findIndex(([n]) => n > 0);
  if (first < 0) return "0s";
  return parts
    .slice(first, first + 3)
    .filter(([n]) => n > 0)
    .map(([n, u]) => `${n}${u}`)
    .join(" ");
}

const number = new Intl.NumberFormat();
export const formatSP = (sp: number) => number.format(Math.floor(sp));
