# Prowlarr tools

Indexer management, application sync and release grabbing. Prowlarr serves
`/api/v1`, not the `/api/v3` Sonarr and Radarr use.

| Area | Tools | Access |
|---|---|---|
| Indexers | `prowlarr_list_indexers`, `prowlarr_get_indexer`, `prowlarr_list_indexer_schemas`, `prowlarr_get_indexer_schema`, `prowlarr_indexer_stats` | read |
| Search | `prowlarr_search` | read |
| Config | `prowlarr_list_applications`, `prowlarr_list_app_profiles`, `prowlarr_list_download_clients` | read |
| Tags | `prowlarr_list_tags` | read |
| Operations | `prowlarr_health`, `prowlarr_history`, `prowlarr_system_status` | read |
| Indexers | `prowlarr_add_indexer`, `prowlarr_update_indexer`, `prowlarr_test_indexer`, `prowlarr_test_all_indexers` | write |
| Operations | `prowlarr_sync_applications`, `prowlarr_grab_release`, `prowlarr_run_command` | write |
| Tags | `prowlarr_create_tag` | write |
| Deletion | `prowlarr_delete_indexer`, `prowlarr_delete_tag` | destructive |

Adding an indexer is a three-step flow: find the definition with
`prowlarr_list_indexer_schemas`, read the settings it accepts with
`prowlarr_get_indexer_schema`, then pass those field names to
`prowlarr_add_indexer`. `prowlarr_search` results carry a `guid` and an
`indexerId`, which is what `prowlarr_grab_release` needs.
