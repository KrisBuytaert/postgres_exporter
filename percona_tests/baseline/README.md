# Upstream sync baseline

Reference metrics for re-syncing with upstream prometheus-community/postgres_exporter
(v0.15.0 → v0.20.1, one PR per upstream minor release). **Not meant to be merged**: each
sync PR is diffed against this.

`run.sh <binary> <outdir>` starts PostgreSQL 13–18 in Docker, runs the exporter the way
PMM does (custom queries hr/mr/lr laid out as pmm-client installs them, plus
`--auto-discover-databases`), and records for each version:

| File | Content |
|---|---|
| `<endpoint>.prom` | raw scrape |
| `<endpoint>.series` | metric names + labels, values stripped, sorted: the set a sync must not lose |
| `<endpoint>.scrape-median-seconds` | median of 5 scrapes |
| `exporter.log` | exporter output (warn and up) |

Endpoints: `default` (plain `/metrics`) and `hr`, `mr`, `lr` (the `collect[]` URLs pmm-agent scrapes).

## Runs

Captured 2026-11-18 on AWS EC2 (Ubuntu 24.04, t3.medium, Docker `postgres:13`…`postgres:18`),
each version as a primary plus a streaming replica on a physical slot.

| Run | Binary | Diff |
|---|---|---|
| `main-22fb54d` | Percona `main` at 22fb54d (VERSION 0.15.0), the pre-sync reference | |
| `sync-v0.16.0` | `sync/v0.16.0` at 283c913 (merge of upstream v0.16.0) | [`diff-main-to-v0.16.0.md`](diff-main-to-v0.16.0.md): no metric names or series lost; adds `pg_roles_connection_limit`, `pg_database_connection_limit`, `pg_stat_database_active_time_seconds_total`, `pg_replication_slot_wal_status`; scrape time +0.3% |

`diff.sh <old> <new>` masks labels that change between any two runs (pids, the temporary
query directory, the walsender pid that upstream exposes as `slot_name` in
`pg_stat_replication_*`) and compares `pg_stat_activity*` by metric name only, since its
label combinations depend on which sessions exist at scrape time.

## Known gaps

- No Aurora/RDS target.
- `percona_tests/custom-queries/medium-resolution/queries.yaml` fails to load
  (`yaml: unmarshal errors`), so the existing `percona_tests` harness does not really
  exercise medium-resolution custom queries. This baseline uses the shipped
  `queries-*.y*ml` files instead.
