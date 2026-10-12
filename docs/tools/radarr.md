# Radarr tools

| Area | Tools | Access |
|---|---|---|
| Library | `radarr_list_movies`, `radarr_get_movie`, `radarr_search_movies`, `radarr_list_collections`, `radarr_calendar` | read |
| Wanted | `radarr_wanted_missing`, `radarr_wanted_cutoff` | read |
| Search & import | `radarr_list_releases`, `radarr_manual_import_preview` | read |
| Files | `radarr_list_movie_files`, `radarr_rename_preview` | read |
| Profiles | `radarr_list_quality_profiles`, `radarr_get_quality_profile`, `radarr_list_quality_definitions`, `radarr_list_custom_formats`, `radarr_get_custom_format`, `radarr_list_delay_profiles`, `radarr_list_release_profiles` | read |
| Config | `radarr_list_root_folders`, `radarr_naming_config`, `radarr_media_management_config`, `radarr_list_indexers`, `radarr_list_download_clients`, `radarr_list_import_lists`, `radarr_list_notifications` | read |
| Providers | `radarr_provider_schemas`, `radarr_get_provider` | read |
| Tags | `radarr_list_tags`, `radarr_tag_details` | read |
| Operations | `radarr_queue`, `radarr_queue_status`, `radarr_history`, `radarr_blocklist`, `radarr_health`, `radarr_disk_space`, `radarr_system_status`, `radarr_list_tasks`, `radarr_list_updates` | read |
| Add & edit | `radarr_add_movie`, `radarr_edit_movies`, `radarr_update_collection`, `radarr_create_tag`, `radarr_update_tag` | write |
| Search & import | `radarr_grab_release`, `radarr_grab_queue_item`, `radarr_manual_import`, `radarr_mark_history_failed` | write |
| Files | `radarr_rename_files`, `radarr_update_files` | write |
| Providers | `radarr_add_provider`, `radarr_update_provider`, `radarr_test_provider` | write |
| Automation | `radarr_trigger_search`, `radarr_refresh_movies`, `radarr_run_command` | write |
| Profiles | `radarr_create_quality_profile`, `radarr_update_quality_profile`, `radarr_create_custom_format`, `radarr_update_custom_format`, `radarr_update_delay_profile` | write |
| Config | `radarr_add_root_folder`, `radarr_update_naming_config`, `radarr_update_media_management_config` | write |
| Deletion | `radarr_delete_movie`, `radarr_delete_movie_files`, `radarr_delete_queue_item`, `radarr_delete_queue_items`, `radarr_delete_blocklist_item`, `radarr_delete_tag`, `radarr_delete_quality_profile`, `radarr_delete_custom_format`, `radarr_delete_root_folder`, `radarr_delete_provider` | destructive |
