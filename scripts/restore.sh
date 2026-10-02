#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "Usage: FORCE=YES $0 BACKUP_DIRECTORY" >&2
	exit 1
fi
if [ "${FORCE:-}" != "YES" ]; then
	echo "Restore replaces the current database and media. Re-run with FORCE=YES." >&2
	exit 1
fi

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE=${ENV_FILE:-"$ROOT_DIR/.env"}
COMPOSE_FILE=${COMPOSE_FILE:-"$ROOT_DIR/build/compose.yaml"}
COMPOSE_EXTRA=${COMPOSE_EXTRA:-}
BACKUP_DIR=$(CDPATH= cd -- "$1" && pwd)

set -a
. "$ENV_FILE"
set +a
(cd "$BACKUP_DIR" && sha256sum -c SHA256SUMS)

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
compose exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists < "$BACKUP_DIR/database.dump"
compose run --rm --no-deps --entrypoint sh \
	-v "$BACKUP_DIR:/backup:ro" uploader \
	-c 'rm -rf "$ENV_MEDIA_DIR"/* "$ENV_MEDIA_DIR"/.[!.]* "$ENV_MEDIA_DIR"/..?*; tar -xzf /backup/media.tar.gz -C "$ENV_MEDIA_DIR"; rm -rf "$ENV_TMP_DIR"/* "$ENV_TMP_DIR"/.[!.]* "$ENV_TMP_DIR"/..?*; tar -xzf /backup/uploads.tar.gz -C "$ENV_TMP_DIR"'

trap - EXIT INT TERM
restart_uploader
echo "Restore completed from: $BACKUP_DIR"
