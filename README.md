# Yulai

EVE Online desktop tool. Wails3 + Go backend, React + TypeScript frontend. Same architecture as
[asset-manager](https://github.com/eve-online-tools/asset-manager) (`poc` branch).

**Status: skeleton.** Packages, contracts, wiring and UI shell exist; behaviour is stubbed and
returns `todo.ErrNotImplemented`. See [docs/PLAN.md](docs/PLAN.md) for what goes where and in what order. Code conventions are in [docs/CONVENTIONS.md](docs/CONVENTIONS.md).

## Setup

1. Register an application at https://developers.eveonline.com with callback URL `http://localhost:45538/callback`. PKCE is used, no client secret needed.
2. Copy `.env.example` to `.env` and fill in `YULAI_SSO_CLIENT_ID`. `.env` is gitignored. The Taskfile builds these values into the binary; variables set in the shell override `.env`. `task package` fails without a client ID.
3. Optional: point the app at another SSO, e.g. Singularity, with `YULAI_SSO_HOST`. Default is `https://login.eveonline.com`. A bare host gets `https://`. Endpoints come from the host's `/.well-known/openid-configuration`.

SSO settings are resolved in this order, later wins:

| Source | Client ID | Callback URL | SSO host |
| --- | --- | --- | --- |
| Build time (`.env` or environment) | `YULAI_SSO_CLIENT_ID` | `YULAI_SSO_CALLBACK_URL` | `YULAI_SSO_HOST` |
| `sso.json` in the OS config dir (`%APPDATA%\yulai` on Windows) | `clientId` | `callbackUrl` | `ssoHost` |
| Runtime environment | `YULAI_SSO_CLIENT_ID` | `YULAI_SSO_CALLBACK_URL` | `YULAI_SSO_HOST` |

Without a client ID the app exits at startup.

In GitHub Actions, store the client ID as a secret and pass it to the build step:

```yaml
env:
  YULAI_SSO_CLIENT_ID: ${{ secrets.YULAI_SSO_CLIENT_ID }}
```

Cross-compiles through Docker (`build:docker`) do not get the build-time values.
4. Install tools: `wails3` (v3.0.0-beta.27), pnpm (`corepack enable` picks up the pinned version), and once tables exist `sqlc` and `goose`.

## Develop

```
wails3 dev -config ./build/config.yml
```

After changing a bound Go service, regenerate the TS bindings (`wails3 dev` does this on its own):

```
wails3 task common:generate:bindings
```

Without a webview (e.g. a headless Linux box without GTK/WebKit) the UI can still be checked in a browser through server mode:

```
wails3 task run:server    # http://localhost:8080
```

## Layout

```
app/            config, wiring of all packages
core/           infrastructure, owns no tables
  db/           sqlite open and goose migrations (schema is global)
  esi/          lib-esi-go client with rate limiting and on-disk cache
  keyring/      OS credential store
  crypt/        AES-GCM sealer with its key in the keyring
  task/         scheduler: interval/startup/on-demand tasks, conditions, pauses, pools
  todo/         ErrNotImplemented for skeleton stubs; delete when unused
identity/       who you are and your tokens
  sso/          EVE SSO protocol: discovery, PKCE, token exchange/refresh, JWT verification
  login/        loopback login server: feature picker, SSO redirect, callback
  token/        tokens table, encrypted refresh tokens, RefreshableToken for ESI
feature/        feature.go: Feature and Job contracts. One subpackage per feature
  character/    characters table, add-character flow, list, remove
  sync/         job scheduler, sync_jobs table
frontend/       pnpm workspaces, Vite + React
  apps/yulai/   the Wails app (TanStack Router/Query), builds to frontend/dist/yulai
    bindings/   generated Wails bindings, imported as @bindings/...
  apps/webserver/  loopback server pages (login), builds to frontend/dist/webserver
  packages/ui/  @yulai/ui: shared styles and components, no Wails imports
docs/           PLAN.md, SCHEDULER.md, DESIGN.md
```
