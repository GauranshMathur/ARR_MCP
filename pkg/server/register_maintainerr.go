package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// registerMaintainerr adds the Maintainerr tools. The surface stops short of
// anything that deletes media now: collection handling, rule and collection
// edits, and /api/settings, which returns every stored credential, are left out.
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
