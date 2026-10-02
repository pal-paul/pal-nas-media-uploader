# Useful features

The NAS media server includes the following optional workflows. All management
endpoints require the existing session cookie. User endpoints require the
`user` role; `/admin` endpoints require `admin`.

## Uploads and processing

- `POST /media/upload/check` checks an owner-scoped SHA-256 digest before
  uploading.
- `POST /media/upload-batches` creates a durable batch. Include its returned
  `id` as `batchId` in the existing `POST /media/upload` request. The embedded
  workspace performs duplicate checks first, so batch file and byte totals
  include only files that will be uploaded.
- `GET /media/upload-batches/{id}` reports completed files and bytes. `POST
/media/upload-batches/{id}/cancel` prevents new or late completions.
- Completed uploads enqueue a PostgreSQL-backed FFmpeg job. `GET
/media/files/{id}/processing-status` reports progress. Failed jobs are retried,
  and interrupted jobs are reclaimed after 15 minutes in `processing`.
- Generated thumbnails are available at `GET /media/files/{id}/thumbnail`.
  Gallery records include dimensions, duration, capture time, and GPS
  coordinates when found. Permanent deletion removes both the original media
  and its generated thumbnail.
- Gallery filters accept `capturedAfter`, `capturedBefore` (RFC3339),
  `latitude`, `longitude`, and `radiusKm`.

## Accounts and storage

- `POST /auth/recovery-codes` replaces and returns ten single-use TOTP recovery
  codes. Store them immediately; only digests are retained.
- `POST /auth/recovery` accepts the normal `challengeToken` plus a recovery
  `code` after password verification.
- `PATCH /admin/users/{id}/password` resets a password and revokes that user's
  sessions.
- `GET /admin/storage/dashboard` reports per-user usage. `PATCH
/admin/users/{id}/quota` sets `storageQuotaBytes` or `null` for the
  environment default.
- `PATCH /admin/users/{id}/trash-retention` sets `days` from 1 through 3650.
  Cleanup runs every six hours and can be triggered with `POST
/admin/trash/cleanup`.

## Sharing and transfer

- `POST /media/public-links` creates an expiring media or album link with an
  optional password. Send `mediaId` or `albumId`, `expiresAt`, and optional
  `password`.
- Public consumers use `GET /public/{token}` and `GET
/public/{token}/download`. Passwords are supplied in `X-Share-Password`;
  three failed attempts lock that link/client pair for five minutes.
- Owners revoke links with `DELETE /media/public-links/{id}`.
- `GET /media/export` downloads versioned JSON metadata. `POST /media/import`
  restores manual albums and only associates media currently owned by the
  importer.

## Embedded pages

- `/media/app` is the mobile-friendly media workspace with multi-file resumable
  upload, duplicate preflight, batch progress, thumbnails, import/export,
  recovery codes, and public-link creation.
- `/admin/config` includes storage usage, quotas, password resets, and trash
  retention controls.

The processing worker and trash scheduler run inside the single application
instance. PostgreSQL keeps queue and progress state across container restarts;
Redis is not required.
