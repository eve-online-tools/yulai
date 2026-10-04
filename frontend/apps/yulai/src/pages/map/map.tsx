import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { MapData, Marker } from "@eve-online-tools/eve-map";
import { EveMap, type EveMapApi } from "@eve-online-tools/eve-map/react";
import type { ListPresenceRow } from "@bindings/github.com/eve-online-tools/yulai/feature/presence";
import type { ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Alert, EmptyState, PageHead } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";
import { mapQuery, presenceQuery } from "../../queries";
import { presenceFeature, useCharactersWithFeature } from "../../features";
import styles from "./map.module.scss";

const onlineColor = "#5ce1e6";
const offlineColor = "#6b7785";

type Pilot = { character: Character; online: boolean };

export function MapPage() {
  const map = useQuery(mapQuery);
  const { data: presence = [] } = useQuery(presenceQuery);
  const tracked = useCharactersWithFeature(presenceFeature);

  const bySystem = useMemo(() => groupBySystem(presence, tracked), [presence, tracked]);
  const markers = useMemo<Marker[]>(
    () =>
      [...bySystem].map(([systemId, pilots]) => ({
        systemId,
        color: pilots.some((p) => p.online) ? onlineColor : offlineColor,
        shape: "diamond",
        size: 12,
      })),
    [bySystem],
  );
  // Follow online characters; with none online, all of them.
  const focus = useMemo(() => {
    const online = [...bySystem].filter(([, pilots]) => pilots.some((p) => p.online)).map(([id]) => id);
    return online.length > 0 ? online : [...bySystem.keys()];
  }, [bySystem]);

  return (
    <>
      <PageHead title="New Eden" />
      {tracked.length === 0 ? (
        <EmptyState title="No characters with Presence">Log in again with Presence checked to see characters on the map.</EmptyState>
      ) : map.isError ? (
        <Alert tone="danger">{String(map.error)}</Alert>
      ) : !map.data ? (
        <p className="muted">Loading map</p>
      ) : (
        <EveMap
          className={styles.map}
          // Bindings type position2d as nullable, but Go omits it instead of sending null.
          data={map.data as MapData}
          markers={markers}
          focus={focus}
          autoFocus={10_000}
          showRegionLabels={2.5}
          showConstellationLabels={[2, 8]}
          showSystemLabels={6}
          fallback={<Alert tone="danger">The map needs WebGL 2, which is not available.</Alert>}
        >
          {(api) => <Pilots api={api} bySystem={bySystem} />}
        </EveMap>
      )}
    </>
  );
}

function groupBySystem(presence: ListPresenceRow[], tracked: Character[]) {
  const characters = new Map(tracked.map((c) => [c.id, c]));
  const out = new Map<number, Pilot[]>();
  for (const p of presence) {
    const character = characters.get(p.characterId);
    if (!character) continue;
    const pilots = out.get(p.solarSystemId) ?? [];
    pilots.push({ character, online: p.online === true });
    out.set(p.solarSystemId, pilots);
  }
  return out;
}

// Portraits next to each occupied system. Re-rendered on every camera change.
function Pilots({ api, bySystem }: { api: EveMapApi; bySystem: Map<number, Pilot[]> }) {
  return (
    <>
      {[...bySystem].map(([systemId, pilots]) => {
        const pos = api.project(systemId);
        if (!pos?.visible) return null;
        return (
          <ul key={systemId} className={styles.pilots} style={{ left: pos.x, top: pos.y }}>
            {pilots.map(({ character, online }) => (
              <li key={character.id} className={online ? undefined : styles.offline}>
                <Portrait id={character.id} size={20} />
                <span>{character.name}</span>
              </li>
            ))}
          </ul>
        );
      })}
    </>
  );
}
