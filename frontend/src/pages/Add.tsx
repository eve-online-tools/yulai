import { useState } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import { Service as Characters } from "../../bindings/github.com/eve-online-tools/yulai/feature/character";
import { featuresQuery } from "../queries";

// Lives in its own small window. After "Log in" the backend sends this window to the SSO.
export function AddPage() {
  const { data: features } = useSuspenseQuery(featuresQuery);
  const { error } = useSearch({ from: "/add" });
  const [chosen, setChosen] = useState<string[]>(() => features.map((f) => f.name));

  const begin = useMutation({ mutationFn: (names: string[]) => Characters.BeginLogin(names) });

  const toggle = (name: string) =>
    setChosen(chosen.includes(name) ? chosen.filter((n) => n !== name) : [...chosen, name]);

  return (
    <div className="add">
      <h1>Add character</h1>
      <p className="muted">Pick what this character should share. You can log in again later to change it.</p>

      {error && <p className="error">{error}</p>}
      {begin.isError && <p className="error">{String(begin.error)}</p>}

      <ul className="feature-list">
        {features.map((f) => (
          <li key={f.name}>
            <label className="check">
              <input type="checkbox" checked={chosen.includes(f.name)} onChange={() => toggle(f.name)} />
              <span>
                <span className="name">{f.name}</span>
                <span className="muted small scopes">{f.scopes.join(", ")}</span>
              </span>
            </label>
          </li>
        ))}
      </ul>

      <footer className="page-footer">
        <button onClick={() => begin.mutate(chosen)} disabled={begin.isPending}>
          {begin.isPending ? "Opening EVE login..." : "Log in with EVE Online"}
        </button>
      </footer>
    </div>
  );
}
