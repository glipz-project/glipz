#!/bin/sh
# Run from the repository root. The destination must be a new directory.
set -eu
umask 077
cd "$(dirname "$0")/.."
env_file=${GLIPZ_ENV_FILE:-.env}
compose() { docker compose --env-file "$env_file" "$@"; }
destination=${1:-"backups/$(date -u +%Y%m%dT%H%M%SZ)"}
mkdir -p "$(dirname "$destination")"
mkdir "$destination"
destination=$(cd "$destination" && pwd)
python3 scripts/check-production.py --env-file "$env_file"
compose ps --status running --services | grep -qx backend || {
  echo 'Backend must be running before backup; no services were changed.' >&2
  exit 1
}
# Brief maintenance window keeps local media and database references consistent.
compose stop backend
trap 'compose start backend >&2' EXIT
compose exec -T postgres pg_dump -U glipz -d glipz -Fc > "$destination/database.dump"
compose run --rm --no-deps -T --entrypoint tar backend \
  -C /app/data/media -czf - . > "$destination/media.tar.gz"
cp "$env_file" "$destination/environment.env"
tar -C data -czf "$destination/legal-docs.tar.gz" legal-docs
git rev-parse HEAD > "$destination/revision.txt"
echo "Backup complete: $destination. Encrypt and copy it off-server. Redis and TLS state are recreated on restore."
