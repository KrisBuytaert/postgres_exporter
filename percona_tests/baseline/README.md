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

## main-22fb54d

Percona `main` at 22fb54d (VERSION 0.15.0), captured 2026-11-18 on AWS EC2 (Ubuntu 24.04,
t3.medium, Docker `postgres:13`…`postgres:18`).

| PG | default | hr | mr | lr | default scrape |
|---|---|---|---|---|---|
| 13 | 637 | 388 | 11 | 36 | 0.098 s |
| 14 | 650 | 399 | 11 | 36 | 0.130 s |
| 15 | 655 | 403 | 11 | 36 | 0.126 s |
| 16 | 666 | 414 | 11 | 36 | 0.132 s |
| 17 | 679 | 429 | 11 | 36 | 0.131 s |
| 18 | 706 | 443 | 11 | 36 | 0.133 s |

No errors in the exporter log on any version.

## Known gaps

- Single server, no replica: `pg_custom_stat_wal_receiver`, `pg_custom_stat_replication` and
  the streaming-replication collectors are absent. A primary/replica pair would be needed.
- No Aurora/RDS target.
- `percona_tests/custom-queries/medium-resolution/queries.yaml` fails to load
  (`yaml: unmarshal errors`), so the existing `percona_tests` harness does not really
  exercise medium-resolution custom queries. This baseline uses the shipped
  `queries-*.y*ml` files instead.
