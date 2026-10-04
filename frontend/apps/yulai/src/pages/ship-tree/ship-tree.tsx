import { useMemo, useRef, type CSSProperties } from "react";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi } from "@tanstack/react-router";
import { Grid, ShipTree, TreeDisplay, type Data, type FactionIdentifier, type Skills } from "@eve-online-tools/eve-ship-tree";
import "@eve-online-tools/eve-ship-tree/styles.css";
import type { Data as ShipTreeData } from "@bindings/github.com/eve-online-tools/yulai/feature/shiptree";
import type { Skill } from "@bindings/github.com/eve-online-tools/yulai/feature/skills";
import { Alert, EmptyState, PageHead, Select } from "@xaroth.nl/design/react";
import { AppIcon } from "@yulai/ui";
import { skillsFeature, useCharactersWithFeature } from "../../features";
import { shipTreeQuery, skillsQuery } from "../../queries";
import { preloadSprites } from "./preload-sprites";
import { useFrameAnchor } from "./use-frame-anchor";
import styles from "./ship-tree.module.scss";

// In the client's order: empires and ORE, pirates, then the rest.
const factions: { id: FactionIdentifier; name: string }[] = [
  { id: 500003, name: "Amarr Empire" },
  { id: 500001, name: "Caldari State" },
  { id: 500004, name: "Gallente Federation" },
  { id: 500002, name: "Minmatar Republic" },
  { id: 500014, name: "ORE" },
  { id: 500010, name: "Guristas Pirates" },
  { id: 500019, name: "Sansha's Nation" },
  { id: 500012, name: "Blood Raider Covenant" },
  { id: 500011, name: "Angel Cartel" },
  { id: 500020, name: "Serpentis" },
  { id: 500016, name: "Servant Sisters of EVE" },
  { id: 500018, name: "Mordu's Legion Command" },
  { id: 500026, name: "Triglavian Collective" },
  { id: 500027, name: "EDENCOM" },
  { id: 500006, name: "CONCORD Assembly" },
  { id: 500017, name: "The Society of Conscious Thought" },
  { id: 500029, name: "Deathless Circle" },
];

preloadSprites();

// Unstyled marker useFrameAnchor finds the grid's header by.
const gridHeaderClass = "ship-tree-grid-header";

const route = getRouteApi("/layout/ship-tree");

export function ShipTreePage() {
  const characters = useCharactersWithFeature(skillsFeature);
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const character = characters.find((c) => c.id === search.character) ?? characters[0];
  const faction = search.faction ?? factions[0].id;
  const pageRef = useRef<HTMLDivElement>(null);
  const anchor = useFrameAnchor(pageRef, gridHeaderClass);

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
    <div className={styles.page} ref={pageRef}>
      <Tree characterID={character.id} faction={faction} />
      <FactionPicker
        value={faction}
        onChange={(f) => navigate({ search: (s) => ({ ...s, faction: f }) })}
        style={anchor ? { top: anchor.top, left: anchor.left } : undefined}
      />
      <Select
        className={styles.character}
        style={anchor ? { top: anchor.top, right: anchor.right } : undefined}
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

// Sits in the tree's top-left corner, like the client's faction box: a grid of logos, or a dropdown when the window is
// too narrow for it.
function FactionPicker({
  value,
  onChange,
  style,
}: {
  value: FactionIdentifier;
  onChange: (f: FactionIdentifier) => void;
  style?: CSSProperties;
}) {
  return (
    <div className={styles.factions} style={style}>
      <div className={styles.box}>
        <div className={styles.logos} role="radiogroup" aria-label="Faction">
          {factions.map((f) => (
            <button
              key={f.id}
              type="button"
              role="radio"
              aria-checked={f.id === value}
              aria-label={f.name}
              className={styles.logo}
              data-faction={f.id}
              onClick={() => onChange(f.id)}
            />
          ))}
        </div>
        <FactionSummary faction={value} />
      </div>
      <Select
        className={styles.dropdown}
        aria-label="Faction"
        value={value}
        onChange={(e) => onChange(Number(e.target.value) as FactionIdentifier)}
      >
        {factions.map((f) => (
          <option key={f.id} value={f.id}>
            {f.name}
          </option>
        ))}
      </Select>
    </div>
  );
}

// The panel under the logos: what the faction's ships excel at, and how they fight.
function FactionSummary({ faction }: { faction: FactionIdentifier }) {
  const { data } = useQuery(shipTreeQuery);
  const summary = data?.shipTreeFactions[faction];
  const name = factions.find((f) => f.id === faction)?.name;
  return (
    <div className={styles.summary}>
      <span className={styles.summaryLogo} data-faction={faction} aria-hidden="true" />
      <div>
        <div className={styles.summaryName}>{name}</div>
        <div className={styles.elements}>
          {summary?.elements.map(({ _value: id }) => {
            const label = data?.shipTreeElements[id]?.name.en ?? "";
            return (
              <span
                key={id}
                className={styles.element}
                data-element={id}
                data-missing={elementIconMissing(id) || undefined}
                role="img"
                aria-label={label}
              />
            );
          })}
        </div>
      </div>
      {summary?.description.en && <p className={styles.description}>{summary.description.en}</p>}
    </div>
  );
}

// The package has no icon for some elements and sets their variable to url(""); those show a placeholder.
const iconMissing = new Map<number, boolean>();
function elementIconMissing(id: number): boolean {
  let missing = iconMissing.get(id);
  if (missing === undefined) {
    const probe = document.createElement("span");
    probe.dataset.element = String(id);
    document.body.append(probe);
    const icon = getComputedStyle(probe).getPropertyValue("--ship-tree-elements-icons").trim();
    probe.remove();
    missing = icon === "" || /url\(\s*["']?["']?\s*\)/.test(icon);
    iconMissing.set(id, missing);
  }
  return missing;
}

function Tree({ characterID, faction }: { characterID: number; faction: FactionIdentifier }) {
  const tables = useQuery(shipTreeQuery);
  const { data: trained } = useQuery(skillsQuery(characterID));
  const data = useMemo(() => tables.data && toData(tables.data), [tables.data]);
  const skills = useMemo(() => toLevels(trained ?? []), [trained]);
  if (tables.isError) return <Alert tone="danger">{String(tables.error)}</Alert>;
  if (!data) return null;
  return (
    <ShipTree.Root skills={skills} faction={faction} data={data} className={styles.tree}>
      <Grid className={styles.grid} classNames={{ header: gridHeaderClass }} disclaimer={null}>
        <TreeDisplay />
      </Grid>
    </ShipTree.Root>
  );
}

// The backend builds the tables the tree and the faction summary read from our SDE; the others stay empty.
function toData(d: ShipTreeData): Data {
  return {
    ...d,
    factions: {},
    groups: {},
    staticDataFiles: {},
    typeBonus: {},
    typeElements: {},
  } as unknown as Data;
}

function toLevels(skills: Skill[]): Skills {
  return Object.fromEntries(skills.map((s) => [s.skillId, s.activeSkillLevel])) as Skills;
}
