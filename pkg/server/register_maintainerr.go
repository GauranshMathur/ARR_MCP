package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// registerMaintainerr adds the Maintainerr tools. The surface stops short of
// anything that deletes media now: collection handling and /api/settings,
// which returns every stored credential, are left out.
func registerMaintainerr(s *Server) {
	const svc = "maintainerr"
	spec := arr.MaintainerrSpec

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_system_status",
		description: "Report a Maintainerr instance's version and whether an update is available.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.MaintainerrStatus, error) {
		return arr.MaintainerrGetStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_list_collections",
		description: "List Maintainerr collections: the media each rule group has marked, the action " +
			"taken when the grace period ends (DELETE removes files from disk), and how many days that is.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (MaintainerrCollectionList, error) {
		cols, err := arr.MaintainerrListCollections(ctx, c)
		return MaintainerrCollectionList{Collections: cols, Count: len(cols)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_collection_media",
		description: "List the items in a Maintainerr collection that are waiting for its action. " +
			"An item is acted on deleteAfterDays after its addDate. Use the mediaServerId with " +
			"maintainerr_add_exclusion to keep an item, or maintainerr_postpone_deletion to delay it.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in CollectionMediaArgs) (arr.MaintainerrMediaPage, error) {
		page, size := in.Page, in.Size
		if page <= 0 {
			page = 1
		}
		if size <= 0 {
			size = 25
		}
		return arr.MaintainerrCollectionMedia(ctx, c, in.CollectionID, page, size)
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_list_rules",
		description: "List Maintainerr rule groups, the rules that decide what enters each collection.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (MaintainerrRuleList, error) {
		rules, err := arr.MaintainerrListRules(ctx, c)
		return MaintainerrRuleList{Rules: rules, Count: len(rules)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_get_rule",
		description: "Show one Maintainerr rule group with its conditions and its collection's action.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.MaintainerrRuleDetail, error) {
		return arr.MaintainerrGetRule(ctx, c, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_list_exclusions",
		description: "List items Maintainerr will never act on. A null ruleGroupId is a global " +
			"exclusion; the id is what maintainerr_remove_exclusion takes.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ExclusionFilterArgs) (MaintainerrExclusionList, error) {
		ex, err := arr.MaintainerrListExclusions(ctx, c, in.RuleGroupID, in.MediaServerID)
		return MaintainerrExclusionList{Exclusions: ex, Count: len(ex)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_rule_execution_status",
		description: "Report whether Maintainerr is evaluating rules, and which rule groups are queued.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.MaintainerrRuleStatus, error) {
		return arr.MaintainerrRuleExecutionStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_overlay_status",
		description: "Report the scheduled overlay job's state. lastRun covers scheduled runs only, " +
			"so it stays null after maintainerr_process_overlays; that is not a failure.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.MaintainerrOverlayState, error) {
		return arr.MaintainerrOverlayStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_media_status",
		description: "Show which exclusions and manual collections apply to one media server item, " +
			"including those inherited from its show or season.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in MediaServerIDArgs) (arr.MaintainerrItemStatus, error) {
		return arr.MaintainerrMediaStatus(ctx, c, in.MediaServerID)
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_list_libraries",
		description: "List the media server libraries a Maintainerr rule group can target, with the libraryId maintainerr_create_rule needs.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (MaintainerrLibraryList, error) {
		libs, err := arr.MaintainerrListLibraries(ctx, c)
		return MaintainerrLibraryList{Libraries: libs, Count: len(libs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_list_arr_servers",
		description: "List the Radarr and Sonarr servers Maintainerr acts through. A movie rule group " +
			"needs a radarr id and a show rule group a sonarr id; ids repeat across the two kinds.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (MaintainerrArrServerList, error) {
		servers, err := arr.MaintainerrListArrServers(ctx, c)
		return MaintainerrArrServerList{Servers: servers, Count: len(servers)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_list_rule_properties",
		description: "List the properties Maintainerr rule conditions can compare, as the App.property " +
			"names rulesYaml uses, with each one's value type and allowed comparisons. Filter by application " +
			"to keep the list short. maintainerr_get_rule shows complete rulesYaml examples.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in RulePropertiesArgs) (MaintainerrRulePropertyList, error) {
		props, err := arr.MaintainerrListRuleProperties(ctx, c, in.Application)
		return MaintainerrRulePropertyList{Properties: props, Count: len(props)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_test_rule",
		description: "Dry-run a saved Maintainerr rule group against one media item: whether it would " +
			"enter the collection, and each condition's values and result. Changes nothing.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in TestRuleArgs) (arr.MaintainerrRuleTest, error) {
		return arr.MaintainerrTestRule(ctx, c, in.RuleGroupID, in.MediaServerID)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_create_rule",
		description: "Create an active Maintainerr rule group and its collection. Items the rules match " +
			"enter the collection on the next rule run, and arrAction runs deleteAfterDays after that. " +
			"Check a known title with maintainerr_test_rule afterwards.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CreateRuleArgs) (arr.MaintainerrRuleDetail, error) {
		return arr.MaintainerrCreateRule(ctx, c, arr.MaintainerrNewRule{
			Name: in.Name, Description: in.Description, LibraryID: in.LibraryID,
			ArrAction: in.ArrAction, DeleteAfterDays: in.DeleteAfterDays, ArrServerID: in.ArrServerID,
			RulesYAML: in.RulesYAML, OverlayEnabled: in.OverlayEnabled,
		})
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_update_rule",
		description: "Change a Maintainerr rule group's name, description, conditions (rulesYaml replaces " +
			"all of them) or schedule. Every other setting is kept. To change the action, grace period or " +
			"active state, use maintainerr_set_deletion_policy. Broadening the conditions of a collection " +
			"whose arrAction deletes files adds more items to it; check with maintainerr_test_rule first. " +
			"Replacing the conditions is refused while the collection acts with no grace period " +
			"(deleteAfterDays 0 and an action other than DO_NOTHING).",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateRuleArgs) (arr.MaintainerrRuleDetail, error) {
		return arr.MaintainerrUpdateRule(ctx, c, in.ID, arr.MaintainerrRulePatch{
			Name: in.Name, Description: in.Description, RulesYAML: in.RulesYAML,
			RuleHandlerCronSchedule: in.RuleHandlerCronSchedule,
		})
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_update_collection",
		description: "Change a Maintainerr collection's overlay and visibility settings. Nothing here affects deletion.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in MaintainerrUpdateCollectionArgs) (arr.MaintainerrRuleDetail, error) {
		return arr.MaintainerrUpdateCollection(ctx, c, in.CollectionID, arr.MaintainerrCollectionPatch{
			OverlayEnabled: in.OverlayEnabled, VisibleOnHome: in.VisibleOnHome,
			VisibleOnRecommended: in.VisibleOnRecommended,
		})
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_set_deletion_policy",
		description: "Change what a Maintainerr collection does to its items (arrAction), after how many " +
			"days, and whether it runs at all. A shorter grace period or a DELETE action applies to items " +
			"already in the collection.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeletionPolicyArgs) (arr.MaintainerrRuleDetail, error) {
		return arr.MaintainerrSetDeletionPolicy(ctx, c, in.CollectionID, arr.MaintainerrDeletionPolicy{
			ArrAction: in.ArrAction, DeleteAfterDays: in.DeleteAfterDays, IsActive: in.IsActive,
		})
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_delete_rule",
		description: "Delete a Maintainerr rule group and its collection, including the collection on the " +
			"media server. Items in it are no longer scheduled; nothing is deleted from disk.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		err := arr.MaintainerrDeleteRule(ctx, c, in.ID)
		return Deleted{ID: in.ID, Deleted: err == nil}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_add_to_collection",
		description: "Add a media item to a Maintainerr collection by hand. It is then scheduled for the " +
			"collection's arrAction like any rule match, deleteAfterDays from now.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in CollectionMembershipArgs) (MembershipChanged, error) {
		err := arr.MaintainerrSetCollectionMembership(ctx, c, in.CollectionID, in.MediaServerID, true)
		return MembershipChanged{CollectionID: in.CollectionID, MediaServerID: in.MediaServerID, InCollection: err == nil}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_remove_from_collection",
		description: "Remove a media item from a Maintainerr collection, cancelling its scheduled action. " +
			"A rule may add it back on its next run; use maintainerr_add_exclusion to keep it out.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CollectionMembershipArgs) (MembershipChanged, error) {
		err := arr.MaintainerrSetCollectionMembership(ctx, c, in.CollectionID, in.MediaServerID, false)
		return MembershipChanged{CollectionID: in.CollectionID, MediaServerID: in.MediaServerID, InCollection: err != nil}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_execute_rules",
		description: "Re-evaluate Maintainerr rules now, for one rule group or every active one. " +
			"This updates collection membership; it does not run any collection's action. " +
			"Follow progress with maintainerr_rule_execution_status.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in ExecuteRulesArgs) (RulesQueued, error) {
		err := arr.MaintainerrExecuteRules(ctx, c, in.RuleGroupID)
		return RulesQueued{RuleGroupID: in.RuleGroupID, Queued: err == nil}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_add_exclusion",
		description: "Exclude a media item so Maintainerr never acts on it, from one collection's rule " +
			"group or from all of them. Excluding a show covers its seasons and episodes.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddExclusionArgs) (ExclusionAdded, error) {
		err := arr.MaintainerrAddExclusion(ctx, c, in.MediaServerID, in.CollectionID)
		return ExclusionAdded{MediaServerID: in.MediaServerID, CollectionID: in.CollectionID, Excluded: err == nil}, err
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_postpone_deletion",
		description: "Delay the action on one item in a Maintainerr collection, by a number of days " +
			"or by restarting the full grace period. Returns the new deletionDate.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in PostponeArgs) (arr.MaintainerrPostponeResult, error) {
		return arr.MaintainerrPostpone(ctx, c, in.CollectionID, in.MediaServerID, in.Days)
	})

	register(s, svc, spec, toolMeta{
		name:        "maintainerr_process_overlays",
		description: "Render Maintainerr's deletion-date overlays for one collection now, and report the counts.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CollectionIDArgs) (arr.MaintainerrOverlayRun, error) {
		return arr.MaintainerrProcessOverlays(ctx, c, in.CollectionID)
	})

	register(s, svc, spec, toolMeta{
		name: "maintainerr_remove_exclusion",
		description: "Delete a Maintainerr exclusion by id from maintainerr_list_exclusions. The item " +
			"becomes eligible again, so a DELETE collection may remove it from disk on its next run.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		err := arr.MaintainerrRemoveExclusion(ctx, c, in.ID)
		return Deleted{ID: in.ID, Deleted: err == nil}, err
	})
}
