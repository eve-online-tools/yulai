import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { FeatureInfo, ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Alert, Button, EmptyState, Icon, PageHead, Table, Tag } from "@xaroth.nl/design/react";
import { Portrait, enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery } from "../../queries";

const columns = [
  { key: "character", label: "Character" },
  { key: "token", label: "Token" },
  { key: "refreshed", label: "Last refreshed" },
  { key: "features", label: "Features" },
  { key: "remove", label: "", align: "end" as const },
];

export function AccountsPage() {
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: features } = useSuspenseQuery(featuresQuery);
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });

  return (
    <>
      <PageHead
        title="Accounts"
        lead="Login continues in your browser."
        actions={
          <Button onClick={() => add.mutate()} start={<Icon name="plus" />}>
            Add character
          </Button>
        }
      />
      {add.isError && <Alert tone="danger">{String(add.error)}</Alert>}

      {characters.length === 0 ? (
        <EmptyState title="No characters yet" />
      ) : (
        <Table label="Characters" hover columns={columns} rows={characters.map((c) => accountRow(c, features))} />
      )}
    </>
  );
}

function accountRow(c: Character, features: FeatureInfo[]) {
  const enabled = enabledFeatures(features, c.scopes);
  const nowSec = Date.now() / 1000;
  const active = c.status === "ok" && c.tokenExpiresAt != null && c.tokenExpiresAt > nowSec;
  const tokenLabel = c.status !== "ok" ? "needs login" : active ? "active" : "expired, refreshes on use";

  return {
    character: (
      <div className="row">
        <Portrait id={c.id} size={24} />
        <span>{c.name}</span>
      </div>
    ),
    token: (
      <span className={c.status !== "ok" ? "warn" : active ? "ok" : "muted"} title={c.statusError ?? ""}>
        {tokenLabel}
      </span>
    ),
    refreshed: <span className="muted">{c.tokenIssuedAt ? new Date(c.tokenIssuedAt * 1000).toLocaleString() : "?"}</span>,
    features: (
      <div className="row">
        {enabled.length === 0 && <span className="muted">none</span>}
        {enabled.map((f) => (
          <Tag key={f.name} active>
            {f.name}
          </Tag>
        ))}
      </div>
    ),
    remove: <RemoveButton id={c.id} />,
  };
}

function RemoveButton({ id }: { id: number }) {
  const remove = useMutation({ mutationFn: () => Characters.Remove(id) });
  return (
    <Button variant="tertiary" tone="danger" size="sm" aria-label="Remove" onClick={() => remove.mutate()}>
      <Icon name="close" />
    </Button>
  );
}
