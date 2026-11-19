#!/bin/bash
# List the upstream pull requests contained in a range of upstream tags, as
# Markdown for a sync PR description. Dependency bumps are listed separately.
# usage: upstream-prs.sh <from tag> <to tag>
set -eu
U=https://github.com/prometheus-community/postgres_exporter
git log --no-merges --reverse --format='%s%x09%an' "$1..$2" > /tmp/upstream-prs.$$
echo "| Upstream PR | Change | Author |"
echo "|---|---|---|"
grep -viE $'\t(dependabot|renovate)' /tmp/upstream-prs.$$ | while IFS="$(printf '\t')" read -r s a; do
  n=$(echo "$s" | grep -oE '\(#[0-9]+\)$' | tr -d '()#' || true)
  t=$(echo "$s" | sed -E 's/ *\(#[0-9]+\)$//; s/\|/\\|/g')
  if [ -n "$n" ]; then echo "| [#$n]($U/pull/$n) | $t | $a |"; else echo "| (direct commit) | $t | $a |"; fi
done
b=$(grep -ciE $'\t(dependabot|renovate)' /tmp/upstream-prs.$$ || true)
if [ "$b" -gt 0 ]; then
  echo; echo "<details><summary>$b dependency updates (dependabot)</summary>"; echo
  grep -iE $'\t(dependabot|renovate)' /tmp/upstream-prs.$$ | while IFS="$(printf '\t')" read -r s a; do
    n=$(echo "$s" | grep -oE '\(#[0-9]+\)$' | tr -d '()#' || true)
    echo "- $(echo "$s" | sed -E "s|\(#([0-9]+)\)$|([#\1]($U/pull/\1))|")"
  done
  echo; echo "</details>"
fi
rm -f /tmp/upstream-prs.$$
