package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// qbitDefaultListLimit caps a torrent listing when the caller does not ask
// for a size, because a large instance would otherwise flood the context.
const qbitDefaultListLimit = 100

// registerQBittorrent adds the torrent management tools. Every mutation is a
// form-encoded POST against qBittorrent's WebUI API v2; the bulk tools take
// torrent hashes joined into a selector, with ["all"] meaning every torrent.
func registerQBittorrent(s *Server) {
	const svc = "qbittorrent"
	spec := arr.QBittorrentSpec

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_system_status",
		description: "Report the qBittorrent application and WebUI API versions.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.QBittorrentVersion, error) {
		return arr.QBittorrentSystemStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_list_torrents",
		description: "List torrents with state, progress, speeds, ratio, category and tags. " +
			"Filter by state, category, tag or specific hashes. Returns the hashes every other qbittorrent tool needs.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in TorrentFilterArgs) (TorrentList, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = qbitDefaultListLimit
		}
		torrents, err := arr.QBittorrentListTorrents(ctx, c, arr.TorrentFilter{
			Filter: in.Filter, Category: in.Category, Tag: in.Tag, Hashes: in.Hashes, Limit: limit,
		})
		return TorrentList{Torrents: torrents, Count: len(torrents)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_torrent_files",
		description: "List the files inside one torrent with size, progress and download priority. Takes a hash from qbittorrent_list_torrents.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, in TorrentHashArgs) (TorrentFileList, error) {
		files, err := arr.QBittorrentTorrentFiles(ctx, c, in.Hash)
		return TorrentFileList{Files: files, Count: len(files)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_transfer_info",
		description: "Report global download and upload speeds, the configured limits, and whether the alternative speed limits are active.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.TransferInfo, error) {
		return arr.QBittorrentTransferInfo(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_list_categories",
		description: "List the categories torrents can be filed under, with their save paths.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (CategoryList, error) {
		cats, err := arr.QBittorrentListCategories(ctx, c)
		return CategoryList{Categories: cats, Count: len(cats)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_list_tags",
		description: "List every tag known to the qBittorrent instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (TagNameList, error) {
		tags, err := arr.QBittorrentListTags(ctx, c)
		return TagNameList{Tags: tags, Count: len(tags)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_add_torrent",
		description: "Add torrents by http, https or magnet link, optionally stopped, categorised, tagged or rate-limited. " +
			"qBittorrent adds asynchronously, so confirm with qbittorrent_list_torrents.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddTorrentArgs) (Requested, error) {
		err := arr.QBittorrentAddTorrent(ctx, c, arr.AddTorrentRequest{
			URLs: in.URLs, SavePath: in.SavePath, Category: in.Category, Tags: in.Tags,
			Stopped: in.Stopped, Rename: in.Rename,
			DownloadLimit: in.DownloadLimit, UploadLimit: in.UploadLimit,
			RatioLimit: in.RatioLimit, SeedingTimeLimit: in.SeedingTimeLimit, AutoTMM: in.AutoTMM,
		})
		if err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "torrents are added asynchronously; confirm with qbittorrent_list_torrents"}, nil
	})

	registerQBitBulkAction(s, svc, spec, "qbittorrent_stop_torrents",
		"Stop (pause) torrents. Takes hashes from qbittorrent_list_torrents, or [\"all\"] to stop every torrent.",
		"stop", arr.QBittorrentStopTorrents)
	registerQBitBulkAction(s, svc, spec, "qbittorrent_start_torrents",
		"Start (resume) stopped torrents. Takes hashes from qbittorrent_list_torrents, or [\"all\"] to start every torrent.",
		"start", arr.QBittorrentStartTorrents)
	registerQBitBulkAction(s, svc, spec, "qbittorrent_recheck_torrents",
		"Re-verify torrent data on disk. Rechecking is slow for large torrents. Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		"recheck", arr.QBittorrentRecheckTorrents)

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_set_category",
		description: "File torrents under a category from qbittorrent_list_categories; an empty category clears it. " +
			"Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentCategoryArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentSetCategory(ctx, c, in.Hashes, in.Category); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "setCategory", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_create_category",
		description: "Create a category, optionally with its own save path.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CategoryArgs) (Requested, error) {
		if err := arr.QBittorrentCreateCategory(ctx, c, in.Name, in.SavePath); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_edit_category",
		description: "Change the save path of an existing category from qbittorrent_list_categories.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in CategoryArgs) (Requested, error) {
		if err := arr.QBittorrentEditCategory(ctx, c, in.Name, in.SavePath); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_add_tags",
		description: "Apply tags to torrents, creating any tag that does not exist yet. " +
			"Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentTagsArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentAddTags(ctx, c, in.Hashes, in.Tags); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "addTags", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_remove_tags",
		description: "Strip tags from torrents; omit tags to strip every tag they carry. " +
			"Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentTagsArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentRemoveTags(ctx, c, in.Hashes, in.Tags); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "removeTags", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_set_location",
		description: "Move torrents' data to another directory; qBittorrent moves the files on disk. " +
			"Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentLocationArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentSetLocation(ctx, c, in.Hashes, in.Location); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "setLocation", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "qbittorrent_rename_torrent",
		description: "Change one torrent's display name. Renames nothing on disk. Takes a hash from qbittorrent_list_torrents.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentRenameArgs) (Requested, error) {
		if err := arr.QBittorrentRenameTorrent(ctx, c, in.Hash, in.Name); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_set_priority",
		description: "Move torrents in the download queue: top, bottom, up or down. Fails when torrent queueing is " +
			"disabled in qBittorrent. Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentPriorityArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentSetPriority(ctx, c, in.Hashes, in.Position); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: in.Position, Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_set_torrent_limits",
		description: "Set per-torrent speed limits (bytes per second, 0 unlimited) and share limits (ratio and " +
			"seeding minutes; -1 no limit, -2 the global default). Omitted fields are left untouched. " +
			"Takes hashes from qbittorrent_list_torrents, or [\"all\"].",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentLimitsArgs) (TorrentActionResult, error) {
		var share *arr.ShareLimits
		if in.RatioLimit != nil || in.SeedingTimeLimit != nil || in.InactiveSeedingTimeLimit != nil {
			share = &arr.ShareLimits{
				RatioLimit:               in.RatioLimit,
				SeedingTimeLimit:         in.SeedingTimeLimit,
				InactiveSeedingTimeLimit: in.InactiveSeedingTimeLimit,
			}
		}
		if err := arr.QBittorrentSetTorrentLimits(ctx, c, in.Hashes, in.DownloadLimit, in.UploadLimit, share); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "setLimits", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_set_global_limits",
		description: "Set the global download or upload limit (bytes per second, 0 unlimited) or switch the " +
			"alternative speed limits on or off. Returns the refreshed transfer state.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in GlobalLimitsArgs) (arr.TransferInfo, error) {
		return arr.QBittorrentSetGlobalLimits(ctx, c, in.DownloadLimit, in.UploadLimit, in.AlternativeMode)
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_delete_torrents",
		description: "Remove torrents from qBittorrent, optionally deleting their downloaded files from disk, " +
			"which cannot be undone. Takes hashes from qbittorrent_list_torrents, or [\"all\"] for every torrent.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in DeleteTorrentsArgs) (TorrentActionResult, error) {
		if err := arr.QBittorrentDeleteTorrents(ctx, c, in.Hashes, in.DeleteFiles); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: "delete", Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})

	register(s, svc, spec, toolMeta{
		name: "qbittorrent_delete_categories",
		description: "Delete categories by name from qbittorrent_list_categories. Torrents filed under them " +
			"become uncategorised; no files are touched.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in CategoryNamesArgs) (Requested, error) {
		if err := arr.QBittorrentRemoveCategories(ctx, c, in.Names); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true}, nil
	})
}

// registerQBitBulkAction registers one hashes-only torrent action, which stop,
// start and recheck all are.
func registerQBitBulkAction(
	s *Server, svc string, spec arr.ServiceSpec, name, description, action string,
	fn func(context.Context, *arr.Client, []string) error,
) {
	register(s, svc, spec, toolMeta{
		name:        name,
		description: description,
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in TorrentHashesArgs) (TorrentActionResult, error) {
		if err := fn(ctx, c, in.Hashes); err != nil {
			return TorrentActionResult{}, err
		}
		return TorrentActionResult{Action: action, Hashes: in.Hashes, Count: len(in.Hashes)}, nil
	})
}
