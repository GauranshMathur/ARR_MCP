package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// applicationSyncCommand pushes Prowlarr's indexer list to every connected app.
const applicationSyncCommand = "ApplicationIndexerSync"

// registerProwlarr adds the indexer management tools.
func registerProwlarr(s *Server) {
	const svc = "prowlarr"
	spec := arr.ProwlarrSpec

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_list_indexers",
		description: "List the indexers configured in Prowlarr.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (IndexerList, error) {
		indexers, err := arr.ProwlarrListIndexers(ctx, c)
		return IndexerList{Indexers: indexers, Count: len(indexers)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_get_indexer",
		description: "Show one indexer's settings, including its field names and values. " +
			"Credential fields report *** instead of the value, so use this to see which " +
			"settings exist before changing them with prowlarr_update_indexer.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.IndexerDetail, error) {
		return arr.ProwlarrGetIndexer(ctx, c, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_list_indexer_schemas",
		description: "Search the indexer definitions Prowlarr ships, by site or definition name. " +
			"Prowlarr knows hundreds, so pass a query; the result gives the definitionName " +
			"that prowlarr_add_indexer needs.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in SchemaQueryArgs) (IndexerSchemaList, error) {
		schemas, err := arr.ProwlarrListIndexerSchemas(ctx, c, in.Query, in.Limit)
		return IndexerSchemaList{Schemas: schemas, Count: len(schemas)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_get_indexer_schema",
		description: "Show one indexer definition with the settings it accepts. Read this before " +
			"prowlarr_add_indexer to learn the field names to pass. Where several presets share " +
			"a definitionName, as every Newznab and Torznab preset does, the first is returned " +
			"as the template and its baseUrl must be set explicitly.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in DefinitionArgs) (arr.IndexerSchema, error) {
		return arr.ProwlarrGetIndexerSchema(ctx, c, in.DefinitionName)
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_search",
		description: "Search all Prowlarr indexers for releases matching a query. Each result " +
			"carries a guid and an indexerId; pass both to prowlarr_grab_release to download it.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ProwlarrSearchArgs) (ReleaseList, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 25
		}
		releases, err := arr.ProwlarrSearch(ctx, c, in.Query, in.Categories, limit)
		return ReleaseList{Releases: releases, Count: len(releases)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_list_applications",
		description: "List the Sonarr, Radarr and other instances Prowlarr syncs its indexers to.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ApplicationList, error) {
		apps, err := arr.ProwlarrListApplications(ctx, c)
		return ApplicationList{Applications: apps, Count: len(apps)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_list_download_clients",
		description: "List the download clients Prowlarr sends grabbed releases to.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (ProviderList, error) {
		clients, err := arr.ListDownloadClients(ctx, c)
		return ProviderList{Providers: clients, Count: len(clients)}, err
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_list_app_profiles",
		description: "List Prowlarr sync profiles, which decide how indexers behave in the apps " +
			"they are pushed to. Needed before adding an indexer with a non-default profile.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (AppProfileList, error) {
		profiles, err := arr.ProwlarrListAppProfiles(ctx, c)
		return AppProfileList{Profiles: profiles, Count: len(profiles)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_list_tags",
		description: "List the tags configured in Prowlarr.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (TagList, error) {
		tags, err := arr.ListTags(ctx, c)
		return TagList{Tags: tags, Count: len(tags)}, err
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_system_status",
		description: "Report version and health information for a Prowlarr instance.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.SystemStatus, error) {
		return arr.GetSystemStatus(ctx, c)
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_indexer_stats",
		description: "Report query, grab and failure counts per Prowlarr indexer. Use to find failing indexers.",
		access:      AccessRead,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (IndexerStatList, error) {
		stats, err := arr.ProwlarrIndexerStats(ctx, c)
		return IndexerStatList{Stats: stats}, err
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_add_indexer",
		description: "Add an indexer from one of Prowlarr's definitions. Take definitionName from " +
			"prowlarr_list_indexer_schemas and the field names from prowlarr_get_indexer_schema; " +
			"private indexers need their credentials in fields, e.g. apiKey.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddIndexerArgs) (arr.IndexerDetail, error) {
		return arr.ProwlarrAddIndexer(ctx, c, arr.IndexerCreateRequest{
			DefinitionName: in.DefinitionName,
			Name:           in.Name,
			Enable:         in.Enable,
			Priority:       in.Priority,
			AppProfileID:   in.AppProfileID,
			Tags:           in.Tags,
			Fields:         in.Fields,
		})
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_update_indexer",
		description: "Change one indexer's settings. Only the arguments given are changed; " +
			"everything else keeps its current value. Use fields to rotate an indexer's apiKey " +
			"or change its baseUrl.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateIndexerArgs) (arr.IndexerDetail, error) {
		return arr.ProwlarrUpdateIndexer(ctx, c, arr.IndexerUpdateRequest{
			ID:           in.ID,
			Name:         in.Name,
			Enable:       in.Enable,
			Priority:     in.Priority,
			AppProfileID: in.AppProfileID,
			Tags:         in.Tags,
			Fields:       in.Fields,
		})
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_test_indexer",
		description: "Ask Prowlarr to contact one indexer and report whether it answered. " +
			"A failing test is reported as isValid false with the reasons, not as an error.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (arr.IndexerTestResult, error) {
		return arr.ProwlarrTestIndexer(ctx, c, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_test_all_indexers",
		description: "Test every configured indexer at once and report which ones failed.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (IndexerTestList, error) {
		results, err := arr.ProwlarrTestAllIndexers(ctx, c)
		failed := 0
		for _, r := range results {
			if !r.IsValid {
				failed++
			}
		}
		return IndexerTestList{Results: results, Count: len(results), Failed: failed}, err
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_sync_applications",
		description: "Push Prowlarr's indexer list to every connected application. Run this after " +
			"adding, editing or deleting an indexer so Sonarr and Radarr pick up the change.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, _ EmptyArgs) (arr.CommandResult, error) {
		return arr.RunCommand(ctx, c, applicationSyncCommand, nil)
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_grab_release",
		description: "Send a release from prowlarr_search to the download client its indexer " +
			"uses. Requires both the guid and the indexerId from the search result.",
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in GrabArgs) (Requested, error) {
		if err := arr.ProwlarrGrabRelease(ctx, c, in.GUID, in.IndexerID); err != nil {
			return Requested{}, err
		}
		return Requested{Requested: true, Detail: "release handed to the download client; " +
			"check prowlarr_history to confirm the grab"}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_create_tag",
		description: "Create a tag in Prowlarr, for grouping indexers or scoping applications.",
		access:      AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in LabelArgs) (arr.Tag, error) {
		return arr.CreateTag(ctx, c, in.Label)
	})

	register(s, svc, spec, toolMeta{
		name: "prowlarr_delete_indexer",
		description: "Delete an indexer from Prowlarr. It is also removed from every application " +
			"Prowlarr syncs to, and its configuration is not recoverable.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.ProwlarrDeleteIndexer(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})

	register(s, svc, spec, toolMeta{
		name:        "prowlarr_delete_tag",
		description: "Delete a Prowlarr tag, detaching it from every indexer and application that carried it.",
		access:      AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		if err := arr.DeleteTag(ctx, c, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}
