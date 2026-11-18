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

# run the exporter against one server and record every endpoint into $1
measure() {
  d=$1; dsn=$2; mkdir -p "$d"
  DATA_SOURCE_NAME="$dsn" \
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
}

cleanup() { docker rm -f baseline-primary baseline-replica >/dev/null 2>&1 || true; }
docker network create baseline-net >/dev/null 2>&1 || true

for v in $VERSIONS; do
  cleanup
  # primary, with streaming replication enabled
  docker run -d --name baseline-primary --network baseline-net -p 127.0.0.1:5432:5432 \
    -e POSTGRES_PASSWORD=postgres postgres:$v \
    -c shared_preload_libraries=pg_stat_statements -c pg_stat_statements.track=all \
    -c track_io_timing=on -c wal_level=replica -c max_wal_senders=5 >/dev/null
  until docker exec baseline-primary pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done
  sleep 3
  docker exec baseline-primary sh -c 'echo "host replication all all md5" >> "$PGDATA/pg_hba.conf"'
  docker exec baseline-primary psql -qU postgres -c 'SELECT pg_reload_conf()' >/dev/null
  # some objects and activity, so per-database and table metrics exist
  docker exec baseline-primary psql -qU postgres -c 'CREATE EXTENSION pg_stat_statements' -c 'CREATE DATABASE app'
  docker exec baseline-primary psql -qU postgres -d app \
    -c 'CREATE TABLE t (id serial PRIMARY KEY, v text)' \
    -c "INSERT INTO t (v) SELECT md5(i::text) FROM generate_series(1,10000) i" \
    -c 'ANALYZE t' -c 'SELECT count(*) FROM t' >/dev/null

  # streaming replica, cloned from the primary
  docker run -d --name baseline-replica --network baseline-net -p 127.0.0.1:5433:5432 \
    -u postgres -e PGPASSWORD=postgres --entrypoint sh postgres:$v -c \
    'pg_basebackup -h baseline-primary -U postgres -D /tmp/data -R -X stream && chmod 700 /tmp/data &&
     exec postgres -D /tmp/data -c shared_preload_libraries=pg_stat_statements -c hot_standby=on' >/dev/null
  # md5 in pg_hba also accepts scram passwords; PG13 still stores md5
  n=0; until [ "$(docker exec baseline-primary psql -tAU postgres -c 'SELECT count(*) FROM pg_stat_replication')" = 1 ]; do
    n=$((n+1)); [ $n -gt 120 ] && { echo "pg$v: replica did not start"; docker logs baseline-replica 2>&1 | tail -3; exit 1; }; sleep 1
  done
  until docker exec baseline-replica pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done

  measure "$OUT/pg$v/primary" "postgresql://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable"
  measure "$OUT/pg$v/replica" "postgresql://postgres:postgres@127.0.0.1:5433/postgres?sslmode=disable"
  cleanup
  for r in primary replica; do
    echo "pg$v $r: $(wc -l < "$OUT/pg$v/$r/default.series") series, default scrape $(cat "$OUT/pg$v/$r/default.scrape-median-seconds")s"
  done
done
docker network rm baseline-net >/dev/null 2>&1 || true
