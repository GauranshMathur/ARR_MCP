# NZBGet tools

Usenet download client, spoken to over its JSON-RPC API with basic auth
(username and password, not an API key).

| Tool | Access |
|---|---|
| `nzbget_status` — speed, limit, disk space, pause states | read |
| `nzbget_list_queue` — source of the NZBIDs every editing tool takes | read |
| `nzbget_history` — finished, failed and deleted downloads | read |
| `nzbget_add_nzb` — by URL or base64 nzb content | write |
| `nzbget_pause_download`, `nzbget_resume_download` — whole queues: download, post or scan | write |
| `nzbget_pause_items`, `nzbget_resume_items` | write |
| `nzbget_move_items`, `nzbget_set_priority` | write |
| `nzbget_set_category`, `nzbget_rename_item` | write |
| `nzbget_retry_history_items` — return to the queue, optionally redownloading | write |
| `nzbget_mark_history_items` — good or bad for duplicate handling | write |
| `nzbget_set_rate_limit`, `nzbget_scan` | write |
| `nzbget_delete_items` — to history by default; `final` discards permanently | destructive |
| `nzbget_delete_history_items` — hides by default; `final` removes permanently | destructive |
