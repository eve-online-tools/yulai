# Static data (SDE) plan

`core/sde` keeps a local copy of CCP's Static Data Export in typed sqlite tables: every file, every language,
always the latest build.

## Source

CCP publishes the SDE at `https://developers.eveonline.com/static-data/`. Docs:
https://developers.eveonline.com/docs/services/static-data/

| URL (under `/static-data/tranquility/`) | Content |
|---|---|
| `latest.jsonl` | `{"_key":"sde","buildNumber":3569502,"releaseDate":"2026-10-02T11:08:57Z"}`. ETag and Last-Modified, `max-age=300` |
| `eve-online-static-data-<build>-jsonl.zip` | The export. ~99 MB, ~580 MB unpacked, ~100 `.jsonl` files |
| `changes/<build>.jsonl` | Changed keys per file. `_meta` holds `lastBuildNumber` |
| `schema-changelog.yaml` | Schema changes per build |

Each line is one record with its key in `_key`. Keys are integers, except in `_sde`, `characterTitles`,
`militaryCampaigns`, `militaryCampaignObjectives` and `translationLanguages`, where they are strings.
Localized fields are objects keyed by language (`{"de":…,"en":…,…}`). Builds ship several times a week.

## Storage

One file, `<data dir>/sde.sqlite`, separate from `yulai.sqlite`:

- Its schema follows CCP's files and the generator, not goose migrations.
- Deleting it is a safe reset; the next check downloads it again.
- Rebuilds do not bloat the app database or hold its single writer.

### Schema

Typed tables with a column per field, generated from the data:

- One table per file, named after it: `types`, `mapSolarSystems`, … `_key` becomes `key`, the primary key.
- Nested objects are flattened: `position.x` → `position_x`.
- Localized objects become a column per language: `name_en`, `name_de`, …
- Arrays become child tables named after the path: `typeDogma.dogmaAttributes` →
  `typeDogma_dogmaAttributes(parent, idx, attributeID, value)`. `parent` is the owning row's key, `idx` the
  position. Elements with a `_key` (CCP's form of integer-keyed maps) get a `key` column instead of `idx`.
  Scalar elements and scalar `_value`s go in `value`. Child tables that have children of their own get an
  `id INTEGER PRIMARY KEY`, which their children use as `parent`.
- Column types are inferred: INTEGER, REAL (also for fields mixing integers and floats), TEXT, BOOLEAN.
  ID fields use the `sqlc.yaml` domain types (`typeID` → `TYPE_ID`, `solarSystemID` → `SOLAR_SYSTEM_ID`, …),
  as do the keys of the matching tables (`types.key` is `TYPE_ID`).
- Every column except `key` and `parent` is nullable, so a field CCP stops sending does not break a build.
- Indexes on every `parent` column.
- `meta(build, release_date, schema)`. `schema` is a hash of `schema.sql`.

Measured on build 3569502: 229 tables, 343 MB with indexes, built in about 55 s. `types` is 163 MB (descriptions in 8 languages),
`mapMoons` 65 MB, `missions_messages` 57 MB. SQLite with a pure Go driver has no page compression;
zstd-compressing long text values per row would save ~45 MB on `types` but makes them unsearchable, so it is
not done.

### Generator

`go generate ./core/sde` runs `core/sde/gen` against a downloaded SDE zip and writes, committed:

- `schema.sql`: the tables, used by the converter and by sqlc.
- `tables_gen.go`: per file, the tables it fills and which JSON path feeds which column.

At runtime, fields not in the generated schema are dropped and logged once per field; missing fields are NULL.
Files not in the schema are skipped. Regenerating against a newer SDE picks up schema changes;
`schema-changelog.yaml` explains them.

### Reading

`sde.Store` holds the open database (`query_only`) behind a read/write lock.
`Store.Read(ctx, fn)` runs `fn` with the sqlc `*Queries` under the read lock, so a swap never closes a database
mid-query. Before the first install it returns `sde.ErrNotInstalled`; `sde.Installed` is a `task.Condition` for
tasks that need static data. Queries go in `core/sde/queries.sql`, with an `sqlc.yaml` entry over
`core/sde/schema.sql`.

## Tasks

| Task | Schedule | Does |
|---|---|---|
| `Check` | startup, every hour | GET `latest.jsonl` with the last ETag. Queues `Update` when the build differs from `meta.build` or `meta.schema` differs from the compiled schema |
| `Update` | on demand, 30 min timeout | Downloads the zip, builds `sde.sqlite.tmp` from it, swaps it in, emits `sde:changed` |

`Check` is `Pausable("sde.check")`; pausing it stops automatic updates. `Update` has no schedule, so it is not
pausable.

`Update` phases:

1. **Download** to `sde-<build>.zip.part`, resuming with `Range` after an interruption. A build's zip never
   changes, so no `If-Range` is needed. A 200 instead of a 206 restarts from zero.
2. **Build**: read each zip entry straight into `sde.sqlite.tmp`, one transaction per file. No extraction to disk.
3. **Index**: create indexes after the inserts.
4. **Swap**: take the write lock, close the database, rename `sde.sqlite.tmp` over `sde.sqlite`, reopen,
   release. Closing first is needed on Windows, which refuses to rename over an open file. The zip is deleted.

Opening the store deletes a leftover `sde.sqlite.tmp` and zips of other builds; a `.zip.part` is kept for resume.

## Progress

Progress is a `core/task` feature. A task opts in with a named key, and the UI tracks bars by key:

```go
var Update = task.New((*Updater).Update,
	task.WithTimeout(30*time.Minute),
	task.WithProgress("sde.update"),
)

type Progress struct {
	Phase string `json:"phase"` // e.g. "download", "build", "index"
	Item  string `json:"item"`  // e.g. "types.jsonl"
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

func Report(ctx context.Context, p Progress)
```

- A run of a task with `WithProgress(key)` emits **start** when it begins and **done** (last progress, error)
  when it ends. The UI shows the bar for `key` on start and hides it on done.
- In between, the task reports through `task.Report(ctx, …)`, which emits **update** (at most every 100 ms).
  Functions it calls get the same ctx, and sub-tasks it queues without a key of their own report into the same
  bar.
- `Report` outside such a run does nothing, so progress is opt-in.
- `app` forwards the three as `progress:start`, `progress:update` and `progress:done` Wails events carrying
  `{key, subject, state, progress, error}`. `ProgressService.Get(key)` returns the current progress for a UI that
  opens mid-run.

`Update` reports one deterministic bar for the whole run: `Done / Total` is the overall fraction, scaled to
10000, and `Phase` and `Item` name the current step. Each phase fills a fixed slice:

| Phase | Slice | Fraction within the slice |
|---|---|---|
| download | 0-50% | bytes on disk / zip size (`Content-Length`, or the total in `Content-Range` when resuming) |
| build | 50-90% | uncompressed bytes decoded so far (decoder input offset in the current file plus finished files) / sum of entry sizes in the zip's central directory |
| index | 90-100% | indexes created / indexes in the schema |

A zip already on disk skips to 50%. The bar never goes back or becomes indeterminate.

`sde.Service` (`SDEService`) has `Status()`: installed build and release date, latest build seen, last check and
the last error. `CheckNow()` queues `Check`.

While `Update` runs, the shell and the welcome screen show a bar pinned below the scrolling content:
"Updating data", the phase and item, and the update bar with its percentage.

## Steps

1. `core/task`: `Progress`, `Report`, `Status.Progress`.
2. Generator and generated schema.
3. Converter and `Store`: build from a zip, indexes, `meta`, swap, progress.
4. `Check` and `Update` tasks, `SDEService`, wiring in `app.New`, `task:changed` and `sde:changed` events.
5. Frontend: updating-data bar in the shell and on the welcome screen.

Later: use `changes/<build>.jsonl` to skip the download when no file changed, and a CI job that regenerates
the schema against the latest SDE.
