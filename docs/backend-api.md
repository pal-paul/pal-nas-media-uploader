# Backend API reference

This backend supports multiple users, resumable media uploads, personal albums,
and owner-controlled sharing. Public URL names are an API contract; they do not
need to match Go package names.

## Domain ownership

<!-- markdownlint-disable MD013 -->

| Domain | Responsibility |
| --- | --- |
| `setup` | Bootstrap-authorized creation of the first persistent administrator. |
| `auth` | Login, TOTP verification, sessions, and logout. |
| `user` | Accounts, roles, upload folders, downloads, and owner-managed shares. |
| `uploader` | Resumable upload sessions, chunks, completion, and cancellation. |
| `autoalbum` | Nightly weekly and high-volume daily album reconciliation. |
| `gallery` | Albums, accessible media, personal favorites, trash, and storage totals. |
| `db` | PostgreSQL implementations of domain repository contracts. |

The domains share an authenticated user identity but do not call each other's
HTTP handlers. For example, `gallery` uses completed uploads as media records;
it does not implement file transfer again.

## Multi-user rules

- Every completed media item has one owner, assigned from the authenticated
  upload session. A client cannot choose `ownerId`.
- An owner may use an active item to share their library with another regular
  user. The recipient can access all current and future active media from that
  owner. Sharing grants list, album membership, favorite, download, storage,
  and automatic-album access; it does not transfer ownership.
- Albums belong to one user. A user may add owned media or media available
  through a library share. If a library share is removed, all media from that
  owner is no longer returned in the recipient's albums.
- Favorite state belongs to `(user, media)`, so users do not overwrite each
  other's favorites.
- Only the owner may trash, restore, permanently delete, share, or unshare an
  item. Trashed media is immediately hidden from recipients.
- Permanent deletion removes the physical file and cascades its album links and
  personal preferences. Library-share relationships remain active for the
  owner's other current and future media.
- Resource lookups are scoped by the authenticated user. An inaccessible ID is
  reported as not found rather than exposing another user's data.

## Base URL and authentication

The default local URL is `http://localhost:8081`. The one-time `/setup` routes
and the authentication routes are public. All other routes require the
`pal_medias_uploader_session` cookie.

Browser requests must include credentials:

```js
await fetch(`${baseUrl}/albums`, { credentials: 'include' })
```

Administrators manage accounts but cannot upload or use gallery routes. Regular
users can upload and use the gallery. Accounts created by an administrator use
password plus TOTP. Environment bootstrap credentials authorize only the
one-time setup form and are never stored as a login account. The first
persistent administrator uses its configured password without TOTP.

`ENV_FRONTEND_PAGES_ENABLED=YES` serves `/setup` and `/admin/config`; `NO`
returns `404` for those HTML pages without disabling the JSON APIs.

## Endpoint summary

### Authentication and users

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/setup` | Public, before setup | Open initial administrator setup. |
| `POST` | `/setup` | Public, before setup | Create the first administrator. |
| `POST` | `/auth/login` | Public | Start password/TOTP login. |
| `POST` | `/auth/verify` | Public | Verify TOTP and issue a session cookie. |
| `GET` | `/auth/session` | Authenticated | Return the current identity and role. |
| `POST` | `/auth/logout` | Authenticated | Delete the current session. |
| `GET` | `/admin/config` | Admin | Open the account configuration page. |
| `GET` | `/admin/users` | Admin | List accounts. |
| `POST` | `/admin/users` | Admin | Create an account and one-time TOTP setup. |
| `PUT` | `/users/me/folder` | Regular user | Change the user's upload folder. |

### Uploads

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/media/upload` | Create a resumable upload. |
| `GET` | `/media/upload/{id}` | List uploaded part numbers. |
| `PUT` | `/media/upload/{id}/parts/{part}` | Upload or replace one zero-based part. |
| `POST` | `/media/upload/{id}/complete` | Verify and assemble completed media. |
| `DELETE` | `/media/upload/{id}` | Cancel an incomplete upload. |

Upload sessions belong to the user who created them. Parts use a fixed 8 MiB
size except for the final part. See [the OpenAPI contract](../spec/openapi.yaml)
for complete request and response schemas.

### Media and sharing

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/media` | Regular user | List active owned and shared media. |
| `GET` | `/media?trash=true` | Regular user | List only the current user's trash. |
| `GET` | `/media/files/{id}/download` | Owner or recipient | Download active media. |
| `POST` | `/media/files/{id}/shares` | Owner | Share all current and future media with `{ "username": "bob" }`. |
| `DELETE` | `/media/files/{id}/shares?username=bob` | Owner | Remove Bob's library-wide access. |
| `PATCH` | `/media/files/{id}/favorite` | Owner or recipient | Set personal favorite state. |
| `DELETE` | `/media/files/{id}` | Owner | Move media to trash. |
| `PATCH` | `/media/files/{id}/restore` | Owner | Restore media from trash. |
| `DELETE` | `/media/files/{id}/permanent` | Owner | Delete the file and database record. |
| `GET` | `/storage` | Regular user | Return totals for accessible active media. |

`GET /media` supports `search`, `kind=photo|video`,
`sort=newest|oldest|title`, and `trash=true`. A returned item's `shared` field
is `true` when the current user is a recipient rather than its owner.

### Albums

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/albums` | List the current user's albums. |
| `POST` | `/albums` | Create an album. |
| `PATCH` | `/albums/order` | Set album display order. |
| `GET` | `/albums/{albumId}` | Get an album and accessible active media. |
| `PATCH` | `/albums/{albumId}` | Update title and description. |
| `DELETE` | `/albums/{albumId}` | Delete the album, not its media. |
| `POST` | `/albums/{albumId}/media` | Add owned or shared media. |
| `DELETE` | `/albums/{albumId}/media/{id}` | Remove media from the album. |
| `PATCH` | `/albums/{albumId}/cover` | Set an accessible album member as cover. |

<!-- markdownlint-enable MD013 -->

Automatic albums are reconciled once per night. The default is `02:00` in the
server's local timezone; configure `ENV_AUTO_ALBUM_RUN_AT` with a `HH:MM` value
and `ENV_AUTO_ALBUM_TIMEZONE` with an IANA timezone such as
`Europe/Stockholm`. A restart does not run the job immediately; the next
configured nightly run processes media that still has no automatic album.

Grouping uses each owner's completed media and UTC calendar dates. Weeks run
Monday through Sunday. A normal week is titled like `October 5 - 11`. When one
date has more than 15 uploads, reconciliation creates a daily album such as
`October 5` and moves all uploads from that date out of the weekly album. Later
unassigned uploads on that date go directly to the daily album.

Responses mark these albums with `"automatic": true`. Their title, order,
membership, and lifecycle are system-managed, so the manual update, delete,
reorder, add, and remove operations return `404` for them. Users may still
select an album cover.

Create or update an album:

```json
{
  "title": "Summer holiday",
  "description": "Coast and mountains"
}
```

Add media:

```json
{
  "mediaIds": ["4c159d2d679c4469b6c51e061f723034"]
}
```

## Deliberately separate concerns

The earlier endpoint inventory included filesystem scanning and generated video
thumbnails. They are not part of the synchronous gallery domain implemented
here. Automatic albums draw from completed media records whose owner was set by
an authenticated upload, so their owner is unambiguous. A filesystem scanner
would still need an explicit owner, and thumbnail generation should run as a
media-processing concern with its own cache and job lifecycle.

## Errors

JSON errors use an `error` field:

```json
{"error":"not found"}
```

Common statuses are `400` for invalid input, `401` for a missing or invalid
session, `403` for the wrong role, `404` for missing or inaccessible resources,
`409` for missing upload parts, `500` for internal failures, and `502` when a
physical file cannot be deleted.
