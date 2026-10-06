import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi } from "@tanstack/react-router";
import {
  FactionSelector,
  FactionSummary,
  Grid,
  ShipTree,
  TreeDisplay,
  preloadShipTreeSprites,
  shipTreeFactionOrder,
  type FactionIdentifier,
  type PreloadedData,
  type Skills,
  type SkillTraining,
} from "@eve-online-tools/eve-ship-tree";
import "@eve-online-tools/eve-ship-tree/styles.css";
import type { Data as ShipTreeData } from "@bindings/github.com/eve-online-tools/yulai/feature/shiptree";
import type { Skill, SkillQueue } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { selectClass } from "@xaroth.nl/design/parts";
import { Alert, EmptyState, PageHead, Select } from "@xaroth.nl/design/react";
import { AppIcon } from "@yulai/ui";
import { skillsFeature, useCharactersWithFeature } from "../../features";
import { shipTreeQuery, skillQueueQuery, skillsQuery } from "../../queries";
import styles from "./ship-tree.module.scss";

// ShipTree also does this on mount; starting with the page chunk gets the status sprites ready sooner.
void preloadShipTreeSprites();

const route = getRouteApi("/layout/ship-tree");

export function ShipTreePage() {
  const characters = useCharactersWithFeature(skillsFeature);
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const character = characters.find((c) => c.id === search.character) ?? characters[0];
  // The route only checks for a number; the faction list lives in the lazily loaded package.
  const faction =
    search.faction && shipTreeFactionOrder.includes(search.faction) ? search.faction : shipTreeFactionOrder[0];
  const tables = useQuery({ ...shipTreeQuery, select: toData });

  if (!character) {
    return (
      <>
        <PageHead title="Ship Tree" />
        <EmptyState icon={<AppIcon name="ship" size={32} />} title={`No character has ${skillsFeature}`}>
          Log in again from the characters page with it checked.
        </EmptyState>
      </>
    );
  }

  // The tree's own frame carries the "Ship Tree" title, so the page has no head and fills the content pane.
  return (
    <div className={styles.page}>
      {tables.isError ? (
        <Alert tone="danger">{String(tables.error)}</Alert>
      ) : (
        tables.data && <Tree data={tables.data} characterID={character.id} faction={faction} />
      )}
      <FactionPicker
        data={tables.data}
        value={faction}
        onChange={(f) => navigate({ search: (s) => ({ ...s, faction: f }) })}
      />
      <Select
        className={styles.character}
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
    </div>
  );
}

// Sits in the pane's top-left corner, like the client's faction box: a grid of logos over a summary of the selected or
// hovered faction, or a dropdown when the window is too narrow for it.
function FactionPicker({
  data,
  value,
  onChange,
}: {
  data?: PreloadedData;
  value: FactionIdentifier;
  onChange: (f: FactionIdentifier) => void;
}) {
  const [hovered, setHovered] = useState<FactionIdentifier | null>(null);
  return (
    <div className={styles.factions}>
      <div className={styles.box}>
        <FactionSelector value={value} onChange={onChange} onHoverChange={setHovered} label="Faction" />
        <FactionSummary faction={hovered ?? value} data={data} />
      </div>
      <FactionSelector
        className={selectClass({ className: styles.dropdown })}
        variant="compact"
        value={value}
        onChange={onChange}
        label="Faction"
      />
    </div>
  );
}

function Tree({ data, characterID, faction }: { data: PreloadedData; characterID: number; faction: FactionIdentifier }) {
  const { data: trained } = useQuery(skillsQuery(characterID));
  const { data: queue } = useQuery(skillQueueQuery(characterID));
  const skills = useMemo(() => toLevels(trained ?? []), [trained]);
  const training = useMemo(() => inTraining(queue ?? []), [queue]);
  return (
    <ShipTree.Root skills={skills} training={training} faction={faction} data={data} className={styles.tree}>
      <Grid className={styles.grid} disclaimer={null}>
        <TreeDisplay />
      </Grid>
    </ShipTree.Root>
  );
}

// Wails types integer-keyed maps as string-keyed with optional values; the records match the package's tables.
function toData(d: ShipTreeData | null): PreloadedData | undefined {
  return (d ?? undefined) as unknown as PreloadedData | undefined;
}

function toLevels(skills: Skill[]): Skills {
  return Object.fromEntries(skills.map((s) => [s.skillId, s.activeSkillLevel])) as Skills;
}

// The entry training now: started and not yet finished. A paused queue has no dates.
function inTraining(queue: SkillQueue[]): SkillTraining | undefined {
  const now = Date.now();
  const e = queue.find((e) => e.startDate && e.finishDate && Date.parse(e.finishDate) > now);
  return e && { skillId: e.skillId, level: e.finishedLevel as SkillTraining["level"] };
}
