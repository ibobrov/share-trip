#!/bin/sh
set -eu

# Pod readiness can precede availability through the Service.
attempt=0
until pg_isready -q -t 1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 30 ]; then
        echo "PostgreSQL is unavailable through the Service after 30 checks" >&2
        exit 1
    fi
    sleep 1
done

exec psql --no-psqlrc --set=ON_ERROR_STOP=1 --file=/scripts/init.sql
