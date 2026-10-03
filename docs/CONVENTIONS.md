# Code conventions

Rules for Go code in this repo. Review against them; change this file when a rule changes.

## Errors

One call per statement. Check its error on the next line and return. Never pass a call's results straight
into another call that checks them.

```go
// Wrong: the request and its checks are one nested expression.
data, err := esi.Check(getcharacterscharacteridship.Request(ctx, f.esi,
	&getcharacterscharacteridship.Input{Character: esicharacter.Identifier(id)}))

// Right
input := &getcharacterscharacteridship.Input{Character: esicharacter.Identifier(id)}
resp, err := getcharacterscharacteridship.Request(ctx, f.esi, input, f.auth(id))
if err != nil {
	return PresenceShip{}, err
}
if err := esi.ResponseError(resp); err != nil {
	return PresenceShip{}, err
}
data := resp.Data
```

The same goes for helpers that wrap a `(value, error)` pair. Call, check, then use the value.

## ESI requests

1. Build the input in its own variable.
2. Call the generated `Request`. A non-nil `err` is a transport, auth or decode failure.
3. Call `esi.ResponseError(resp)`. It returns an `*esi.Error` for a non-2xx status, or for a 2xx without the
   body the endpoint promises. lib-esi-go does neither itself.
4. Read `resp.Data`.

Authenticated requests pass `authentication.WithToken(tokens.For(characterID))` as the last argument.

## Declarations

- One `var` per task, not a `var (...)` block, so each reads at top level.
- Options on their own lines when there are more than two, one per line with a trailing comma.
- Struct literals with more than three fields go one field per line.

## Tasks

- A task returns what it fetched and stored, normally the row from an `Upsert ... RETURNING *` query. A run
  triggered on demand (`Run`) then gets the data, not `struct{}`.
- `task.Pausable` is for user-facing units only. Do not make the sub-tasks of a feature pausable.
- One file per ESI endpoint in the feature package, holding its input type, task var and run method.
  Shared state, seeds and wiring go in `<feature>.go`.
- Fan-out is `Task.Queue` from inside a run. Dedupe in the receiver when many inputs share one target.

## Tables

- One table per ESI endpoint, columns typed and nullable like the response, so sqlc params take ESI output
  fields as they are. No `ptr(int64(...))` conversions at call sites.
- ESI ids use the column types mapped in `sqlc.yaml` (`TYPE_ID`, `ITEM_ID`, `SOLAR_SYSTEM_ID`, ...), which
  sqlc turns into lib-esi-go `Identifier` types, pointers when nullable. A new id kind is one pair of
  overrides there.
- Flags are `BOOLEAN`, times are `TIMESTAMP` (stored as RFC 3339 text, write UTC).
- Name tables after their feature (`presence_ships`, `character_sheets`), or after the entity for universe
  data shared across features (`corporations`, `alliances`).
- `character_id` stays `INTEGER` and references `characters(id)` with `ON DELETE CASCADE`.

## Events

Emit `<feature>:changed` only when stored values change, not on every fetch, so the frontend does not
refetch every tick.
