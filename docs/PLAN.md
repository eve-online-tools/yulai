# Yulai application framework plan

Yulai follows the architecture proven in asset-manager's `poc` branch. This document is the
plan for the framework and where features plug in.

## Principles

- **Feature = consent.** A feature names the SSO scopes it needs. A character has a feature
  when its token's `scp` claim covers those scopes. Consent is never stored separately.
- **Backend owns state, frontend renders it.** The Go side persists everything in sqlite and emits
  `data:changed` with a topic when something changes. The frontend reads through bound services with
  TanStack Query and invalidates the queries under that topic. It never polls.
- **Packages own their tables.** The schema is global (one goose migration set), but each table
  has one owning package with its own `queries.sql` and sqlc output. Other packages go through
  the owner's Go API.
- **Dependencies point inward.** `app` → `feature/*` → `identity/*` → `core/*`. Features build from
  `feature.Deps`, not from each other, and `app` lists them.

## Layers

| Package            | Responsibility                                                     | Skeleton state |
|--------------------|--------------------------------------------------------------------|----------------|
| `main.go`          | Load config, build `app.App`, create the Wails app and main window | done           |
| `app`              | Config, wiring, event registration                                 | done           |
| `core/db`          | sqlite (modernc, pure Go), WAL, single writer, goose migrations   | done           |
| `core/esi`         | lib-esi-go transport, rate limit + disk cache middleware, `Check`/`Fetch`/`ExpiresAt` | ported (tested) |
| `core/keyring`     | `Store` interface, OS impl via zalando/go-keyring                  | done           |
| `core/crypt`       | AES-GCM `Sealer`, master key in keyring                            | done (tested)  |
| `identity/sso`     | Discovery, PKCE, exchange, refresh (`ErrInvalidGrant`), JWT verify | ported         |
| `identity/login`   | Permanent loopback server: feature picker, SSO redirect, callback  | done (tested)   |
| `identity/token`   | `tokens` table, sealed refresh tokens, `For()` → `RefreshableToken`| ported         |
| `feature`          | `Feature` (ID, name, scopes, tasks, services), `Deps`, `Covers()`  | done (tested) |
| `feature/character`| `characters` table, list, remove, needs-login                      | done (tested) |
| `feature/presence` | `presence_online`, `presence_locations`, `presence_ships`. Paced by online state | sync done (tested), no UI yet |
| `feature/charactersheet`| Always on, no scopes. `character_sheets`, `corporations`, `alliances` | sync done (tested), no UI yet |
| `feature/skills`   | `character_attributes`, `skill_totals`, `skills`, `skill_queue`. Every 5 minutes | sync done (tested), no UI yet |
| `feature/shiptree` | Ship tree tables derived from the SDE                              | done (tested)  |
| `feature/starmap`  | Known space derived from the SDE, for the map                      | done (tested)  |
| `core/task`        | Generic in-memory scheduler, `task_pauses` table (see SCHEDULER.md) | engine done (tested), not wired |
| `core/sde`         | Static Data Export in its own `sde.sqlite`, checked hourly (see SDE.md) | planned |
| `frontend`         | pnpm workspaces: `apps/yulai` (router, query layer, events, pages), `packages/ui` | done, renders stub data |

The asset-manager `poc` implementations are the reference for every stub. Port each one and
replace the asset-manager module path and `github.com/xaroth/lib-esi-go` with the paths below.

## Boot sequence

1. `app.LoadConfig()` resolves the data dir (`xdg.DataFile("yulai")`) and the SSO registration:
   build-time values, then `<config>/yulai/sso.json`, then `YULAI_SSO_*` environment variables.
   A missing client ID is fatal.
2. `app.New()`:
   1. `db.Open(DBPath)` runs migrations.
   2. `crypt.Open(keyring.OS{Service: "eve-online-tools/yulai", User: "master-key"})`
   3. SSO client → verifier → login flow → token store → ESI client
   4. Token store (dead tokens → `Characters.MarkNeedsLogin`), ESI client, character service, scheduler.
   5. `feature.Deps`, then the `[]feature.Feature` list. Every feature's tasks are registered; those with
      scopes go on the picker, in list order.
   6. Login server (`login.New`), handler `Characters.Store` then `Scheduler.Refresh` for the character.
3. Wails app with `a.Services()`, single-instance, and the main window. A second launch exits
   here and focuses the first.
4. `a.Start(ctx)` binds the login port (busy port is fatal) and starts the scheduler goroutine.
5. `wails.Run()`.

## Login flow

SSO runs in the system browser, never in a Wails window: the user can only trust the login page
if they can see the real URL and know the app is not reading their input. A loopback HTTP server
in `identity/login` runs for the whole app lifetime.

1. "Add character" (`ConsentService.AddCharacter`, `identity/login`) opens `http://localhost:45538/` in the system browser.
2. `GET /` renders the feature picker (`apps/webserver`, with the `[]feature.Feature` list as page data).
3. `POST /start` takes the selected features, creates an attempt (`state`, PKCE verifier, scopes)
   and redirects to the SSO authorize URL.
4. `GET /callback` matches `state` to the attempt, exchanges the code, verifies the JWT, hands the
   result to the character service, then redirects to `/done`. The redirect keeps a reload from
   replaying a used code.
5. `/done` names the character and links back to `/` to add another. It notes that EVE SSO
   remembers the account, so adding a character from another account needs an SSO logout first.

Rules:

- Scopes requested are exactly the selected features' scopes. Logging in again replaces the token
  and its scopes, so downscoping is allowed and removes features. A removed feature's tasks stop;
  its stored data is kept.
- Owner hash changes are recorded, not acted on.
- Attempts live in memory with no timeout, since the user can take arbitrarily long at SSO. The map is
  capped (oldest evicted) to bound memory. Unknown `state` and `error=access_denied` render an error
  page with a link to `/`.
- Bind loopback only. Reject requests whose `Host` is not `localhost:45538` or `127.0.0.1:45538`
  (DNS rebinding). The picker form carries a CSRF token.
- The callback URL is `localhost`, which browsers may resolve to `::1`, so bind both `127.0.0.1`
  and `[::1]`.
- Port in use is fatal at boot. Use Wails single-instance so a second launch focuses the running
  app instead of hitting that error.
- No in-app progress for now. The character list updates through the `characters` topic.
- The SSO is the configured host (default `https://login.eveonline.com`), overridable with
  `YULAI_SSO_HOST` at build time, `"ssoHost"` in the SSO file, or `YULAI_SSO_HOST` at runtime. Everything else comes from its
  `/.well-known/openid-configuration`: authorize and token endpoints, `jwks_uri` for signature
  keys, and `issuer`, which must equal the configured one and is what `iss` is validated against.

## Data model (first migration, `core/db/migrations/00001_init.sql`)

Ported from asset-manager, minus `presence`:

- `characters`: id, name, owner_hash, corporation_id, alliance_id, account_label,
  status (`ok` | `needs_login`), status_error, added_at, updated_at
- `tokens`: character_id (FK, cascade), access_token, refresh_token_enc, expires_at,
  scopes (space separated `scp`), issued_at (`iat`)
- `task_pauses`: (kind, value) PK. kind is `task` or `subject`. Scheduling itself is not persisted, see `docs/SCHEDULER.md`.

Each feature then adds its own tables in a new migration (`00002_presence.sql`: `presence_online`, `presence_locations`, `presence_ships`, `00003_charactersheet.sql`: `character_sheets`, `corporations`, `alliances`). Once a table exists, the hand-written
`ListRow` / `SyncJob` structs are replaced by sqlc-generated ones (`sqlc_*.go`, `*_sqlc.go`). Keep the
field names and JSON tags so the bindings don't change. `sqlc.yaml` is added together with the first
`queries.sql` (copy asset-manager's anchor-based layout).

## Adding a feature (the extension point)

1. `feature/<name>/`: `func Feature(d feature.Deps) feature.Feature` returning the ID, picker name, scopes
   (none: always on), task bindings and Wails services. sqlc owns `New` in every package, hence the name.
   Take extra arguments for anything `Deps` does not have.
2. Tasks in the feature package: an unexported receiver type holding dependencies, one file per ESI
   endpoint with `var X = task.New((*Receiver).x, opts...)`. See `docs/SCHEDULER.md`.
3. Tables go in a new migration. Queries go in `feature/<name>/queries.sql` with a matching `sqlc.yaml` entry.
   Follow `docs/CONVENTIONS.md` for ESI calls, task results and table types.
4. Optional Wails `Service` for the UI, with `ServiceName()`, in the feature's `Services`. Call
   `d.Events.Changed(Topic)` after writes that change data.
5. Add one line to the feature list in `app.New`.
6. Frontend: add a key starting with the topic and a `queryOptions` to `apps/yulai/src/queries.ts`. A query
   that also depends on other topics lists them in `meta.topics`. Add a route in
   `apps/yulai/src/router.tsx` and/or a panel in the character card. Gate pages on the feature ID with
   `useFeature` from `apps/yulai/src/features.ts`.

## Frontend conventions

- `frontend/` is a pnpm workspace root. Its `dev`/`build` scripts forward to `@yulai/yulai`, so the
  Wails tasks run unchanged. `apps/yulai` builds to `frontend/dist/yulai` and `apps/webserver` (the loopback
  server pages) to `frontend/dist/webserver`. `main.go` embeds both and hands the webserver build to `identity/login`.
- `@yulai/ui` (`packages/ui`) holds styles and components shared by every app. It exports TS source,
  so it has no build step. It has no Wails dependency; pnpm does not hoist, so importing
  `@wailsio/runtime` or bindings from it fails to resolve. Data comes in through props.
- Bindings are generated into `apps/yulai/bindings` (`-d` in the Taskfiles) and imported as `@bindings/...`.
- `pnpm-workspace.yaml` sets a 7-day `minimumReleaseAge`. `@wailsio/runtime` is exempt and pinned to the
  Wails version in `go.mod`.
- Hash history, so secondary windows open at `/#/<route>`.
- Routes under `layout` get the sidebar shell. Routes directly under root sit in the frame only.
- `/` redirects to `/welcome` while there are no characters. It has no shell and a horizontal stepper:
  add a character, synchronizing (until the character's tasks have all run once this session), complete (redirects
  to `/characters`). Optional onboarding is the sidebar checklist in `components/getting-started`. Its items
  are derived from backend state; only skips and dismissal are kept, in `localStorage`.
- Loaders call `queryClient.ensureQueryData` and components use `useSuspenseQuery`.
- Bindings are generated as classes (`-ts`, no `-i`) and committed.

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
- Imports in use: the root package (`CompatibilityDate`), `transport`, `request`, `middleware/authentication`,
  `middleware/cache`, `middleware/ratelimiting{,/memory}` and the generated `esi/*` endpoints.

## Milestones

1. **Infrastructure:** `core/db` + first migration, `core/keyring`, `core/crypt`, `core/esi` (port and test).
   Uncomment `db.Open` in `app.New`.
2. **Identity:** `identity/sso`, `identity/login`, `identity/token`, sqlc for `tokens`.
3. **Characters:** `character` queries, `Store`/`Remove`/`MarkNeedsLogin`, change events.
4. **Scheduler:** `core/task`, seeds and gates, `TaskService` per `docs/SCHEDULER.md`.
5. **First real feature** using the recipe above.

## Open questions

- **What Yulai's features are.** That decides scopes, tables and pages beyond the shell.
- **Shared module.** `core/*`, `identity/*` and `feature` are app-agnostic and would be
  duplicated with asset-manager. They could be extracted into a shared `eve-online-tools/*` module once both
  apps have stabilised them. Until then, port by copy.
- **Callback port.** Yulai uses `45538` (asset-manager uses `45537`) so both apps can run side by side.
  Each needs its own SSO app registration.
- **Mobile.** The build targets exist from the template, but `keyring.OS` needs a mobile `Store`.
