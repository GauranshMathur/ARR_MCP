# Sonarr tools

| Area | Tools | Access |
|---|---|---|
| Library | `sonarr_list_series`, `sonarr_get_series`, `sonarr_search_series`, `sonarr_list_episodes`, `sonarr_calendar` | read |
| Wanted | `sonarr_wanted_missing`, `sonarr_wanted_cutoff` | read |
| Search & import | `sonarr_list_releases`, `sonarr_manual_import_preview` | read |
| Files | `sonarr_list_episode_files`, `sonarr_rename_preview` | read |
| Profiles | `sonarr_list_quality_profiles`, `sonarr_get_quality_profile`, `sonarr_list_quality_definitions`, `sonarr_list_custom_formats`, `sonarr_get_custom_format`, `sonarr_list_delay_profiles`, `sonarr_list_release_profiles` | read |
| Config | `sonarr_list_root_folders`, `sonarr_naming_config`, `sonarr_media_management_config`, `sonarr_list_indexers`, `sonarr_list_download_clients`, `sonarr_list_import_lists`, `sonarr_list_notifications` | read |
| Providers | `sonarr_provider_schemas`, `sonarr_get_provider` | read |
| Tags | `sonarr_list_tags`, `sonarr_tag_details` | read |
| Operations | `sonarr_queue`, `sonarr_queue_status`, `sonarr_history`, `sonarr_blocklist`, `sonarr_health`, `sonarr_disk_space`, `sonarr_system_status`, `sonarr_list_tasks`, `sonarr_list_updates` | read |
| Add & edit | `sonarr_add_series`, `sonarr_edit_series`, `sonarr_set_season_monitored`, `sonarr_monitor_episodes`, `sonarr_create_tag`, `sonarr_update_tag` | write |
| Search & import | `sonarr_grab_release`, `sonarr_grab_queue_item`, `sonarr_manual_import`, `sonarr_mark_history_failed` | write |
| Files | `sonarr_rename_files`, `sonarr_update_files` | write |
| Providers | `sonarr_add_provider`, `sonarr_update_provider`, `sonarr_test_provider` | write |
| Automation | `sonarr_trigger_search`, `sonarr_refresh_series`, `sonarr_run_command` | write |
| Profiles | `sonarr_create_quality_profile`, `sonarr_update_quality_profile`, `sonarr_create_custom_format`, `sonarr_update_custom_format`, `sonarr_create_release_profile`, `sonarr_update_release_profile`, `sonarr_update_delay_profile` | write |
| Config | `sonarr_add_root_folder`, `sonarr_update_naming_config`, `sonarr_update_media_management_config` | write |
| Deletion | `sonarr_delete_series`, `sonarr_delete_episode_files`, `sonarr_delete_queue_item`, `sonarr_delete_queue_items`, `sonarr_delete_blocklist_item`, `sonarr_delete_tag`, `sonarr_delete_quality_profile`, `sonarr_delete_custom_format`, `sonarr_delete_release_profile`, `sonarr_delete_root_folder`, `sonarr_delete_provider` | destructive |
