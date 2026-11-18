#!/bin/sh
# Capture a metrics baseline of a postgres_exporter binary against PostgreSQL 13-18,
# run the way PMM runs it (custom queries hr/mr/lr, auto-discover-databases).
# Used to diff each upstream sync PR against the pre-sync Percona exporter.
#
# usage: run.sh <exporter binary> <output dir> [pg versions...]
# needs: docker, curl
set -eu
BIN=$(realpath "$1"); OUT=$2; shift 2
VERSIONS=${*:-13 14 15 16 17 18}
REPO=$(cd "$(dirname "$0")/../.." && pwd)
# lay out the query files the way PMM installs them (percona/pmm build/scripts/install_tarball)
CQ=$(mktemp -d)
mkdir -p $CQ/high-resolution $CQ/medium-resolution $CQ/low-resolution
cp $REPO/queries-postgres-uptime.yml $REPO/queries-hr.yml $CQ/high-resolution/
cp $REPO/queries-mr.yaml $CQ/medium-resolution/
cp $REPO/queries-lr.yaml $CQ/low-resolution/
PORT=19187
mkdir -p "$OUT"
"$BIN" --version > "$OUT/version.txt" 2>&1

scrape() { curl -s -m 120 "http://127.0.0.1:$PORT/$1"; }
# series without values, sorted: what the sync must not lose
series() { grep -v '^#' | sed -E 's/ [^ ]+( [0-9]+)?$//' | sort -u; }

for v in $VERSIONS; do
  d=$OUT/pg$v; mkdir -p "$d"
  docker rm -f baseline-pg >/dev/null 2>&1 || true
  docker run -d --name baseline-pg -p 127.0.0.1:5432:5432 \
    -e POSTGRES_PASSWORD=postgres postgres:$v \
    -c shared_preload_libraries=pg_stat_statements -c pg_stat_statements.track=all \
    -c track_io_timing=on >/dev/null
  until docker exec baseline-pg pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done
  sleep 3
  # some objects and activity, so per-database and table metrics exist
  docker exec baseline-pg psql -qU postgres -c 'CREATE EXTENSION pg_stat_statements' -c 'CREATE DATABASE app'
  docker exec baseline-pg psql -qU postgres -d app \
    -c 'CREATE TABLE t (id serial PRIMARY KEY, v text)' \
    -c "INSERT INTO t (v) SELECT md5(i::text) FROM generate_series(1,10000) i" \
    -c 'ANALYZE t' -c 'SELECT count(*) FROM t' >/dev/null

  DATA_SOURCE_NAME="postgresql://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable" \
  "$BIN" --web.listen-address=127.0.0.1:$PORT \
    --auto-discover-databases \
    --exclude-databases=template0,template1,postgres,cloudsqladmin,pmm-managed-dev,azure_maintenance,rdsadmin \
    --collect.custom_query.hr --collect.custom_query.hr.directory=$CQ/high-resolution \
    --collect.custom_query.mr --collect.custom_query.mr.directory=$CQ/medium-resolution \
    --collect.custom_query.lr --collect.custom_query.lr.directory=$CQ/low-resolution \
    --log.level=warn > "$d/exporter.log" 2>&1 &
  pid=$!
  until scrape metrics >/dev/null 2>&1; do sleep 1; done

  # the same endpoints pmm-agent scrapes, plus the plain default
  for e in "default:metrics" \
           "hr:metrics?collect%5B%5D=custom_query.hr&collect%5B%5D=exporter&collect%5B%5D=standard.go&collect%5B%5D=standard.process" \
           "mr:metrics?collect%5B%5D=custom_query.mr" \
           "lr:metrics?collect%5B%5D=custom_query.lr"; do
    name=${e%%:*}; path=${e#*:}
    scrape "$path" > "$d/$name.prom"
    series < "$d/$name.prom" > "$d/$name.series"
    # scrape time, 5 runs
    for i in 1 2 3 4 5; do curl -s -m 120 -o /dev/null -w '%{time_total}\n' "http://127.0.0.1:$PORT/$path"; done \
      | sort -n | sed -n 3p > "$d/$name.scrape-median-seconds"
  done

  kill $pid; wait $pid 2>/dev/null || true
  docker rm -f baseline-pg >/dev/null
  echo "pg$v: $(wc -l < "$d/default.series") series, default scrape $(cat "$d/default.scrape-median-seconds")s"
done
