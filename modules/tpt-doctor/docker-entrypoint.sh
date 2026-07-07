#!/bin/sh
# Entrypoint for the tpt-doctor container: runs migrations, applies the
# e2e/dev seed data (idempotent — safe to run on every start), then starts
# the server.
set -e

tpt-doctor migrate

if [ -f /seed/e2e_seed.sql ] && [ -n "$DATABASE_URL" ]; then
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f /seed/e2e_seed.sql
fi

exec tpt-doctor serve
