# Yulai application framework plan

Yulai follows the architecture proven in asset-manager's `poc` branch. This document is the
plan for the framework; the code in this repo is a compiling skeleton of it. Every stub returns
`todo.ErrNotImplemented` (grep `core/todo` to find them) and the UI shell renders.

Real features are out of scope for now. This plan describes where they plug in.

## Principles

- **Feature = consent.** A feature names the SSO scopes it needs. A character has a feature
  when its token's `scp` claim covers those scopes. Consent is never stored separately.
- **Backend owns state, frontend renders it.** The Go side persists everything in sqlite and emits
  a Wails event when something changes. The frontend reads through bound services with TanStack
  Query and invalidates on events. It never polls.
- **Packages own their tables.** The schema is global (one goose migration set), but each table
  has one owning package with its own `queries.sql` and sqlc output. Other packages go through
  the owner's Go API.
- **Dependencies point inward.** `app` → `feature/*` → `identity/*` → `core/*`. Features depend
  on interfaces (`Enroller`, `LoginUI`, `Tokens`), not on each other, and `app` wires them.

## Layers

| Package            | Responsibility                                                     | Skeleton state |
|--------------------|--------------------------------------------------------------------|----------------|
| `main.go`          | Load config, build `app.App`, create the Wails app and main window | done           |
| `app`              | Config, wiring, event registration, Wails windows (`loginWindow`)  | wiring done, db commented out |
| `core/db`          | sqlite (glebarez, pure Go), WAL, single writer, goose migrations   | stub           |
| `core/esi`         | lib-esi-go transport, rate limit + disk cache middleware, `Check`/`Fetch`/`ExpiresAt` | returns a bare `http.Client` |
| `core/keyring`     | `Store` interface, OS impl via zalando/go-keyring                  | stub           |
| `core/crypt`       | AES-GCM `Sealer`, master key in keyring                            | stub           |
| `identity/sso`     | Discovery, PKCE, exchange, refresh (`ErrInvalidGrant`), JWT verify | types only     |
| `identity/login`   | Callback listener, `Pending` attempt, `Wait`/`Cancel`              | stub           |
| `identity/token`   | `tokens` table, sealed refresh tokens, `For()` → `RefreshableToken`| stub           |
| `feature`          | `Feature`, `Job`, `Run`, `Outcome` contracts, `Enabled()`          | done (tested)  |
| `feature/character`| `characters` table, add-character flow, list, remove, needs-login  | `Features()` real, rest stub |
| `feature/sync`     | Per-character job scheduler, `sync_jobs` table                     | job registry real, loop stub |
| `frontend`         | Router, query layer, event listener, Root/Characters/Accounts/Add pages | done, renders stub data |

The asset-manager `poc` implementations are the reference for every stub. Port each one and
replace the asset-manager module path and `github.com/xaroth/lib-esi-go` with the paths below.

## Boot sequence

1. `app.LoadConfig()` resolves the data dir (`xdg.DataFile("yulai")`) and reads
   `sso.dev.json` or `<config>/yulai/sso.json`.
2. `app.New()`:
   1. `db.Open(DBPath)` runs migrations. Commented out until `core/db` is real.
   2. `crypt.Open(keyring.OS{Service: "eve-online-tools/yulai", User: "master-key"})`
   3. SSO client → verifier → login flow → token store → ESI client
   4. Build the `[]feature.Feature` list. Its order is the UI order.
   5. Scheduler (with `onAuth` → `Characters.MarkNeedsLogin`), then the character service.
3. Wails app with `a.Services()` and the main window.
4. `a.Start(ctx)` enrolls every character and starts the scheduler goroutine.
5. `wails.Run()`.

## Data model (first migration, `core/db/migrations/00001_init.sql`)

Ported from asset-manager, minus `presence`:

- `characters`: id, name, owner_hash, corporation_id, alliance_id, account_label,
  status (`ok` | `needs_login`), status_error, added_at, updated_at
- `tokens`: character_id (FK, cascade), access_token, refresh_token_enc, expires_at,
  scopes (space separated `scp`), issued_at (`iat`)
- `sync_jobs`: (character_id, job) PK, next_run (NULL = paused), last_run, last_error, state (JSON)

Each feature then adds its own tables in a new migration. Once a table exists, the hand-written
`ListRow` / `SyncJob` structs are replaced by sqlc-generated ones (`sqlc_*.go`, `*_sqlc.go`). Keep the
field names and JSON tags so the bindings don't change. `sqlc.yaml` is added together with the first
`queries.sql` (copy asset-manager's anchor-based layout).

## Adding a feature (the extension point)

1. `feature/<name>/`: a type implementing `feature.Feature` (`Name`, `Scopes`, `Jobs`).
2. Jobs return `feature.Outcome{Next, State, Wake}`. Use `esi.ExpiresAt` to pick `Next` from cache headers.
3. Tables go in a new migration. Queries go in `feature/<name>/queries.sql` with a matching `sqlc.yaml` entry.
4. Optional Wails `Service` for the UI, with `ServiceName()`. Emit `<name>:changed` after writes and register
   the event in `app/app.go` `init()`.
5. Register the feature in `app.New`'s `features` slice and its service in `Services()`.
6. Frontend: add a key, a `queryOptions` and an `Events.On` to `src/queries.ts`. Add a route in
   `src/router.tsx` and/or a panel in the character card.

## Frontend conventions

- Hash history, so secondary windows open at `/#/<route>` (e.g. `/#/add` for the picker window).
- Routes under `layout` get the top bar. Routes under root (`/add`) are chromeless windows.
- Loaders call `queryClient.ensureQueryData` and components use `useSuspenseQuery`.
- Bindings are generated as classes (`-ts`, no `-i`) into `frontend/bindings` and committed.

## lib-esi-go move

`Xaroth/lib-esi-go` is moving to `eve-online-tools/lib-esi-go`. Yulai already imports only the new path.

- Today `go.mod` pins `github.com/eve-online-tools/lib-esi-go v1.0.1-0.20261001221054-bcddf34454dc`.
  That is the `chore/rename-module-path` branch, which is pushed but not merged.
- `v1.0.0` exists under the new path but still declares the old module name, so do not use it.
- Once the rename is merged and tagged (`v1.0.1` or `v1.1.0`), run
  `go get github.com/eve-online-tools/lib-esi-go@<tag>`. If the branch is squash-merged, do this promptly,
  because the pinned commit only lives on in the module proxy.
- For local co-development, use a temporary `replace github.com/eve-online-tools/lib-esi-go => ../lib-esi-go`.
  Never commit it.
- Imports in use: `middleware/authentication` (contracts). When `core/esi` is ported it will also use
  the root package (`CompatibilityDate`), `transport`, `request`, `middleware/cache` and
  `middleware/ratelimiting{,/memory}`.

## Milestones

1. **Infrastructure:** `core/db` + first migration, `core/keyring`, `core/crypt`, `core/esi` (port and test).
   Uncomment `db.Open` in `app.New`.
2. **Identity:** `identity/sso`, `identity/login`, `identity/token`, sqlc for `tokens`.
3. **Characters:** `character` queries, `BeginLogin`/`store`/`Remove`/`MarkNeedsLogin`, `EventChanged`.
   `LoadConfig` makes a missing SSO file fatal again.
4. **Sync:** scheduler loop, `Enroll`, `RunNow`, `sync_jobs` queries, port asset-manager's scheduler tests.
5. **First real feature** using the recipe above.
6. Delete `core/todo`.

## Open questions

- **What Yulai's features are.** That decides scopes, tables and pages beyond the shell.
- **Shared module.** `core/*`, `identity/*`, `feature` and `feature/sync` are app-agnostic and would be
  duplicated with asset-manager. They could be extracted into a shared `eve-online-tools/*` module once both
  apps have stabilised them. Until then, port by copy.
- **Callback port.** Yulai uses `45538` (asset-manager uses `45537`) so both apps can log in side by side.
  Each needs its own SSO app registration.
- **Mobile.** The build targets exist from the template, but `keyring.OS` needs a mobile `Store`.
