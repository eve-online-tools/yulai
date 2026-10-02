import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { FeatureInfo, ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Portrait, Tag, enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery } from "../queries";

export function AccountsPage() {
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: features } = useSuspenseQuery(featuresQuery);
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });

  return (
    <>
      <header className="page-header">
        <h1>Accounts</h1>
      </header>

      {characters.length === 0 ? (
        <p className="muted">No characters yet.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Character</th>
                <th>Token</th>
                <th>Last refreshed</th>
                <th>Features</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {characters.map((c) => (
                <AccountRow key={c.id} c={c} features={features} />
              ))}
            </tbody>
          </table>
        </div>
      )}

      <footer className="page-footer">
        {add.isError && <p className="error">{String(add.error)}</p>}
        <button onClick={() => add.mutate()}>Add character</button>
        <p className="muted small">Login continues in your browser.</p>
      </footer>
    </>
  );
}

function AccountRow({ c, features }: { c: Character; features: FeatureInfo[] }) {
  const remove = useMutation({ mutationFn: () => Characters.Remove(c.id) });
  const enabled = enabledFeatures(features, c.scopes);

  const nowSec = Date.now() / 1000;
  const active = c.status === "ok" && c.tokenExpiresAt != null && c.tokenExpiresAt > nowSec;
  const tokenLabel = c.status !== "ok" ? "needs login" : active ? "active" : "expired, refreshes on use";

  return (
    <tr>
      <td>
        <div className="row">
          <Portrait id={c.id} size={24} className="avatar" />
          <span>{c.name}</span>
        </div>
      </td>
      <td>
        <span className={c.status !== "ok" ? "warn" : active ? "ok" : "muted"} title={c.statusError ?? ""}>
          {tokenLabel}
        </span>
      </td>
      <td className="muted">{c.tokenIssuedAt ? new Date(c.tokenIssuedAt * 1000).toLocaleString() : "?"}</td>
      <td>
        <div className="row small">
          {enabled.length === 0 && <span className="muted">none</span>}
          {enabled.map((f) => (
            <Tag key={f.name} on>{f.name}</Tag>
          ))}
        </div>
      </td>
      <td>
        <button className="ghost" onClick={() => remove.mutate()} title="Remove">
          x
        </button>
      </td>
    </tr>
  );
}
