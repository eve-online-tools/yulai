import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { MapData, Marker } from "@eve-online-tools/eve-map";
import { EveMap, type EveMapApi } from "@eve-online-tools/eve-map/react";
import type { ListPresenceRow } from "@bindings/github.com/eve-online-tools/yulai/feature/presence";
import type { ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Alert, EmptyState, PageHead, Select } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";
import { mapQuery, presenceQuery } from "../../queries";
import { presenceFeature, useCharactersWithFeature } from "../../features";
import styles from "./map.module.scss";

const onlineColor = "#5ce1e6";
const offlineColor = "#8a96a3";
const all = "all";

type Pilot = { character: Character; online: boolean };

export function MapPage() {
  const map = useQuery(mapQuery);
  const { data: presence = [] } = useQuery(presenceQuery);
  const tracked = useCharactersWithFeature(presenceFeature);
  const [selected, setSelected] = useState(all);
  const [frame, setFrame] = useState<HTMLDivElement | null>(null);
  useRenderOnResize(frame);

  const bySystem = useMemo(() => groupBySystem(presence, tracked), [presence, tracked]);
  const onlineSystems = useMemo(
    () => [...bySystem].filter(([, pilots]) => pilots.some((p) => p.online)).map(([id]) => id),
    [bySystem],
  );
  const markers = useMemo<Marker[]>(
    () =>
      [...bySystem].map(([systemId, pilots]) => ({
        systemId,
        color: pilots.some((p) => p.online) ? onlineColor : offlineColor,
        shape: "diamond",
        size: 18,
      })),
    [bySystem],
  );
  // One character's system, or all online characters; with none online, all of them.
  const focus = useMemo(() => {
    if (selected !== all) {
      const row = presence.find((p) => String(p.characterId) === selected);
      if (row) return [row.solarSystemId];
    }
    return onlineSystems.length > 0 ? onlineSystems : [...bySystem.keys()];
  }, [selected, presence, onlineSystems, bySystem]);

  return (
    <>
      <PageHead
        title="New Eden"
        actions={
          tracked.length > 0 && (
            <Select aria-label="Focus on" value={selected} onChange={(e) => setSelected(e.target.value)}>
              <option value={all}>All characters</option>
              {tracked.map((c) => (
                <option key={c.id} value={String(c.id)} disabled={!presence.some((p) => p.characterId === c.id)}>
                  {c.name}
                </option>
              ))}
            </Select>
          )
        }
      />
      {tracked.length === 0 ? (
        <EmptyState title="No characters with Presence">Log in again with Presence checked to see characters on the map.</EmptyState>
      ) : map.isError ? (
        <Alert tone="danger">{String(map.error)}</Alert>
      ) : !map.data ? (
        <p className="muted">Loading map</p>
      ) : (
        <div ref={setFrame} className={styles.frame}>
          <EveMap
            className={styles.map}
            // Bindings type position2d as nullable, but Go omits it instead of sending null.
            data={map.data as MapData}
            markers={markers}
            highlight={onlineSystems}
            focus={focus}
            autoFocus={10_000}
            showRegionLabels={2.5}
            showConstellationLabels={[2, 8]}
            showSystemLabels={6}
            fallback={<Alert tone="danger">The map needs WebGL 2, which is not available.</Alert>}
          >
            {(api) => <Pilots api={api} bySystem={bySystem} />}
          </EveMap>
        </div>
      )}
    </>
  );
}

// eve-map re-projects on resize without a camera event, so the overlay would
// keep stale positions. Re-render once per frame while el changes size.
// Drop once eve-online-tools/node-packages#59 is fixed.
function useRenderOnResize(el: HTMLElement | null) {
  const [, setCount] = useState(0);
  useEffect(() => {
    if (!el) return;
    let raf = 0;
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => setCount((n) => n + 1));
    });
    observer.observe(el);
    return () => {
      cancelAnimationFrame(raf);
      observer.disconnect();
    };
  }, [el]);
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

// Tags above each occupied system. Re-rendered on every camera change and resize.
function Pilots({ api, bySystem }: { api: EveMapApi; bySystem: Map<number, Pilot[]> }) {
  return (
    <>
      {[...bySystem].map(([systemId, pilots]) => {
        const pos = api.project(systemId);
        if (!pos?.visible) return null;
        const online = pilots.some((p) => p.online);
        return (
          <ul
            key={systemId}
            className={online ? styles.pilots : `${styles.pilots} ${styles.offline}`}
            style={{ left: pos.x, top: pos.y }}
          >
            {pilots.map(({ character, online }) => (
              <li key={character.id} className={online ? undefined : styles.away}>
                <Portrait id={character.id} size={28} />
                <span>{character.name}</span>
              </li>
            ))}
          </ul>
        );
      })}
    </>
  );
}
