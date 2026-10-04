import { useMemo } from "react";
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
import styles from "./ship-tree.module.scss";

// Only the empire factions have layouts in the package.
const factions: { id: FactionIdentifier; name: string }[] = [
  { id: 500003, name: "Amarr Empire" },
  { id: 500001, name: "Caldari State" },
  { id: 500004, name: "Gallente Federation" },
  { id: 500002, name: "Minmatar Republic" },
];

const route = getRouteApi("/layout/ship-tree");

export function ShipTreePage() {
  const characters = useCharactersWithFeature(skillsFeature);
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const character = characters.find((c) => c.id === search.character) ?? characters[0];
  const faction = search.faction ?? factions[0].id;

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

  return (
    <>
      <PageHead
        title="Ship Tree"
        actions={
          <div className={styles.pickers}>
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
            <Select
              aria-label="Faction"
              value={faction}
              onChange={(e) => navigate({ search: (s) => ({ ...s, faction: Number(e.target.value) as FactionIdentifier }) })}
            >
              {factions.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.name}
                </option>
              ))}
            </Select>
          </div>
        }
      />
      <Tree characterID={character.id} faction={faction} />
    </>
  );
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
      <Grid className={styles.grid} disclaimer={null}>
        <TreeDisplay />
      </Grid>
    </ShipTree.Root>
  );
}

// The backend builds only the tables the tree reads from our SDE; the others stay empty.
function toData(d: ShipTreeData): Data {
  return {
    ...d,
    factions: {},
    groups: {},
    shipTreeElements: {},
    shipTreeFactions: {},
    staticDataFiles: {},
    typeBonus: {},
    typeElements: {},
  } as unknown as Data;
}

function toLevels(skills: Skill[]): Skills {
  return Object.fromEntries(skills.map((s) => [s.skillId, s.activeSkillLevel])) as Skills;
}
