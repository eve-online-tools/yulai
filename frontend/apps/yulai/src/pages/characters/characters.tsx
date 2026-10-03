import { useEffect, useState } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { FeatureInfo, ListRow as Character } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import { Alert, Button, ConfirmDialog, EmptyState, Icon, PageHead, Table, Tag, Tooltip } from "@xaroth.nl/design/react";
import { Portrait, enabledFeatures } from "@yulai/ui";
import { charactersQuery, featuresQuery } from "../../queries";

const columns = [
  { key: "character", label: "Character" },
  { key: "token", label: "Token" },
  { key: "features", label: "Features" },
  { key: "remove", label: "", align: "end" as const },
];

export function CharactersPage() {
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const { data: features } = useSuspenseQuery(featuresQuery);
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });
  const now = useNow();

  return (
    <>
      <PageHead
        title="Characters"
        actions={
          <Tooltip id="add-character" text="Login continues in your browser." placement="bottom">
            <Button onClick={() => add.mutate()} start={<Icon name="plus" />}>
              Add character
            </Button>
          </Tooltip>
        }
      />
      {add.isError && <Alert tone="danger">{String(add.error)}</Alert>}

      {characters.length === 0 ? (
        <EmptyState title="No characters yet" />
      ) : (
        <Table label="Characters" hover columns={columns} rows={characters.map((c) => characterRow(c, features, now))} />
      )}
    </>
  );
}

function characterRow(c: Character, features: FeatureInfo[], nowSec: number) {
  const enabled = enabledFeatures(features, c.scopes);
  const active = c.status === "ok" && c.tokenExpiresAt != null && c.tokenExpiresAt > nowSec;
  const tokenLabel = c.status !== "ok" ? "needs login" : active ? "active" : "expired, refreshes on use";
  const refreshed = `Last refreshed ${c.tokenIssuedAt ? timeAgo(nowSec - c.tokenIssuedAt) : "never"}`;

  return {
    character: (
      <div className="row">
        <Portrait id={c.id} size={24} />
        <span>{c.name}</span>
      </div>
    ),
    token: (
      <Tooltip id={`token-${c.id}`} text={c.statusError ? `${c.statusError}. ${refreshed}` : refreshed}>
        <span className={c.status !== "ok" ? "warn" : active ? "ok" : "muted"} tabIndex={0}>
          {tokenLabel}
        </span>
      </Tooltip>
    ),
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
    remove: <RemoveButton id={c.id} name={c.name} />,
  };
}

function RemoveButton({ id, name }: { id: number; name: string }) {
  const [confirming, setConfirming] = useState(false);
  const remove = useMutation({
    mutationFn: () => Characters.Remove(id),
    onSuccess: () => setConfirming(false),
  });

  return (
    <>
      <Tooltip id={`remove-${id}`} text={`Remove ${name}`}>
        <Button variant="tertiary" tone="danger" size="sm" aria-label={`Remove ${name}`} onClick={() => setConfirming(true)}>
          <Icon name="close" />
        </Button>
      </Tooltip>
      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => remove.mutate()}
        title="Remove character"
        tone="danger"
        confirmLabel="Remove"
        pending={remove.isPending}
      >
        <p>Remove {name} and its stored token? Log in again to add it back.</p>
        {remove.isError && <Alert tone="danger">{String(remove.error)}</Alert>}
      </ConfirmDialog>
    </>
  );
}

// Seconds since epoch, ticking every minute so relative times stay current.
function useNow() {
  const [now, setNow] = useState(() => Date.now() / 1000);
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now() / 1000), 60_000);
    return () => clearInterval(t);
  }, []);
  return now;
}

function timeAgo(sec: number) {
  const plural = (n: number, unit: string) => `${n} ${unit}${n === 1 ? "" : "s"} ago`;
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)} min ago`;
  if (sec < 86400) return plural(Math.floor(sec / 3600), "hour");
  return plural(Math.floor(sec / 86400), "day");
}
