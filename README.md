# migrate-it

[![CI](https://github.com/guiiamorim/migrate-it/actions/workflows/ci.yml/badge.svg)](https://github.com/guiiamorim/migrate-it/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**A CLI for comparing database schemas and generating dependency-aware migrations.**

`migrate-it` connects to two databases, reflects each into a driver-agnostic model, and computes the difference between them. From that diff it can produce an **executable SQL migration script** whose statements are ordered to respect foreign-key dependencies, or export the diff as **JSON**, **YAML**, or a **git-diff-style HTML report**.

It works with **MySQL**, **PostgreSQL**, and **SQLite**.

---

## Table of contents

- [Why](#why)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [DSN formats](#dsn-formats)
  - [Flags](#flags)
- [Output formats](#output-formats)
  - [SQL migration](#sql-migration)
  - [JSON / YAML diff](#json--yaml-diff)
  - [HTML report](#html-report)
- [How it works](#how-it-works)
- [Driver support & caveats](#driver-support--caveats)
- [Development](#development)
- [License](#license)

---

## Why

Hand-writing migrations between two schema states is tedious and error-prone — especially getting the **order** right. You can't create a child table before its parent, drop a parent before its children, or add a foreign key before the table it references exists. `migrate-it` models these relationships as a directed graph and orders the generated statements so the migration applies cleanly in a single pass.

## Features

- **Schema reflection** for MySQL, PostgreSQL, and SQLite — tables, columns, primary keys, unique/plain indexes, and foreign keys (with `ON DELETE` / `ON UPDATE` rules).
- **Structural diff** between a source and target schema: tables, columns, constraints/indexes, and foreign keys added, dropped, or modified.
- **Dependency-ordered SQL migrations.** A directed foreign-key graph drives the ordering so parents are created before children, children dropped before parents, and foreign keys added only once their targets exist. Cyclic foreign keys are detected and deferred to `ALTER TABLE ... ADD` after the tables are created.
- **Multiple output formats:** `sql` (migration script), `json` and `yaml` (the structural diff), and `html` (a self-contained, git-diff-style report with light/dark themes).
- **Driver-selected by DSN scheme** — mix and match `mysql://`, `postgres://`, and `sqlite://` endpoints.
- **Write to stdout or a file** via `-out`.

## Installation

Requires **Go 1.25+**.

```bash
# Build from a clone
git clone https://github.com/guiiamorim/migrate-it.git
cd migrate-it
go build -o migrate-it ./cmd
```

Or run without building:

```bash
go run ./cmd -source <dsn> -target <dsn>
```

`go install github.com/guiiamorim/migrateit/cmd@latest` also works, but installs the binary as `cmd` (Go names it after the package directory) — rename it to `migrate-it` if you install this way. All database drivers are pure Go (or cgo-free), so no external C libraries are needed.

## Usage

```bash
migrate-it -source <dsn> -target <dsn> [-format sql|json|yaml|html] [-out file]
```

The migration transforms the **source** schema into the **target** schema (i.e. "make source look like target"). The **target's** driver determines the dialect of any generated SQL.

```bash
# Print the SQL migration that upgrades the "prod" schema to match "staging"
migrate-it \
  -source 'mysql://user:pass@tcp(prod-host:3306)/app' \
  -target 'mysql://user:pass@tcp(staging-host:3306)/app'

# Export the diff as JSON
migrate-it -source 'postgres://…/old' -target 'postgres://…/new' -format json

# Produce an HTML report and save it to a file
migrate-it -source 'sqlite://old.db' -target 'sqlite://new.db' -format html -out diff.html
```

### DSN formats

The driver is chosen by the DSN scheme:

| Database   | DSN example                                                        |
|------------|-------------------------------------------------------------------|
| MySQL      | `mysql://user:pass@tcp(localhost:3306)/dbname`                     |
| PostgreSQL | `postgres://user:pass@localhost:5432/dbname?sslmode=disable`       |
| SQLite     | `sqlite://path/to/file.db` (also `sqlite:path`, a bare `*.db` / `*.sqlite` path, or `:memory:`) |

For PostgreSQL, the schema/namespace defaults to `public`; override it with a `search_path` query parameter.

### Flags

| Flag       | Default  | Description                                                        |
|------------|----------|-------------------------------------------------------------------|
| `-source`  | —        | Source DSN — the schema to migrate **from**. Required.            |
| `-target`  | —        | Target DSN — the schema to migrate **to**. Required.             |
| `-format`  | `sql`    | Output format: `sql`, `json`, `yaml`, or `html`.                  |
| `-out`     | *stdout* | Write output to this file instead of standard output.            |

The format is validated before any database connection is made, so a typo fails fast.

## Output formats

### SQL migration

The default. Emits an ordered migration script for the target's dialect:

```sql
DROP TABLE `legacy`;
CREATE TABLE `products` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `sku` varchar(64) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB;
CREATE TABLE `orders` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL,
  PRIMARY KEY (`id`),
  CONSTRAINT `fk_orders_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB;
ALTER TABLE `users` ADD COLUMN `email` varchar(255) NOT NULL;
```

Notice `products` and `orders` are created after the tables they depend on, and each statement is dependency-safe.

### JSON / YAML diff

A structural view of the changes (source/target names plus each change with its kind and payload), suitable for tooling or review:

```json
{
  "source": "app@prod",
  "target": "app@staging",
  "changes": [
    { "kind": "ADD COLUMN", "table": "users",
      "column": { "name": "email", "type": "VARCHAR", "definition": "varchar(255)", "nullable": false } },
    { "kind": "ADD FOREIGN KEY", "table": "orders",
      "foreignKey": { "name": "fk_orders_user", "columns": ["user_id"], "refTable": "users", "refColumns": ["id"], "onDelete": "CASCADE" } }
  ]
}
```

`-format yaml` produces the same structure in YAML.

### HTML report

`-format html` renders a self-contained, single-file report styled like a GitHub diff: a summary bar, one card per changed table with a `created` / `dropped` / `modified` badge, and grouped **Columns**, **Indexes & Constraints**, and **Foreign Keys** sections. Added elements appear as green `+` lines, removed as red `-` lines, and modified columns show both the old and new definition. It adapts to light and dark color schemes automatically and needs no external assets.

## How it works

The pipeline is **reflect → diff → order → render**:

1. **Reflect** — each database is introspected into a driver-agnostic model (`Schema → Table → Column / Constraint / ForeignKey`). MySQL and PostgreSQL read `information_schema` (Postgres also uses `pg_catalog` for reliable composite key/index ordering); SQLite uses the `PRAGMA` family.
2. **Diff** — the source and target models are compared into a list of changes.
3. **Order** — a **directed foreign-key graph** is built (an edge means "must exist before"). A topological sort yields creation order (parents first) and its reverse for drops. The migration planner sequences statements in dependency-safe phases: drop foreign keys, drop tables (children first), create tables (parents first, inlining foreign keys where the target already exists), alter existing tables, then add remaining/cyclic foreign keys.
4. **Render** — the diff (or planned migration) is emitted in the requested format.

## Driver support & caveats

| Capability            | MySQL | PostgreSQL | SQLite |
|-----------------------|:-----:|:----------:|:------:|
| Reflect schema        |  ✅   |     ✅     |   ✅   |
| Generate migration    |  ✅   |     ✅     |   ⚠️   |
| Diff (json/yaml/html) |  ✅   |     ✅     |   ✅   |

- **Diffs are meaningful within the same engine.** A column's type is stored as the engine's native definition (e.g. `varchar(255)`), so comparing a MySQL schema against a PostgreSQL one would compare incompatible type strings. Use matching source/target engines.
- **SQLite `ALTER TABLE` is limited.** Adding/dropping columns and indexes is fully supported, but modifying a column or adding/dropping a primary or foreign key requires a table rebuild. Rather than emit invalid SQL, `migrate-it` outputs an explanatory `--` comment for those operations.
- **Foreign-key cycles** (e.g. two tables that reference each other) are handled by creating the tables first and adding the cyclic foreign keys afterwards.

## Development

```bash
go build ./...          # build
go test ./...           # run tests
go test -cover ./...    # with coverage
go vet ./...
gofmt -l .              # formatting check (should print nothing)
```

Run a single test:

```bash
go test -run TestBuildMigration ./internal/schema/
```

The SQLite tests run anywhere (pure-Go driver, temp-file databases). The MySQL connection test is an opt-in integration test — set `MIGRATEIT_MYSQL_DSN` to a reachable server to run it; otherwise it is skipped.

Continuous integration (GitHub Actions) runs formatting, vet, build, and race-enabled tests with a coverage floor on every pull request; `master` is protected and requires a passing build to merge.

## License

[MIT](LICENSE) © Guilherme Amorim
