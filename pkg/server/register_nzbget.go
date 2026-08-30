package server

import (
	"context"
	"fmt"
	"strconv"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// defaultNZBHistoryLimit caps nzbget_history when no limit is given, because
// NZBGet returns the entire history in one response.
const defaultNZBHistoryLimit = 20

// requireNZBIDs rejects an empty id list before NZBGet is contacted; editqueue
// with no ids answers true while doing nothing, which would report success.
func requireNZBIDs(ids []int, source string) error {
	if len(ids) == 0 {
		return fmt.Errorf("ids is required; take NZBID values from %s", source)
	}
	return nil
}

// pauseScope applies the download default shared by the pause and resume tools.
func pauseScope(scope string) string {
	if scope == "" {
		return "download"
	}
	return scope
}

// registerNZBGet adds the NZBGet download client tools.
func registerNZBGet(s *Server) {
	const svc = "nzbget"
	spec := arr.NZBGetSpec

	register(s, svc, spec, toolMeta{
		name: "nzbget_status",
		description: "Report NZBGet server state: version, download speed and limit, queued and " +
			"free disk sizes, and whether the download, post-processing and scan queues are paused.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.NZBStatus, error) {
		return arr.NZBGetStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_list_queue",
		description: "List the NZBGet download queue, one entry per nzb with its NZBID, status, " +
			"sizes, health and priority. Every queue editing tool takes ids from here.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (NZBGroupList, error) {
		groups, err := arr.NZBGetListGroups(ctx, c)
		return NZBGroupList{Groups: groups, Count: len(groups)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_history",
		description: "List finished, failed and deleted downloads, newest first. hidden also " +
			"returns entries removed non-finally and old duplicate records. The history tools " +
			"take ids from here.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in NZBHistoryArgs) (NZBHistoryList, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = defaultNZBHistoryLimit
		}
		items, err := arr.NZBGetHistory(ctx, c, in.Hidden, limit)
		return NZBHistoryList{Items: items, Count: len(items)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_add_nzb",
		description: "Add a download to NZBGet from an nzb URL or base64-encoded nzb content " +
			"(exactly one of the two). Returns the NZBID for the queue tools.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddNZBArgs) (NZBAdded, error) {
		id, err := arr.NZBGetAppend(ctx, c, arr.AppendNZBRequest{
			Filename: in.Filename, URL: in.URL, Content: in.Content, Category: in.Category,
			Priority: in.Priority, AddToTop: in.AddToTop, AddPaused: in.AddPaused,
			DupeKey: in.DupeKey, DupeScore: in.DupeScore, DupeMode: in.DupeMode,
		})
		return NZBAdded{NZBID: id}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_pause_download",
		description: "Pause one of NZBGet's queues: download (all downloading, the default), " +
			"post (post-processing) or scan (watching the incoming nzb directory). To pause " +
			"individual queue items use nzbget_pause_items instead.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in PauseScopeArgs) (PauseState, error) {
		scope := pauseScope(in.Scope)
		return PauseState{Scope: scope, Paused: true}, arr.NZBGetSetPaused(ctx, c, scope, true)
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_resume_download",
		description: "Resume a paused NZBGet queue: download (the default), post or scan. " +
			"Items paused individually stay paused; resume those with nzbget_resume_items.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in PauseScopeArgs) (PauseState, error) {
		scope := pauseScope(in.Scope)
		return PauseState{Scope: scope, Paused: false}, arr.NZBGetSetPaused(ctx, c, scope, false)
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_pause_items",
		description: "Pause specific queue entries. ids from nzbget_list_queue. The rest of " +
			"the queue keeps downloading.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBIDsArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return Updated{}, err
		}
		err := arr.NZBGetEditQueue(ctx, c, "GroupPause", "", in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "nzbget_resume_items",
		description: "Resume specific paused queue entries. ids from nzbget_list_queue.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBIDsArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return Updated{}, err
		}
		err := arr.NZBGetEditQueue(ctx, c, "GroupResume", "", in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_move_items",
		description: "Reorder queue entries: position top, bottom, or offset with a number of " +
			"positions (negative moves towards the top). ids from nzbget_list_queue.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBMoveArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return Updated{}, err
		}
		var command, param string
		switch in.Position {
		case "top":
			command = "GroupMoveTop"
		case "bottom":
			command = "GroupMoveBottom"
		case "offset":
			command, param = "GroupMoveOffset", strconv.Itoa(in.Offset)
		default:
			return Updated{}, fmt.Errorf("unknown position %q; valid positions: top, bottom, offset", in.Position)
		}
		err := arr.NZBGetEditQueue(ctx, c, command, param, in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_set_priority",
		description: "Set the download priority of queue entries. ids from nzbget_list_queue. " +
			"900 (force) downloads even while the queue is paused.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBPriorityArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return Updated{}, err
		}
		err := arr.NZBGetEditQueue(ctx, c, "GroupSetPriority", strconv.Itoa(in.Priority), in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_set_category",
		description: "Change the category of queue entries, which controls where completed " +
			"files land and which post-processing runs. ids from nzbget_list_queue.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBCategoryArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return Updated{}, err
		}
		err := arr.NZBGetEditQueue(ctx, c, "GroupSetCategory", in.Category, in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "nzbget_rename_item",
		description: "Rename one queue entry. id from nzbget_list_queue.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBRenameArgs) (Updated, error) {
		err := arr.NZBGetEditQueue(ctx, c, "GroupSetName", in.Name, []int{in.ID})
		return Updated{Updated: 1}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_retry_history_items",
		description: "Return history entries to the download queue. By default the " +
			"already-downloaded data is kept and only the rest is fetched; redownload starts " +
			"over from scratch. ids from nzbget_history.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBRetryArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_history"); err != nil {
			return Updated{}, err
		}
		command := "HistoryReturn"
		if in.Redownload {
			command = "HistoryRedownload"
		}
		err := arr.NZBGetEditQueue(ctx, c, command, "", in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_mark_history_items",
		description: "Mark history entries good or bad for duplicate handling. Marking bad " +
			"also downloads the next queued duplicate if one exists. ids from nzbget_history.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in NZBMarkArgs) (Updated, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_history"); err != nil {
			return Updated{}, err
		}
		var command string
		switch in.Mark {
		case "good":
			command = "HistoryMarkGood"
		case "bad":
			command = "HistoryMarkBad"
		default:
			return Updated{}, fmt.Errorf("unknown mark %q; valid marks: good, bad", in.Mark)
		}
		err := arr.NZBGetEditQueue(ctx, c, command, "", in.IDs)
		return Updated{Updated: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "nzbget_set_rate_limit",
		description: "Set the download speed limit in KiB/s. 0 removes the limit.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in RateLimitArgs) (RateLimit, error) {
		return RateLimit{LimitKB: in.LimitKB}, arr.NZBGetRate(ctx, c, in.LimitKB)
	})

	register(s, svc, spec, toolMeta{
		name:        "nzbget_scan",
		description: "Scan the incoming nzb directory now instead of waiting for the next interval.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (Requested, error) {
		if err := arr.NZBGetScan(ctx, c); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_delete_items",
		description: "Delete entries from the download queue. By default they move to history, " +
			"from where nzbget_retry_history_items can bring them back; final skips history and " +
			"discards them permanently. ids from nzbget_list_queue.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in NZBDeleteArgs) (DeletedCount, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_list_queue"); err != nil {
			return DeletedCount{}, err
		}
		command := "GroupDelete"
		if in.Final {
			command = "GroupFinalDelete"
		}
		err := arr.NZBGetEditQueue(ctx, c, command, "", in.IDs)
		return DeletedCount{Deleted: len(in.IDs)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "nzbget_delete_history_items",
		description: "Delete entries from history. By default they are only hidden and still " +
			"appear under nzbget_history with hidden=true; final removes them permanently. " +
			"ids from nzbget_history.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in NZBDeleteArgs) (DeletedCount, error) {
		if err := requireNZBIDs(in.IDs, "nzbget_history"); err != nil {
			return DeletedCount{}, err
		}
		command := "HistoryDelete"
		if in.Final {
			command = "HistoryFinalDelete"
		}
		err := arr.NZBGetEditQueue(ctx, c, command, "", in.IDs)
		return DeletedCount{Deleted: len(in.IDs)}, err
	})
}
