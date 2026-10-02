import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { charactersQuery, setupQuery } from "../queries";

// UI-only state, so it lives in the webview rather than the database.
const storageKey = "yulai.gettingStarted";

type Stored = { dismissed: boolean; skipped: string[] };

function load(): Stored {
  try {
    return { dismissed: false, skipped: [], ...JSON.parse(localStorage.getItem(storageKey) ?? "{}") };
  } catch {
    return { dismissed: false, skipped: [] };
  }
}

type Item = { id: string; title: string; done: boolean; body: ReactNode; skippable?: boolean };

// Completion is derived from backend state. Only skips and dismissal are stored.
export function GettingStarted() {
  const [stored, setStored] = useState(load);
  const { data: characters = [] } = useQuery(charactersQuery);
  const { data: setup } = useQuery(setupQuery);
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });

  const save = (next: Stored) => {
    localStorage.setItem(storageKey, JSON.stringify(next));
    setStored(next);
  };

  const ssoHost = setup?.issuer ? new URL(setup.issuer).host : "the EVE SSO";

  const items: Item[] = [
    {
      id: "character",
      title: "Add a character",
      done: characters.length > 0,
      body: <button onClick={() => add.mutate()}>Add character</button>,
    },
    {
      id: "accounts",
      title: "Add more characters",
      done: characters.length > 1,
      skippable: true,
      body: (
        <>
          <p className="muted">EVE SSO remembers your account. For a character on another account, log out at {ssoHost} first.</p>
          <button onClick={() => add.mutate()}>Add character</button>
        </>
      ),
    },
  ].map((i) => ({ ...i, done: i.done || stored.skipped.includes(i.id) }));

  const doneCount = items.filter((i) => i.done).length;
  if (stored.dismissed || doneCount === items.length) return null;

  return (
    <details className="getting-started">
      <summary className="nav-link">
        Getting started {doneCount}/{items.length}
      </summary>
      <div className="popover">
        <ul className="checklist">
          {items.map((i) => (
            <li key={i.id} className={i.done ? "done" : ""}>
              <div className="row">
                <span className="check">{i.done ? "✓" : ""}</span>
                <span>{i.title}</span>
                {!i.done && i.skippable && (
                  <button className="ghost push-right" onClick={() => save({ ...stored, skipped: [...stored.skipped, i.id] })}>
                    skip
                  </button>
                )}
              </div>
              {!i.done && <div className="checklist-body small">{i.body}</div>}
            </li>
          ))}
        </ul>
        {add.isError && <p className="error small">{String(add.error)}</p>}
        <button className="ghost" onClick={() => save({ ...stored, dismissed: true })}>
          Hide checklist
        </button>
      </div>
    </details>
  );
}
