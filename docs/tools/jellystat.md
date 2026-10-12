# Jellystat tools

What was watched, by whom and when, from [Jellystat](https://github.com/CyferShepard/Jellystat)'s
record of Jellyfin playback: the questions "what has nobody watched in a year" and "which
series are actually used" that nothing else here can answer. Every tool only reads. Several
of Jellystat's reads are `POST`s because it takes filters in a JSON body; that does not make
them writes. Durations are seconds, except `jellystat_views_over_time`, which is minutes.
User, device, client and library names and titles are passed through verbatim and flagged
as data, not instructions. Playback history rows are trimmed to who, what, when, how long,
client, device and play method. Pages hold 25 rows by default and at most 100.

| Tool | Access |
|---|---|
| `jellystat_library_overview` — items, seasons and episodes per library | read |
| `jellystat_list_libraries` — all-time plays and watch time per library, what was played last | read |
| `jellystat_playback_history` — all plays, or one user's, item's or library's; paged | read |
| `jellystat_most_viewed` — top five titles of a type by plays over a window | read |
| `jellystat_most_popular` — top five titles of a type by distinct viewers | read |
| `jellystat_most_active_users` — top five users by plays | read |
| `jellystat_user_activity` — every user's totals and last playback; paged | read |
| `jellystat_item_stats` — plays and watch time of one item over a window of hours | read |
| `jellystat_library_items` — a library's items with play count; sort by Views ascending for what nobody watched | read |
| `jellystat_views_over_time` — plays and minutes per library per day | read |
