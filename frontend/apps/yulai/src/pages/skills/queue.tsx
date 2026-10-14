import type { SkillQueue } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { Progress } from "@xaroth.nl/design/react";
import { Levels } from "./levels";
import {
  activeEntry,
  entryProgress,
  entryRate,
  formatDuration,
  formatSP,
  isPaused,
  pending,
  romanLevel,
  type SkillState,
} from "./training";
import { useNow } from "./use-now";
import styles from "./skills.module.scss";

export function SkillQueuePanel({
  queue,
  names,
  states,
}: {
  queue: SkillQueue[];
  names: Map<number, string>;
  states: Map<number, SkillState>;
}) {
  const now = useNow(1000);
  const entries = pending(queue, now);
  const active = activeEntry(queue, now);
  const paused = isPaused(queue);
  const last = entries[entries.length - 1]?.finishDate;
  const rate = active && entryRate(active);

  return (
    <>
      <div className={styles.head}>
        <h2 className={styles.title}>Skill Queue</h2>
        <span className="small muted">
          {entries.length === 0
            ? "Empty"
            : paused
              ? "Paused"
              : last && `${formatDuration(Date.parse(last) - now)} · ${new Date(last).toLocaleString()}`}
        </span>
      </div>
      <ol className={styles.list}>
        {entries.map((e) => {
          const isActive = e === active;
          const start = e.startDate ? Math.max(Date.parse(e.startDate), now) : undefined;
          return (
            <li key={`${e.skillId}-${e.finishedLevel}`} className={styles.entry} data-active={isActive || undefined}>
              <span className={styles.entryName}>
                {names.get(e.skillId) ?? e.skillId} {romanLevel(e.finishedLevel)}
              </span>
              <Levels state={states.get(e.skillId)} />
              <span className={`small ${styles.entryTime}`}>
                {e.finishDate && start !== undefined ? formatDuration(Date.parse(e.finishDate) - start) : "paused"}
              </span>
              {isActive && (
                <Progress
                  className={styles.entryBar}
                  size="sm"
                  value={entryProgress(e, now) * 100}
                  label={`${names.get(e.skillId) ?? e.skillId} ${romanLevel(e.finishedLevel)} progress`}
                />
              )}
            </li>
          );
        })}
      </ol>
      {rate !== undefined && <div className={`small muted ${styles.foot}`}>Training at {formatSP(rate)} SP/hour</div>}
    </>
  );
}
