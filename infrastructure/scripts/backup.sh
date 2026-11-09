#!/bin/sh
set -eu
umask 077
mkdir -p backups
backup_file="backups/journey-$(date -u +%Y%m%dT%H%M%SZ).dump"
trap 'rm -f "$backup_file.partial"' EXIT
docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_file.partial"
mv "$backup_file.partial" "$backup_file"
printf 'Backup created: %s\n' "$backup_file"
