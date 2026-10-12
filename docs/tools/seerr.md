# Seerr tools

Media requests: who asked for what, what is waiting for approval, and what failed to
reach Sonarr or Radarr. A request carries only TMDB ids, so `seerr_get_media` turns one
into a title. Requests made through these tools are filed as the administrator unless
`userId` names someone else. See [Scope](../scope.md#seerr) for what is left out.

| Tool | Access |
|---|---|
| `seerr_system_status` — version, update available, totals | read |
| `seerr_request_counts` — pending, approved, declined, processing, available, completed | read |
| `seerr_list_requests` — filter by state or user; `pending` awaits approval, `failed` needs a retry | read |
| `seerr_get_request` | read |
| `seerr_search`, `seerr_discover` — trending, popular movies or popular shows | read |
| `seerr_get_media` — a movie or show by TMDB id, with its availability and requests | read |
| `seerr_list_users` — request counts per user; emails are never returned | read |
| `seerr_list_issues`, `seerr_get_issue` — problems users reported, with comments | read |
| `seerr_create_request` — a movie, or a show's seasons; an administrator's request is approved at once | write |
| `seerr_approve_request`, `seerr_decline_request`, `seerr_retry_request` | write |
| `seerr_comment_issue`, `seerr_set_issue_status` — resolve or reopen | write |
| `seerr_delete_request` | destructive |
