# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`migrate-it` is a CLI tool for **comparing and migrating databases**. It reflects two databases into driver-agnostic schema snapshots, diffs them, and emits a migration SQL script whose statement order respects foreign-key dependencies (parents created before children, children dropped before parents). **MySQL, PostgreSQL and SQLite** are all supported; each lives behind the same `Reflector` + `Dialect` interfaces.

## Commands

```bash
go build ./...                          # build everything
go test ./...                           # run all tests
go test ./internal/schema/              # test one package
go test -run TestBuildMigration ./internal/schema/   # run a single test
go vet ./...

# Run the tool: migrate SOURCE schema into TARGET schema. Driver is chosen by DSN scheme.
go run ./cmd -source 'mysql://user:pass@tcp(host:3306)/dbA' -target 'mysql://user:pass@tcp(host:3306)/dbB'
go run ./cmd -source 'postgres://user:pass@host:5432/dbA?sslmode=disable' -target 'postgres://...'
go run ./cmd -source 'sqlite://old.db' -target 'sqlite://new.db'   # or sqlite:path, or a bare *.db path
go run ./cmd -source <dsn> -target <dsn> -format json             # diff as json (also: yaml)
go run ./cmd -source <dsn> -target <dsn> -format html -out d.html # git-diff-style html report
go run ./cmd -source <dsn> -target <dsn> -out migrate.sql          # write to a file instead of stdout
```

The migration transforms **source → target**; the **target's** driver determines the SQL dialect.

Flags: `-format` selects output (`sql` (default) = ordered migration script; `json`/`yaml` = the structural diff; `html` = a self-contained git-diff-style report). `-out` writes to a file (default: stdout). The format is validated up front so a bad `-format` fails before any database is touched.

Testing notes:
- The **MySQL** connection test (`internal/connection/mysql/connection_test.go`) is an integration test: it dials a live MySQL server with hardcoded credentials and fails unless that server is reachable.
- The **SQLite** reflector test (`internal/reflection/sqlite/reflector_test.go`) is a real round-trip test but needs **no server** — `modernc.org/sqlite` is a pure-Go driver, so it seeds a temp-file DB and reflects it. This is the best place to exercise the full reflect→diff pipeline against actual SQL.
- Everything under `internal/schema/` and the dialect/type tests under each `internal/reflection/<driver>/` are pure unit tests.

## Architecture

The pipeline is **reflect → diff → order → render**, split across three packages with a clean dependency direction: `schema` (core, imports nothing internal) ← `reflection` (imports `schema` + `connection`); `connection` is standalone.

- **`internal/connection`** (+`/mysql`, `/postgres`, `/sqlite`) — connectivity only. `Connection` interface (`Connect`/`Disconnect`/`GetTableNames`); each impl additionally exposes `DB() *sql.DB` plus the identifiers its reflector needs (`Database()`, and for Postgres `SchemaName()`). Drivers: MySQL (`go-sql-driver/mysql`), PostgreSQL (`lib/pq`, pure Go), SQLite (`modernc.org/sqlite`, pure Go / no CGO).

- **`internal/schema`** — the driver-agnostic core. This is where most logic lives:
  - **Model**: `Schema` → `Table` → `Column` / `Constraint` / `ForeignKey`. `Column.Definition` holds the full native type (e.g. `varchar(255)`) and is what diffing and DDL rendering use. `*.Equal` methods define what counts as "changed".
  - **`graph.go`** — the **directed dependency graph**. An edge `from → to` means "from must be created before to" (parent → child). `BuildGraph` derives edges from foreign keys (self-references and out-of-schema refs ignored). `TopologicalSort` (Kahn's, deterministic tie-break) returns ordered nodes plus a list of nodes stuck in cycles. `CreationOrder` = parents first; `DropOrder` = its reverse.
  - **`diff.go`** — `Compare(source, target)` produces an unordered `[]Change` (`CreateTable`, `AddColumn`, `AddForeignKey`, …). Ordering is deliberately *not* done here.
  - **`migration.go`** — `BuildMigration(diff, Dialect)` is where the graph earns its keep. It orders statements in five phases so dependencies always hold: (1) drop FKs first, (2) drop tables children-first, (3) create tables parents-first with FKs inlined only when the referent already exists, (4) alter existing tables, (5) add remaining/cyclic FKs last. Cycles are handled by deferring the offending FKs to phase 5.
  - **`dialect.go`** — `Dialect` interface: renders each change kind to SQL. Driver-agnostic; implemented per driver.
  - **`report.go`** — `(*Diff).Report()` returns a `DiffReport`: a serialization-friendly view (source/target names + changes with the kind as a string) used for JSON/YAML output. The model structs carry `json`/`yaml` tags so `DiffReport` can embed them directly.

- **`internal/output`** — renders a `*schema.Diff` in a requested format. `Render(diff, format, dialect)` returns bytes; `Validate(format)` is a cheap up-front check (called before any DB work). `sql` runs `BuildMigration(...).SQL()` and needs the target dialect; `json`/`yaml` marshal `diff.Report()` and ignore the dialect; `html` (`html.go`) builds a self-contained git-diff-style report. Keep all format logic here, not in `cmd`.
  - **`html.go`** groups changes by table into a view model (created/dropped/modified tables → Columns / Indexes & Constraints / Foreign Keys sections → add/del/mod rows) and renders it through an `html/template` with inline CSS (single portable file, light/dark via `prefers-color-scheme`). Dropped tables are re-expanded from `diff.Source`; created tables from `diff.Target`. A changed constraint/FK (which `Compare` emits as a drop+add pair on the same name) is collapsed back into one "mod" row. Element display uses dialect-neutral `*Signature` helpers, **not** SQL — those helpers only read the `schema` model, so they work across all drivers.

- **`internal/reflection`** (+`/mysql`, `/postgres`, `/sqlite`) — `Reflector` interface (`Reflect() (*schema.Schema, error)`) and one package per driver. Each package contains a `reflector.go` (DB → model), a `dialect.go` (model/change → SQL, the `schema.Dialect` impl), and `column_types.go` / `constraints.go` (driver type/constraint constants typed as `schema.ColumnType` / `schema.ConstraintType`). Introspection sources differ per engine: MySQL and Postgres read `information_schema` (Postgres additionally uses `pg_catalog` for reliable composite FK/index column ordering); SQLite has no `information_schema` and uses `PRAGMA table_info` / `index_list` / `index_info` / `foreign_key_list`.

### Driver gotchas worth knowing

- **`Column.Definition` is per-driver native SQL** (MySQL's `COLUMN_TYPE` verbatim; Postgres reconstructed from `data_type` + length/precision; SQLite the declared type). Diffing is therefore only meaningful between two databases of the **same** engine — cross-engine diffs would compare incompatible type strings. The CLI uses the target's dialect for output accordingly.
- **SQLite `ALTER TABLE` is limited**: only ADD/DROP COLUMN and index operations are emitted as real SQL. Modifying a column or adding/dropping a PK or FK requires a table rebuild, so the SQLite dialect returns an explanatory `--` comment instead of invalid SQL. Don't "fix" these into ALTER statements — SQLite will reject them.
- **SQLite foreign keys are anonymous**; the reflector synthesises a stable, content-based name (`fk_<table>_<cols>`), *not* the volatile `PRAGMA` id. Using the id caused logically identical schemas to diff against each other — see `TestReflect_ForeignKeyNamesAreStable`.

### Key invariant to preserve

The foreign-key graph is the single source of truth for statement precedence. When adding change kinds or a new driver, keep ordering decisions in `schema/migration.go` (graph-driven) and keep `Dialect` implementations pure string rendering — a dialect method must never decide *when* a statement runs, only *what* it says. Tests in `migration_test.go` use a recording dialect to assert ordering independently of SQL grammar; mirror that pattern rather than asserting on real SQL when testing order.

### Adding another driver

Implement `reflection.Reflector` (DB → `*schema.Schema`) and `schema.Dialect` (change → SQL) in a new `internal/reflection/<driver>` package, add a connectivity impl under `internal/connection/<driver>`, and wire it into `cmd/main.go`'s `open()` scheme dispatch. The `schema` package needs no changes.
