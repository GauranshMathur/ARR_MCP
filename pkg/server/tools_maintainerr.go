package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- maintainerr tool inputs ---

// CollectionMediaArgs selects one page of a Maintainerr collection.
type CollectionMediaArgs struct {
	InstanceArg
	CollectionID int `json:"collectionId" jsonschema:"collection id from maintainerr_list_collections"`
	Page         int `json:"page,omitempty" jsonschema:"page number starting at 1; defaults to 1"`
	Size         int `json:"size,omitempty" jsonschema:"items per page; defaults to 25"`
}

// ExclusionFilterArgs narrows maintainerr_list_exclusions.
type ExclusionFilterArgs struct {
	InstanceArg
	RuleGroupID   int    `json:"ruleGroupId,omitempty" jsonschema:"only exclusions for this rule group, plus global ones"`
	MediaServerID string `json:"mediaServerId,omitempty" jsonschema:"only exclusions for this media server item or its children"`
}

// MediaServerIDArgs identifies one media server item.
type MediaServerIDArgs struct {
	InstanceArg
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id, e.g. from maintainerr_collection_media"`
}

// ExecuteRulesArgs selects which rule groups to evaluate.
type ExecuteRulesArgs struct {
	InstanceArg
	RuleGroupID int `json:"ruleGroupId,omitempty" jsonschema:"rule group id from maintainerr_list_rules; omit to evaluate every active group"`
}

// AddExclusionArgs protects one item from Maintainerr.
type AddExclusionArgs struct {
	InstanceArg
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id, e.g. from maintainerr_collection_media"`
	CollectionID  int    `json:"collectionId,omitempty" jsonschema:"exclude from this collection's rule group only; omit to exclude from every rule group"`
}

// PostponeArgs delays one item's arrAction.
type PostponeArgs struct {
	InstanceArg
	CollectionID  int    `json:"collectionId" jsonschema:"collection id the item is in"`
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id from maintainerr_collection_media"`
	Days          int    `json:"days,omitempty" jsonschema:"days to push the deletion out, 1 to 3650; omit to restart the collection's full grace period"`
}

// CollectionIDArgs identifies one Maintainerr collection.
type CollectionIDArgs struct {
	InstanceArg
	CollectionID int `json:"collectionId" jsonschema:"collection id from maintainerr_list_collections"`
}

// RulePropertiesArgs narrows maintainerr_list_rule_properties.
type RulePropertiesArgs struct {
	InstanceArg
	Application string `json:"application,omitempty" jsonschema:"only this application, e.g. Radarr, Sonarr, Jellyfin, Seerr; omit for all"`
}

// TestRuleArgs dry-runs a rule group against one item.
type TestRuleArgs struct {
	InstanceArg
	RuleGroupID   int    `json:"ruleGroupId" jsonschema:"rule group id from maintainerr_list_rules"`
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id to test, e.g. from maintainerr_collection_media"`
}

// CreateRuleArgs is the input for maintainerr_create_rule. arrAction and
// deleteAfterDays have no defaults: an omitted action is DELETE upstream.
type CreateRuleArgs struct {
	InstanceArg
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	LibraryID       string `json:"libraryId" jsonschema:"library id from maintainerr_list_libraries; decides movie or show"`
	ArrAction       string `json:"arrAction" jsonschema:"what happens when the grace period ends: DELETE, UNMONITOR_DELETE_ALL, UNMONITOR_DELETE_EXISTING, UNMONITOR, DO_NOTHING, DELETE_SHOW_IF_EMPTY, UNMONITOR_SHOW_IF_EMPTY or CHANGE_QUALITY_PROFILE"`
	DeleteAfterDays int    `json:"deleteAfterDays" jsonschema:"grace period in days, 1 to 36500, between an item entering the collection and its arrAction"`
	ArrServerID     int    `json:"arrServerId,omitempty" jsonschema:"id from maintainerr_list_arr_servers of the kind the library needs (radarr for movies, sonarr for shows); required unless arrAction is DO_NOTHING"`
	RulesYAML       string `json:"rulesYaml" jsonschema:"conditions in Maintainerr's YAML; see maintainerr_get_rule for examples and maintainerr_list_rule_properties for property names"`
	OverlayEnabled  bool   `json:"overlayEnabled,omitempty" jsonschema:"show the deletion date on posters"`
}

// --- maintainerr tool outputs ---

// MaintainerrCollectionList wraps collection results.
type MaintainerrCollectionList struct {
	Collections []arr.MaintainerrCollection `json:"collections"`
	Count       int                         `json:"count"`
}

// MaintainerrRuleList wraps rule group results.
type MaintainerrRuleList struct {
	Rules []arr.MaintainerrRule `json:"rules"`
	Count int                   `json:"count"`
}

// MaintainerrExclusionList wraps exclusion results.
type MaintainerrExclusionList struct {
	Exclusions []arr.MaintainerrExclusion `json:"exclusions"`
	Count      int                        `json:"count"`
}

// RulesQueued reports that rule evaluation was queued.
type RulesQueued struct {
	RuleGroupID int  `json:"ruleGroupId,omitempty" jsonschema:"absent when every active group was queued"`
	Queued      bool `json:"queued"`
}

// ExclusionAdded reports a new exclusion.
type ExclusionAdded struct {
	MediaServerID string `json:"mediaServerId"`
	CollectionID  int    `json:"collectionId,omitempty" jsonschema:"absent for a global exclusion"`
	Excluded      bool   `json:"excluded"`
}

// MaintainerrLibraryList wraps library results.
type MaintainerrLibraryList struct {
	Libraries []arr.MaintainerrLibrary `json:"libraries"`
	Count     int                      `json:"count"`
}

// MaintainerrArrServerList wraps Radarr and Sonarr server results.
type MaintainerrArrServerList struct {
	Servers []arr.MaintainerrArrServer `json:"servers"`
	Count   int                        `json:"count"`
}

// MaintainerrRulePropertyList wraps rule property results.
type MaintainerrRulePropertyList struct {
	Properties []arr.MaintainerrRuleProperty `json:"properties"`
	Count      int                           `json:"count"`
}
