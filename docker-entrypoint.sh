#!/bin/sh
set -eu

echo "migrate: mulai"
/app/migrate up
echo "migrate: selesai"

exec /app/api
