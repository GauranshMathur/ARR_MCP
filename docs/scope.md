# Scope

ARR-MCP covers the \*arr-named applications that share the common \*arr API contract: the
same versioned `/api` shape, the same API-key header, the same `/system/status` and health
endpoints. That shared contract is the whole reason the project is cheap to extend — a
service is described by a `ServiceSpec` rather than a bespoke client, so it inherits the
transport, the instance registry and the permission model for free. The two download
clients are the deliberate exceptions: each needed exactly one extra auth scheme on that
shared transport, not a client of its own. Anything that would need more than that is
where the line is drawn. Synology DSM is the one approved exception beyond that line: it needed a call shape of its own
(`synoCall`) as well as an auth scheme, see [Synology](#synology) below.

## Planned

Cleanuparr and Notifiarr.

## Maintainerr

Maintainerr is not \*arr-named, but it drives Sonarr and Radarr and serves a plain JSON
`/api` with no authentication, so it needed no transport change: `AuthNone` already
existed. Rule groups are created and edited through Maintainerr's YAML encode and decode
endpoints, so the model never handles its numeric rule encoding, and every edit reads
the whole group back and writes it whole, because Maintainerr's update resets fields it
is not sent. Left out on purpose: running collection handling (`/collections/handle`,
which deletes due media immediately), collection-only groups without rules, and every
write under `/api/settings`, which returns every stored credential in plaintext.

## Seerr

Seerr (the merged Jellyseerr and Overseerr) serves `/api/v1` with the same `X-Api-Key`
header the \*arr apps use, so it needed no transport change. Its `/status` answers
without a key, so the health check reads `/settings/about` instead, which means
`--check` fails on a wrong key rather than passing. Left out on purpose: user
management, every `/settings` write, editing a request (`PUT /request/{id}`), and
`DELETE /media/{id}` and `DELETE /media/{id}/file`, the second of which deletes files
from disk; Maintainerr owns deletion.

Seerr is multi-user, unlike the \*arr apps. Issue comments and display names are written
by Seerr's own users and reach the model verbatim, so a user could write a comment that
reads like an instruction. The tool schemas mark that text as data, but the default
`confirm` permission mode is what actually stands between such text and
`seerr_approve_request` or `seerr_delete_request`. Do not run a Seerr instance in `full`
mode if untrusted users can file issues.

## Jellyfin

Jellyfin is not \*arr-named and its API is not the \*arr contract, but it needed one
addition to the shared transport rather than a client of its own: the key goes in the
`Authorization` header wrapped as `MediaBrowser Token="<key>"`, which is
`ServiceSpec.AuthHeaderFormat`. It is still `AuthHeaderKey`. Two quirks of 12.0: the
connectivity check is `/System/Info`, because `/System/Info/Public` answers without a key,
and `GET /Items/{id}` rejects a request with no user, so `jellyfin_get_item` reads as the
first enabled user unless told otherwise. Left out on purpose: user and permission
management, playback control, item deletion (Maintainerr owns that) and server settings.

## Jellystat

Jellystat authenticates with the raw key in `X-Api-Token`, which is `AuthHeaderKey` with
`AuthHeader` set, so no transport change was needed. Its routes are `/api/...` and
`/stats/...`, there is no base path, and the connectivity check is
`/stats/getLibraryOverview` because `/auth/isConfigured` answers without a key and
`/api/getconfig` returns configuration. Left out on purpose: syncs, backup and restore,
logs, every settings and API-key endpoint, purges and playback deletion.

## Synology

A Synology NAS is not \*arr-named and its API is not the \*arr contract, so it got its own
call shape beside NZBGet's `nzbCall`: `synoCall` in `pkg/arr/synology.go`. Every DSM API is
served from `/webapi/entry.cgi` and chosen by `api`, `version` and `method` form fields; the
call is a `POST` with a form body so nothing sensitive is in a URL. Login is
`SYNO.API.Auth` v7 with `format=sid`, and the session id is cached per instance like
qBittorrent's cookie and sent as the `_sid` field. DSM answers HTTP 200 for failures, with
`{"success":false,"error":{"code":N}}`, so the body is inspected on every call; a session
error (105, 106, 107, 119) logs in once more and retries once. It needed one new auth scheme,
`AuthDSM`, because the id travels in the body rather than a cookie or header.

`synoCall` is unexported and refuses any api, method and version that is not in one of two
fixed sets: `synoReadOps` (reads) and `synoWriteOps` (File Station `CreateFolder`, `Rename`,
`CopyMove`, and the `start`, `status` and `stop` methods of `Delete`, not its blocking `delete`).
The code cannot reach `SYNO.Core.User`, `SYNO.Core.Group`,
`SYNO.Core.ISCSI.*`, share permissions or any other call that
changes state. The write tools also pass every path through the `allowedPaths` check in
`pkg/arr/synology_write.go` before logging in.

## Not planned: Plex

Plex is not \*arr-named and does not speak the \*arr API contract: it uses its own token
scheme and an XML-flavoured API. Supporting it means a bespoke client with its own auth
handling, response shapes and tests, for a capability that overlaps heavily with what the
\*arr tools and Jellyfin already expose.

## Download clients (qBittorrent, NZBGet)

The \*arr queue tools already show what is downloading, and
`sonarr_delete_queue_item` can drop a stuck download with `removeFromClient`.
What they cannot do is anything the \*arr app did not initiate: add a torrent
or NZB by hand, pause or reprioritise the client's queue, change categories
and speed limits, or clean up the client's history. Those are the operations
the download-client tools cover, so you can do from an MCP client what you
would otherwise do in the client's own web UI.

Neither client speaks the \*arr contract, so each got exactly one addition to
the shared transport rather than a bespoke client: qBittorrent logs in with a
username and password and replays the session cookie (`AuthSession`), and
NZBGet's JSON-RPC rides on HTTP basic auth (`AuthBasic`). Everything else —
the instance registry, permission tiers, `--check`, credential redaction — is
inherited unchanged.

## Not planned: SABnzbd

SABnzbd's query-parameter `mode=` API would need a third transport shape for
an application NZBGet already covers in this stack. Open an issue if you run
SABnzbd and want it.
