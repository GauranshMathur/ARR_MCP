# Maintainerr tools

Rule-driven library cleanup. A rule group fills a collection; each item in it is acted
on (usually deleted from disk through Sonarr or Radarr) `deleteAfterDays` after it
entered. These tools show what is about to go, keep or delay it, and create and edit
the rule groups themselves, written in Maintainerr's own YAML. Anything that brings a
deletion closer is destructive-tier. See [Scope](../scope.md#maintainerr) for what is left out.

| Tool | Access |
|---|---|
| `maintainerr_system_status` | read |
| `maintainerr_list_collections` — each collection's action and grace period | read |
| `maintainerr_collection_media` — items waiting for the action, with their addDate | read |
| `maintainerr_list_rules`, `maintainerr_get_rule` — notification webhooks are never returned | read |
| `maintainerr_list_exclusions`, `maintainerr_media_status` | read |
| `maintainerr_rule_execution_status`, `maintainerr_overlay_status` | read |
| `maintainerr_execute_rules` — re-evaluates membership; does not run any action | write |
| `maintainerr_add_exclusion` — one collection, or every rule group | write |
| `maintainerr_postpone_deletion` — by N days, or restart the grace period | write |
| `maintainerr_process_overlays` | write |
| `maintainerr_list_libraries`, `maintainerr_list_arr_servers` — ids a new rule group needs; API keys are never returned | read |
| `maintainerr_list_rule_properties` — the `App.property` names rulesYaml uses | read |
| `maintainerr_test_rule` — dry-run a rule group against one item | read |
| `maintainerr_create_rule` — active, with an explicit action and a grace period of at least a day | write |
| `maintainerr_update_rule` — name, description, rulesYaml, schedule | write |
| `maintainerr_update_collection` — overlays and visibility | write |
| `maintainerr_remove_from_collection` | write |
| `maintainerr_set_deletion_policy` — arrAction, grace period, active | destructive |
| `maintainerr_add_to_collection` — schedules the item for the collection's action | destructive |
| `maintainerr_delete_rule` — also removes the media server collection | destructive |
| `maintainerr_remove_exclusion` — the item becomes deletable again | destructive |
