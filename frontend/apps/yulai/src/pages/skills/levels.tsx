import { maxLevel, romanLevel, type SkillState } from "./training";
import styles from "./skills.module.scss";

// Five boxes like the client: filled when trained, lit when training, outlined when queued.
export function Levels({ state }: { state?: SkillState }) {
  const level = state?.level ?? 0;
  return (
    <span className={styles.levels} role="img" aria-label={`Level ${level} of ${maxLevel}`}>
      {Array.from({ length: maxLevel }, (_, i) => {
        const l = i + 1;
        const kind =
          l <= level ? "trained" : l === state?.training ? "training" : state?.queued.includes(l) ? "queued" : undefined;
        return <span key={l} className={styles.box} data-kind={kind} title={kind && `${romanLevel(l)} ${kind}`} />;
      })}
    </span>
  );
}
