# Yulai

EVE Online desktop tool. Wails3 + Go backend, React + TypeScript frontend. Same architecture as
[asset-manager](https://github.com/eve-online-tools/asset-manager) (`poc` branch).

**Status: skeleton.** Packages, contracts, wiring and UI shell exist; behaviour is stubbed and
returns `todo.ErrNotImplemented`. See [docs/PLAN.md](docs/PLAN.md) for what goes where and in what order.

## Setup

1. Register an application at https://developers.eveonline.com with callback URL `http://localhost:45538/callback`. PKCE is used, no client secret needed.
2. Copy `sso.example.json` to `sso.dev.json` and fill in `clientId`. `sso.dev.json` is gitignored. Without it the app looks for `sso.json` in the OS config dir (`%APPDATA%\yulai` on Windows). A missing file logs a warning and the login page reports it.
3. Optional: point the app at another SSO with `"issuer"` in the SSO file or the `YULAI_SSO_ISSUER` environment variable (which wins). Default is `https://login.eveonline.com`. A bare host gets `https://`. Endpoints come from the issuer's `/.well-known/openid-configuration`.
4. Install tools: `wails3` (v3.0.0-beta.27), and once tables exist `sqlc` and `goose`.

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
frontend/       Vite + React + TanStack Router/Query
docs/           PLAN.md, SCHEDULER.md
```
