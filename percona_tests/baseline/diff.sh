#!/bin/bash
# Compare two baseline runs: which metric names and series disappeared or
# appeared, per PostgreSQL version, node and endpoint, plus scrape times.
# usage: diff.sh <old run dir> <new run dir>
set -eu
export LC_ALL=C
# Series compared after masking labels that differ between any two runs
# (pids, the temp query dir) and dropping per-connection activity series,
# whose label combinations depend on which sessions exist at scrape time.
# Their metric names are still compared.
s() {
  grep -vE '^(pg_(custom_)?stat_activity|postgres_exporter_build_info)' "$1" |
    sed -E 's/pid="[0-9]+"/pid="…"/g; s/^(pg_stat_replication_[a-z_]+\{.*)slot_name="[0-9]+"/\1slot_name="…"/; s#/tmp/tmp\.[A-Za-z0-9]+/#/tmp/…/#g' | sort -u
}
OLD=$1; NEW=$2
names() { cut -d'{' -f1 "$1" | sort -u; }
printf '# %s → %s\n\n' "$(head -1 "$OLD/version.txt")" "$(head -1 "$NEW/version.txt")"
printf '| PG | node | endpoint | series old → new | names lost | names added | series lost | scrape old → new |\n|---|---|---|---|---|---|---|---|\n'
for d in "$OLD"/pg*/*/; do
  rel=${d#"$OLD"/}; rel=${rel%/}; pg=${rel%%/*}; node=${rel#*/}
  for e in default hr mr lr; do
    o=$OLD/$rel/$e.series; n=$NEW/$rel/$e.series
    [ -f "$n" ] || { echo "| $pg | $node | $e | missing in new run | | | | |"; continue; }
    nl=$(comm -23 <(names "$o") <(names "$n") | wc -l)
    na=$(comm -13 <(names "$o") <(names "$n") | wc -l)
    sl=$(comm -23 <(s "$o") <(s "$n") | wc -l)
    printf '| %s | %s | %s | %s → %s | %s | %s | %s | %s → %s |\n' "$pg" "$node" "$e" \
      "$(wc -l < "$o")" "$(wc -l < "$n")" "$nl" "$na" "$sl" \
      "$(cat "$OLD/$rel/$e.scrape-median-seconds")" "$(cat "$NEW/$rel/$e.scrape-median-seconds")"
  done
done
printf '\n## Metric names lost (any version, node or endpoint)\n\n'
for f in "$OLD"/pg*/*/*.series; do rel=${f#"$OLD"/}; [ -f "$NEW/$rel" ] && comm -23 <(names "$f") <(names "$NEW/$rel"); done | sort | uniq -c | sort -rn | sed 's/^/    /'
printf '\n## Metric names added\n\n'
for f in "$OLD"/pg*/*/*.series; do rel=${f#"$OLD"/}; [ -f "$NEW/$rel" ] && comm -13 <(names "$f") <(names "$NEW/$rel"); done | sort | uniq -c | sort -rn | sed 's/^/    /'
printf '\n## Series lost with label changes (same name, different labels), first 40\n\n'
for f in "$OLD"/pg*/*/*.series; do rel=${f#"$OLD"/}; [ -f "$NEW/$rel" ] || continue
  comm -23 <(s "$f") <(s "$NEW/$rel") | while read -r s; do nm=${s%%\{*}; grep -q "^$nm[{ ]\|^$nm\$" "$NEW/$rel" && echo "$s"; done
done | sort -u | head -40 | sed 's/^/    /'
