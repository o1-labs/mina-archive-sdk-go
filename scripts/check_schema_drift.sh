#!/usr/bin/env bash
# Compare the vendored `schema.graphql` against the upstream copy in
# `o1-labs/Archive-Node-API@main`. Exits non-zero on drift so a CI workflow
# can open a sync PR.

set -euo pipefail

UPSTREAM_URL="${UPSTREAM_SCHEMA_URL:-https://raw.githubusercontent.com/o1-labs/Archive-Node-API/main/schema.graphql}"
LOCAL_SCHEMA="$(dirname "$0")/../schema.graphql"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

curl --fail --silent --show-error --location "$UPSTREAM_URL" -o "$tmp"

if diff -u "$LOCAL_SCHEMA" "$tmp" >/dev/null; then
  echo "schema.graphql in sync with upstream"
  exit 0
fi

echo "schema drift detected:"
diff -u "$LOCAL_SCHEMA" "$tmp" || true
exit 1
