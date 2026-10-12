# qBittorrent tools

Every mutating tool takes `hashes` from `qbittorrent_list_torrents`; the single
element `"all"` targets every torrent, which is how the WebUI's select-all buttons work.

| Tool | Access |
|---|---|
| `qbittorrent_list_torrents` | Read |
| `qbittorrent_torrent_files` | Read |
| `qbittorrent_transfer_info` | Read |
| `qbittorrent_list_categories` | Read |
| `qbittorrent_list_tags` | Read |
| `qbittorrent_system_status` | Read |
| `qbittorrent_add_torrent` | Write |
| `qbittorrent_start_torrents` | Write |
| `qbittorrent_stop_torrents` | Write |
| `qbittorrent_recheck_torrents` | Write |
| `qbittorrent_set_category` | Write |
| `qbittorrent_create_category` | Write |
| `qbittorrent_edit_category` | Write |
| `qbittorrent_add_tags` | Write |
| `qbittorrent_remove_tags` | Write |
| `qbittorrent_set_location` | Write |
| `qbittorrent_rename_torrent` | Write |
| `qbittorrent_set_priority` | Write |
| `qbittorrent_set_torrent_limits` | Write |
| `qbittorrent_set_global_limits` | Write |
| `qbittorrent_delete_torrents` | Destructive |
| `qbittorrent_delete_categories` | Destructive |
