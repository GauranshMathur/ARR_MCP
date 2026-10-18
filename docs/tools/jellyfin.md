# Jellyfin tools

What Jellyfin itself knows, as opposed to what the \*arr apps acquired: libraries and scan
state, items as Jellyfin indexes them with their chapters and skip markers (media segments),
who is connected and playing, users, scheduled tasks
and the activity log. Output is trimmed hard; a Jellyfin item has over a hundred fields.
Names set by users and devices are passed through verbatim and flagged in the output schema
as data, not instructions, and the default `confirm` permission mode is the guard against
acting on them. One name is withheld: Jellyfin records a failed login as `Failed login
attempt from <username>` with whatever the caller typed, and the caller needs no account, so
the activity log reports the attempt, its time and its address without the username. Lists
are capped at 100 entries per call. See [Scope](../scope.md#jellyfin) for what is left out.

| Tool | Access |
|---|---|
| `jellyfin_system_info` — version, pending restart or update | read |
| `jellyfin_list_libraries` — folders and scan state | read |
| `jellyfin_search_items` — by name, type or library; paged, 25 by default | read |
| `jellyfin_get_item` — overview, genres, provider ids, chapters and one user's play state | read |
| `jellyfin_list_media_segments` — one item's Intro, Outro, Recap, Preview and Commercial spans, in seconds | read |
| `jellyfin_list_episode_markers` — every episode under a series or season with its chapters and media segments; paged, 25 by default | read |
| `jellyfin_list_sessions` — who is connected and what is playing | read |
| `jellyfin_list_users` — no policy or configuration | read |
| `jellyfin_list_plugins` — every installed build with its version and load status (Active, Restart, Superseded, ...) | read |
| `jellyfin_get_plugin_config` — an allowlisted plugin's settings; other plugins are refused unread (see below) | read |
| `jellyfin_list_tasks` — state and last result | read |
| `jellyfin_activity_log` — newest first, paged | read |
| `jellyfin_scan_library` — every library, in the background | write |
| `jellyfin_refresh_item` — one item or library; never replaces existing metadata | write |
| `jellyfin_run_task` — start a scheduled task now; some tasks (clean cache, logs, activity log) delete data | destructive |

## Plugin settings

`jellyfin_get_plugin_config` reads only plugins and keys on a fixed allowlist, because plugin
settings hold credentials: on the server it was built against, Jellyfin Enhanced keeps Seerr,
Sonarr, Radarr and TMDB keys among 299 settings, LDAP its bind password, and Trakt, SSO and
Webhook theirs inside nested objects whose top-level names look harmless. Allowlisted: Intro
Skipper (all 76 settings), Jellyfin Enhanced (11 skip and playback settings), InPlayerEpisodePreview,
Jellyfin Tweaks and Subtitle Extract: their key names and value types were checked against the
live server, though not every value was seen, and some are text an administrator types, such as
Intro Skipper's exclusion lists and file paths. Any other plugin is refused before its settings
are read.
An allowlisted key is returned only while its value is a scalar or a list of scalars; keys left
out are named under `withheld`, and a key a later plugin version adds stays withheld until it is
listed in `pkg/arr/jellyfin_plugin_config.go`.

## Intro Skipper

These tools call the [Intro Skipper](https://github.com/intro-skipper/intro-skipper) plugin's own
API, which the Jellyfin server serves behind the same key; without the plugin they answer 404.
The plugin publishes no API document, so they follow its source at 12.0.4.0 and 12.0.5.0. It names
segment types Introduction, Credits, Preview, Recap and Commercial, where Jellyfin's media segments
say Intro and Outro. Its records are the source of Jellyfin's media segments: a write reports
`projection` Applied when Jellyfin's copy, which playback reads, already shows it, and Pending
when that copy catches up later.

| Tool | Access |
|---|---|
| `jellyfin_list_introskipper_segments` — the plugin's segments for an item, with ids and sources; `includeSuppressed` adds deleted detected ones | read |
| `jellyfin_get_introskipper_analyzers` — a season's analyzer per segment type | read |
| `jellyfin_get_introskipper_scan_status` — per season on 12.0.5.0, server-wide on 12.0.4.0 | read |
| `jellyfin_create_introskipper_segment` — add a hand-made segment, in seconds | write |
| `jellyfin_update_introskipper_segment` — move a segment; it becomes hand-made | write |
| `jellyfin_restore_introskipper_segment` — bring back a deleted detected segment | write |
| `jellyfin_set_introskipper_analyzers` — change a season's analyzers for the named types; refuses a season the plugin does not know. Intro Skipper then re-analyzes the season by itself, and its segments can read as missing until the scan status shows it idle | write |
| `jellyfin_delete_introskipper_segment` — a detected segment is kept as a restorable tombstone; a hand-made one is gone for good | destructive |
| `jellyfin_scan_introskipper_season` — **erases every segment in the season, hand-made ones included**, then re-analyzes it | destructive |
