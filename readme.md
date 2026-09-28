# NAS Media Uploader

Go API for authenticated, resumable media uploads. Authentication uses an
administrator password followed by TOTP. Uploads are transferred in 8 MiB
parts, assembled, checksum-verified, and recorded in PostgreSQL.

## Requirements

- Docker with Docker Compose
- A TOTP authenticator application
- `curl`, `jq`, `file`, `shasum`, and `split` for the API examples

## Local setup

Create the environment file and replace the example passwords:

```bash
cp .env.example .env
```

The initial administrator is created from `ENV_ADMIN_USERNAME` and
`ENV_ADMIN_PASSWORD` only when the users table is empty. The password must be
at least 12 characters. Changing these values later does not update an existing
user.

Start the API and PostgreSQL:

```bash
docker compose --env-file .env -f build/compose.yaml up --build
```

The example configuration exposes the API at `http://localhost:8081`. Change
`ENV_PORT` in `.env` if that port is occupied.

Stop the services while retaining uploaded media and database data:

```bash
docker compose --env-file .env -f build/compose.yaml down
```

Add `--volumes` only when you intentionally want to delete all local data.

## Authentication

Only `POST /auth/login` and `POST /auth/verify` are public. All other endpoints
require the `pal_medias_uploader_session` cookie.

Set values used by the examples:

```bash
BASE_URL=http://localhost:8081
COOKIE_JAR=/tmp/pal-uploader-cookies.txt
USERNAME=admin
PASSWORD='your-admin-password'
```

Begin login:

```bash
LOGIN_RESPONSE=$(curl -fsS \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg username "$USERNAME" \
    --arg password "$PASSWORD" \
    '{username:$username,password:$password}')" \
  "$BASE_URL/auth/login")

echo "$LOGIN_RESPONSE" | jq
CHALLENGE_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.challengeToken')
```

On the first login, the response also contains `provisioningUri` and `secret`.
Add either value to your authenticator application. Later logins return only
`challengeToken`.

Verify a current six-digit TOTP code and save the session cookie:

```bash
printf 'TOTP code: '
read -r TOTP_CODE

curl -fsS -c "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg challengeToken "$CHALLENGE_TOKEN" \
    --arg code "$TOTP_CODE" \
    '{challengeToken:$challengeToken,code:$code}')" \
  "$BASE_URL/auth/verify" | jq
```

Confirm the session:

```bash
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/auth/session" | jq
```

## Upload media

Choose a real media file and calculate its metadata:

```bash
MEDIA_FILE="$PWD/test/test-video.mp4"
FILE_NAME=$(basename "$MEDIA_FILE")
FILE_SIZE=$(wc -c < "$MEDIA_FILE" | tr -d ' ')
MIME_TYPE=$(file -b --mime-type "$MEDIA_FILE")
SHA256=$(shasum -a 256 "$MEDIA_FILE" | awk '{print $1}')
```

Create an upload session:

```bash
CREATE_RESPONSE=$(curl -fsS -b "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n \
    --arg filename "$FILE_NAME" \
    --arg mime_type "$MIME_TYPE" \
    --arg sha256 "$SHA256" \
    --argjson size "$FILE_SIZE" \
    '{filename:$filename,mime_type:$mime_type,size:$size,sha256:$sha256}')" \
  "$BASE_URL/media/upload")

echo "$CREATE_RESPONSE" | jq
UPLOAD_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id')
CHUNK_SIZE=$(echo "$CREATE_RESPONSE" | jq -r '.chunk_size')
```

Split the file and upload every part. Part numbers are zero-based, and the
chunk endpoint requires `PUT` with raw bytes as its body.

```bash
CHUNK_DIR=$(mktemp -d)
split -b "$CHUNK_SIZE" "$MEDIA_FILE" "$CHUNK_DIR/part-"

PART_NUMBER=0
for CHUNK in "$CHUNK_DIR"/part-*; do
  echo "Uploading part $PART_NUMBER"
  curl -fsS -X PUT -b "$COOKIE_JAR" \
    -H 'Content-Type: application/octet-stream' \
    --data-binary "@$CHUNK" \
    "$BASE_URL/media/upload/$UPLOAD_ID/parts/$PART_NUMBER" | jq
  PART_NUMBER=$((PART_NUMBER + 1))
done
```

Inspect progress before completion:

```bash
curl -fsS -b "$COOKIE_JAR" \
  "$BASE_URL/media/upload/$UPLOAD_ID" | jq
```

Assemble, verify, and persist the media:

```bash
curl -fsS -X POST -b "$COOKIE_JAR" \
  "$BASE_URL/media/upload/$UPLOAD_ID/complete" | jq

rm -rf "$CHUNK_DIR"
```

The completion response contains the relative `media_path`, final `size`, and
server-computed `sha256`. Metadata is stored in `media_uploads`, and the file is
stored in the Compose `media_data` volume.

Cancel an incomplete upload and remove its temporary parts:

```bash
curl -fsS -X DELETE -b "$COOKIE_JAR" \
  "$BASE_URL/media/upload/$UPLOAD_ID" | jq
```

## API endpoints

<!-- markdownlint-disable MD013 -->

| Method   | Path                              | Authentication | Purpose                                  |
| -------- | --------------------------------- | -------------- | ---------------------------------------- |
| `POST`   | `/auth/login`                     | Public         | Create a password and TOTP challenge     |
| `POST`   | `/auth/verify`                    | Public         | Verify TOTP and issue a session cookie   |
| `GET`    | `/auth/session`                   | Cookie         | Return the authenticated username        |
| `POST`   | `/auth/logout`                    | Cookie         | Delete the session and expire the cookie |
| `POST`   | `/media/upload`                   | Cookie         | Create a resumable upload                |
| `GET`    | `/media/upload/{id}`              | Cookie         | List uploaded parts                      |
| `PUT`    | `/media/upload/{id}/parts/{part}` | Cookie         | Upload or replace one part               |
| `POST`   | `/media/upload/{id}/complete`     | Cookie         | Assemble and persist the media           |
| `DELETE` | `/media/upload/{id}`              | Cookie         | Cancel an incomplete upload              |

<!-- markdownlint-enable MD013 -->

The complete OpenAPI 3.1 contract is in
[spec/openapi.yaml](spec/openapi.yaml).

## Development checks

Run Go checks:

```bash
gofmt -w app cmd
go vet ./...
go test ./...
```

Validate the API specification:

```bash
npx --yes @redocly/cli lint --config spec/redocly.yaml spec/openapi.yaml
```
