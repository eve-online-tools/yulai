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
  const relogin = useMutation({ mutationFn: (id: number) => Characters.Relogin(id) });
  const now = useNow();
  const [removing, setRemoving] = useState<Character | null>(null);

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
      {relogin.isError && <Alert tone="danger">{String(relogin.error)}</Alert>}

      {characters.length === 0 ? (
        <EmptyState title="No characters yet" />
      ) : (
        <Table label="Characters" hover columns={columns} rows={characters.map((c) => characterRow(c, features, now, relogin.mutate, setRemoving))} />
      )}
      <RemoveDialog character={removing} onClose={() => setRemoving(null)} />
    </>
  );
}

function characterRow(
  c: Character,
  features: FeatureInfo[],
  nowSec: number,
  onRelogin: (id: number) => void,
  onRemove: (c: Character) => void,
) {
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
      <div className="row">
        <Tooltip id={`token-${c.id}`} text={c.statusError ? `${c.statusError}. ${refreshed}` : refreshed}>
          <span className={c.status !== "ok" ? "warn" : active ? "ok" : "muted"} tabIndex={0}>
            {tokenLabel}
          </span>
        </Tooltip>
        {c.status !== "ok" && (
          <Tooltip id={`relogin-${c.id}`} text="Login continues in your browser.">
            <Button variant="secondary" size="sm" onClick={() => onRelogin(c.id)}>
              Log in again
            </Button>
          </Tooltip>
        )}
      </div>
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
    remove: (
      <Tooltip id={`remove-${c.id}`} text={`Remove ${c.name}`}>
        <Button variant="tertiary" tone="danger" size="sm" aria-label={`Remove ${c.name}`} onClick={() => onRemove(c)}>
          <Icon name="close" />
        </Button>
      </Tooltip>
    ),
  };
}

// One dialog for the page, outside the table: a dialog inherits text alignment from its DOM parent, and the
// remove column is end-aligned.
function RemoveDialog({ character, onClose }: { character: Character | null; onClose: () => void }) {
  // Keeps the name on screen while the dialog animates closed.
  const [shown, setShown] = useState(character);
  if (character && character !== shown) setShown(character);
  const remove = useMutation({ mutationFn: (id: number) => Characters.Remove(id), onSuccess: onClose });
  const close = () => {
    remove.reset();
    onClose();
  };

  return (
    <ConfirmDialog
      open={character != null}
      onClose={close}
      onConfirm={() => shown && remove.mutate(shown.id)}
      title="Remove character"
      tone="danger"
      confirmLabel="Remove"
      pending={remove.isPending}
    >
      <p>Remove {shown?.name} and its stored token? Log in again to add it back.</p>
      {remove.isError && <Alert tone="danger">{String(remove.error)}</Alert>}
    </ConfirmDialog>
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
