# Jellyfin tools

What Jellyfin itself knows, as opposed to what the \*arr apps acquired: libraries and scan
state, items as Jellyfin indexes them, who is connected and playing, users, scheduled tasks
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
| `jellyfin_get_item` — overview, genres, provider ids and one user's play state | read |
| `jellyfin_list_sessions` — who is connected and what is playing | read |
| `jellyfin_list_users` — no policy or configuration | read |
| `jellyfin_list_tasks` — state and last result | read |
| `jellyfin_activity_log` — newest first, paged | read |
| `jellyfin_scan_library` — every library, in the background | write |
| `jellyfin_refresh_item` — one item or library; never replaces existing metadata | write |
| `jellyfin_run_task` — start a scheduled task now; some tasks (clean cache, logs, activity log) delete data | destructive |
