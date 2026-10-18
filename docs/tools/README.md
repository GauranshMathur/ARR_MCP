# Tools

| Service | Tools |
|---|---|
| [Sonarr](sonarr.md) | 77 |
| [Radarr](radarr.md) | 73 |
| [Bazarr](bazarr.md) | 33 |
| [Prowlarr](prowlarr.md) | 23 |
| [qBittorrent](qbittorrent.md) | 22 |
| [NZBGet](nzbget.md) | 18 |
| [Maintainerr](maintainerr.md) | 25 |
| [Seerr](seerr.md) | 17 |
| [Jellyfin](jellyfin.md) | 23 |
| [Jellystat](jellystat.md) | 10 |
| [Synology](synology.md) | 14 |

Sonarr and Radarr are kept at parity: 62 of their tool names are common to both,
and the rest differ only where the APIs genuinely do (seasons and episodes
versus movies and collections).

The interactive search and manual import tools mirror what the two web UIs call
"interactive search" and "manual import": `*_list_releases` runs a real query
against every configured indexer and reports why each release would be accepted
or rejected, `*_grab_release` takes one anyway, and the `*_manual_import` pair
previews and then imports files the automatic importer could not place.

Quality profiles and custom formats are addressed by name rather than by id:
`*_create_quality_profile` starts from the instance's own template and takes
quality names as `*_list_quality_definitions` reports them, and
`*_update_quality_profile` scores custom formats by the names
`*_list_custom_formats` returns. The configuration editors read the record
before writing it, so a setting they do not model is left as it was rather than
reset.

## What responses contain

Upstream payloads are far too large to return as they arrive — a single Sonarr
custom format list is 283 KB, one series' episode files 252 KB, and Radarr's
collection list 259 KB. Every tool returns a projection: identities, counts and
the fields that answer a question, never overviews, artwork, alternate titles or
embedded media info.

Indexer, download client, import list and notification listings never return the
provider `fields` array. That array holds each provider's own credentials —
indexer API keys, download client passwords, notification webhook URLs — and
none of it belongs in a model's context.

Prowlarr's indexer tools do return that array, because you cannot configure an
indexer without knowing its field names — but every field whose upstream
`privacy` is anything other than `normal` reports `***` in place of its value,
which covers Prowlarr's `apiKey`, `password` and `userName` fields.

Sonarr's and Radarr's provider tools (`*_get_provider`, `*_add_provider`,
`*_update_provider`) redact that same array by the same rule, so an indexer's
API key or a download client's password is reported as `***` while its field
name still is not. Editing a provider sends the stored values back untouched:
the mask is never written over a credential the caller did not change.

Services with no configured instances register no tools at all, so the advertised list always reflects what is actually reachable.
