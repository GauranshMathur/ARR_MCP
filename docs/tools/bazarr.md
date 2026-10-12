# Bazarr tools

Subtitle management, including two instances if you run one per Sonarr/Radarr pair.

| Tool | Access |
|---|---|
| `bazarr_badges` — outstanding counts, cheapest first call | read |
| `bazarr_wanted_episodes`, `bazarr_wanted_movies` | read |
| `bazarr_list_series`, `bazarr_list_movies` | read |
| `bazarr_list_episode_subtitles` — the only source of subtitle file paths | read |
| `bazarr_list_providers`, `bazarr_list_languages`, `bazarr_list_language_profiles` | read |
| `bazarr_health`, `bazarr_system_status`, `bazarr_list_tasks` | read |
| `bazarr_manual_search_episode`, `bazarr_manual_search_movie` — candidates with scores; downloads nothing | read |
| `bazarr_episode_history`, `bazarr_movie_history`, `bazarr_list_blacklist` | read |
| `bazarr_subtitle_info` — audio and embedded tracks, for a sync reference | read |
| `bazarr_search_episode_subtitles`, `bazarr_search_movie_subtitles` — automatic, best-match | write |
| `bazarr_download_episode_subtitle`, `bazarr_download_movie_subtitle` — one named search result | write |
| `bazarr_set_series_profile`, `bazarr_set_movie_profile` — assign a languages profile | write |
| `bazarr_series_action`, `bazarr_movie_action` — scan-disk, search-missing, search-wanted | write |
| `bazarr_modify_subtitle` — sync, translate, remove_HI and the other mods | write |
| `bazarr_reset_providers`, `bazarr_run_task` | write |
| `bazarr_delete_episode_subtitle`, `bazarr_delete_movie_subtitle` | destructive |
| `bazarr_blacklist_subtitle` — deletes the file, then searches for a replacement | destructive |
| `bazarr_delete_blacklist_item` | destructive |

The automatic search tools report success whether or not a provider had a
match, because that is all Bazarr tells them. `bazarr_manual_search_episode`
lists the candidates with their scores and lets
`bazarr_download_episode_subtitle` take a named one, which is the only way to
know what was downloaded.

Blacklisting is destructive rather than a write: Bazarr deletes the subtitle
file from disk before starting the replacement search.

Subtitle *upload* is deliberately out of scope. Bazarr's upload endpoint takes
a multipart file body, and a model has no file bytes to send — only paths it
read from a tool result.
