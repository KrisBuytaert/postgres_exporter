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

Captured 2026-11-19 on one AWS EC2 host (Ubuntu 24.04, t3.medium, Docker `postgres:13`…`postgres:18`),
each version as a primary plus a streaming replica on a physical slot. Raw scrapes (`*.prom`) are
not committed; the `.series` files hold every metric name and label set.

| Run | Binary | Step diff | Names lost | Names added |
|---|---|---|---|---|
| `run-main` | Percona `main` 22fb54d, pre-sync reference | | | |
| `run-v0.16.0` | `sync/v0.16.0` 20d254e | [diff](diff-main-to-v0.16.0.md) | 0 | `pg_roles_connection_limit`, `pg_database_connection_limit`, `pg_stat_database_active_time_seconds_total`, `pg_replication_slot_wal_status` |
| `run-v0.17.1` | `sync/v0.17.1` c45a1cb | [diff](diff-v0.16.0-to-v0.17.1.md) | 0 | 1 |
| `run-v0.18.1` | `sync/v0.18.1` ef8ccda | [diff](diff-v0.17.1-to-v0.18.1.md) | 0 | 0 |
| `run-v0.19.1` | `sync/v0.19.1` cde34e2 | [diff](diff-v0.18.1-to-v0.19.1.md) | 0 | 0 |
| `run-v0.20.1` | `sync/v0.20.1` 60e4a17 | [diff](diff-v0.19.1-to-v0.20.1.md) | 2, not used by PMM | `pg_replication_slots_*`, `pg_replication_last_replay_seconds` |

Overall: [main → v0.20.1](diff-main-to-v0.20.1.md). Every `pg_*` metric that PMM's dashboards, alert
templates and advisor checks use and that `main` produces (107 names) is still produced by v0.20.1, on
the same pmm-agent endpoints. v0.20.1 drops `pg_stat_archiver_last_archive_age` and
`pg_stat_replication_reply_time` (unused by PMM) and, in upstream's new settings, replication and
replication_slots collectors, the `server` label (unused by PMM for PostgreSQL).

This baseline caught one real regression: from v0.19.1 every custom query failed because the Percona
handler did not pass `--metric-prefix` to its exporters; fixed in cde34e2/60e4a17.

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
