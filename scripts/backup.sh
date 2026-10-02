#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE=${ENV_FILE:-"$ROOT_DIR/.env"}
COMPOSE_FILE=${COMPOSE_FILE:-"$ROOT_DIR/build/compose.yaml"}
COMPOSE_EXTRA=${COMPOSE_EXTRA:-}
BACKUP_ROOT=${1:-"$ROOT_DIR/backups"}
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP_DIR="$BACKUP_ROOT/$STAMP"

if [ ! -f "$ENV_FILE" ]; then
	echo "Environment file not found: $ENV_FILE" >&2
	exit 1
fi

set -a
. "$ENV_FILE"
set +a
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

compose() {
	if [ -n "$COMPOSE_EXTRA" ]; then
		docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" -f "$COMPOSE_EXTRA" "$@"
	else
		docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
	fi
}

restart_uploader() {
	compose start uploader >/dev/null 2>&1 || true
}
trap restart_uploader EXIT INT TERM

compose stop uploader >/dev/null
compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$BACKUP_DIR/database.dump"
compose run --rm --no-deps --entrypoint sh \
	-v "$BACKUP_DIR:/backup" uploader \
	-c 'tar -czf /backup/media.tar.gz -C "$ENV_MEDIA_DIR" . && tar -czf /backup/uploads.tar.gz -C "$ENV_TMP_DIR" .'
cp "$ENV_FILE" "$BACKUP_DIR/environment.env"
chmod 600 "$BACKUP_DIR/environment.env"
(cd "$BACKUP_DIR" && sha256sum database.dump media.tar.gz uploads.tar.gz environment.env > SHA256SUMS)

trap - EXIT INT TERM
restart_uploader
echo "Backup created: $BACKUP_DIR"
