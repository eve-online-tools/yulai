import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi } from "@tanstack/react-router";
import type { SkillGroup, SkillType } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { Alert, EmptyState, Input, PageHead, Panel, Select } from "@xaroth.nl/design/react";
import { AppIcon } from "@yulai/ui";
import { skillsFeature, useFeature } from "../../features";
import { skillCatalogQuery, skillQueueQuery, skillTotalsQuery, skillsQuery } from "../../queries";
import { Levels } from "./levels";
import { SkillQueuePanel } from "./queue";
import { formatSP, levelSP, maxLevel, romanLevel, skillStates, type SkillState } from "./training";
import { useNow } from "./use-now";
import styles from "./skills.module.scss";

const route = getRouteApi("/layout/skills");

export function SkillsPage() {
  const { name: skillsName, characters } = useFeature(skillsFeature);
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const character = characters.find((c) => c.id === search.character) ?? characters[0];

  if (!character) {
    return (
      <>
        <PageHead title="Skills" />
        <EmptyState icon={<AppIcon name="school" size={32} />} title={`No character has ${skillsName}`}>
          Log in again from the characters page with it checked.
        </EmptyState>
      </>
    );
  }

  return (
    <>
      <PageHead
        title="Skills"
        meta={<Totals characterID={character.id} />}
        actions={
          <Select
            aria-label="Character"
            value={character.id}
            onChange={(e) => navigate({ search: (s) => ({ ...s, character: Number(e.target.value) }) })}
          >
            {characters.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        }
      />
      <Skills
        characterID={character.id}
        groupID={search.group}
        onGroup={(group) => navigate({ search: (s) => ({ ...s, group }) })}
      />
    </>
  );
}

function Totals({ characterID }: { characterID: number }) {
  const { data: totals } = useQuery(skillTotalsQuery(characterID));
  if (!totals) return null;
  return (
    <span className="muted">
      {formatSP(totals.totalSp)} SP
      {!!totals.unallocatedSp && ` · ${formatSP(totals.unallocatedSp)} unallocated`}
    </span>
  );
}

function Skills({
  characterID,
  groupID,
  onGroup,
}: {
  characterID: number;
  groupID?: number;
  onGroup: (id: number) => void;
}) {
  const catalog = useQuery(skillCatalogQuery);
  const { data: trained = [] } = useQuery(skillsQuery(characterID));
  const { data: queue = [] } = useQuery(skillQueueQuery(characterID));
  // Coarse: only moves levels that finished since the last fetch. The queue ticks on its own.
  const now = useNow(60_000);
  const states = useMemo(() => skillStates(trained, queue, now), [trained, queue, now]);
  const [filter, setFilter] = useState("");

  const groups = catalog.data?.groups ?? [];
  const group = groups.find((g) => g.id === groupID) ?? groups[0];
  const names = useMemo(() => new Map(groups.flatMap((g) => g.skills.map((s) => [s.id, s.name] as const))), [groups]);

  const term = filter.trim().toLowerCase();
  const shown = term
    ? groups.flatMap((g) => g.skills.filter((s) => s.name.toLowerCase().includes(term)))
    : (group?.skills ?? []);

  return (
    <div className={styles.layout}>
      <div className={styles.grid}>
        <Panel className={styles.groups} padding="sm" aria-label="Skill groups">
          {catalog.isError && <Alert tone="danger">{String(catalog.error)}</Alert>}
          <div className={styles.tiles}>
            {groups.map((g) => (
              <GroupTile
                key={g.id}
                group={g}
                states={states}
                selected={!term && g.id === group?.id}
                onSelect={() => {
                  setFilter("");
                  onGroup(g.id);
                }}
              />
            ))}
          </div>
        </Panel>

        <Panel className={styles.skills} padding="none" aria-label="Skills">
          <div className={styles.head}>
            <h2 className={styles.title}>{term ? "Search" : group?.name}</h2>
            <Input
              type="search"
              placeholder="Search skills"
              aria-label="Search skills"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className={styles.search}
            />
          </div>
          <ul className={styles.list}>
            {shown.map((s) => (
              <SkillRow key={s.id} skill={s} state={states.get(s.id)} states={states} names={names} />
            ))}
            {term && shown.length === 0 && <li className={`muted ${styles.empty}`}>No skills match.</li>}
          </ul>
        </Panel>

        <Panel className={styles.queue} padding="none" aria-label="Skill queue">
          <SkillQueuePanel queue={queue} names={names} states={states} />
        </Panel>
      </div>
    </div>
  );
}

// Like the client's group tiles: name, skills trained of the group and SP towards all of it at V.
function GroupTile({
  group,
  states,
  selected,
  onSelect,
}: {
  group: SkillGroup;
  states: Map<number, SkillState>;
  selected: boolean;
  onSelect: () => void;
}) {
  let injected = 0;
  let sp = 0;
  let max = 0;
  let training = false;
  for (const s of group.skills) {
    const st = states.get(s.id);
    if (st?.injected) injected++;
    if (st?.training) training = true;
    sp += st?.sp ?? 0;
    max += levelSP(s.rank, maxLevel);
  }
  return (
    <button
      type="button"
      className={styles.tile}
      aria-pressed={selected}
      data-training={training || undefined}
      onClick={onSelect}
    >
      <span className={styles.tileName}>{group.name}</span>
      <span className={styles.tileCount}>
        {injected}/{group.skills.length}
      </span>
      <span className={styles.tileBar} style={{ "--fill": max ? sp / max : 0 } as React.CSSProperties} />
    </button>
  );
}

function SkillRow({
  skill,
  state,
  states,
  names,
}: {
  skill: SkillType;
  state?: SkillState;
  states: Map<number, SkillState>;
  names: Map<number, string>;
}) {
  const [open, setOpen] = useState(false);
  const level = state?.level ?? 0;
  const next = Math.min(level + 1, maxLevel);
  return (
    <li className={styles.skill} data-injected={state?.injected || undefined} data-open={open || undefined}>
      <button type="button" className={styles.skillRow} aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={styles.skillName}>
          {skill.name} <span className="muted small">({skill.rank}x)</span>
        </span>
        <Levels state={state} />
        <span className={`small ${styles.skillSP}`}>
          {state?.injected ? (
            <>
              {formatSP(state.sp)} / {formatSP(levelSP(skill.rank, next))}
            </>
          ) : (
            <span className="muted">Not injected</span>
          )}
        </span>
      </button>
      {open && (
        <div className={styles.details}>
          {skill.description && <p className={styles.description}>{skill.description}</p>}
          <dl className={`small ${styles.facts}`}>
            {skill.primary && (
              <>
                <dt>Attributes</dt>
                <dd className={styles.attrs}>
                  {skill.primary} / {skill.secondary}
                </dd>
              </>
            )}
            <dt>Level</dt>
            <dd>
              {level ? romanLevel(level) : "Untrained"}
              {state?.training ? `, training ${romanLevel(state.training)}` : ""}
            </dd>
            {skill.requires.length > 0 && (
              <>
                <dt>Requires</dt>
                <dd>
                  <ul className={styles.requires}>
                    {skill.requires.map((r) => {
                      const met = (states.get(r.skillId)?.level ?? 0) >= r.level;
                      return (
                        <li key={r.skillId} className={met ? undefined : "warn"}>
                          {names.get(r.skillId) ?? r.skillId} {romanLevel(r.level)}
                        </li>
                      );
                    })}
                  </ul>
                </dd>
              </>
            )}
          </dl>
        </div>
      )}
    </li>
  );
}
