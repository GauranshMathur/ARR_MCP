package server

import (
	"context"

	"github.com/GauranshMathur/ARR_MCP/pkg/arr"
)

// prowlarrHint is repeated in every provider tool description because adding an
// indexer directly to Sonarr or Radarr is usually the wrong move: in a stack
// running Prowlarr, indexers are managed centrally and pushed out, and one
// added here is overwritten or orphaned by the next sync.
const prowlarrHint = "If Prowlarr manages this stack's indexers, add and edit them with the prowlarr_* " +
	"tools instead and let the sync push them here; kind=indexer is for a service that has no Prowlarr."

// kindHint explains the enum and what each family calls its enable switch.
const kindHint = "kind selects the family: indexer, downloadClient, notification or importList. " +
	"The enable flags differ per kind -- an indexer has enableRss, enableAutomaticSearch and " +
	"enableInteractiveSearch, a download client has enable, an import list has enableAutomaticAdd, " +
	"and a notification's onGrab/onDownload triggers keep their current values."

// registerProviders adds the generic provider management tools. Indexers,
// download clients, notifications and import lists are one resource shape at
// four routes, so one implementation covers all of them for both services.
func registerProviders(s *Server, svc string, spec arr.ServiceSpec) {
	register(s, svc, spec, toolMeta{
		name: svc + "_provider_schemas",
		description: "List the provider implementations " + svc + " can create, with the settings each " +
			"one accepts. Read this before " + svc + "_add_provider to learn the implementation name and " +
			"the field names to pass. Field values are never returned, only names, labels and types. " +
			kindHint,
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ProviderSchemaArgs) (ProviderSchemaList, error) {
		schemas, err := arr.ListProviderSchemas(ctx, c, in.Kind, in.Query, in.Limit)
		return ProviderSchemaList{Schemas: schemas, Count: len(schemas)}, err
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_get_provider",
		description: "Show one configured indexer, download client, notification or import list, " +
			"including its field names and values. Credential fields report *** instead of the value, " +
			"so use this to see which settings exist before changing them with " + svc + "_update_provider. " +
			kindHint,
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in ProviderIDArgs) (arr.ProviderDetail, error) {
		return arr.GetProvider(ctx, c, in.Kind, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_add_provider",
		description: "Add an indexer, download client, notification or import list to " + svc + ". " +
			"Take implementation and the field names from " + svc + "_provider_schemas; a provider that " +
			"authenticates needs its credentials in fields, e.g. apiKey or password. " +
			kindHint + " " + prowlarrHint,
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in AddProviderArgs) (arr.ProviderDetail, error) {
		return arr.AddProvider(ctx, c, arr.ProviderCreateRequest{
			Kind:           in.Kind,
			Implementation: in.Implementation,
			Name:           in.Name,
			Flags:          in.flags(),
			Priority:       in.Priority,
			Tags:           in.Tags,
			Fields:         in.Fields,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_update_provider",
		description: "Change one provider's settings. Only the arguments given are changed; everything " +
			"else keeps its current value, including stored credentials, which are sent back untouched " +
			"rather than overwritten with the mask a read shows. Use fields to rotate an apiKey or " +
			"change a host. " + kindHint + " " + prowlarrHint,
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in UpdateProviderArgs) (arr.ProviderDetail, error) {
		return arr.UpdateProvider(ctx, c, arr.ProviderUpdateRequest{
			Kind:     in.Kind,
			ID:       in.ID,
			Name:     in.Name,
			Flags:    in.flags(),
			Priority: in.Priority,
			Tags:     in.Tags,
			Fields:   in.Fields,
		})
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_test_provider",
		description: "Ask " + svc + " to contact one provider and report whether it answered. " +
			"A failing test is reported as isValid false with the reasons, not as an error. " +
			"This records the outcome against the provider, so it is a write. " + kindHint,
		access: AccessWrite,
	}, func(ctx context.Context, c *arr.Client, in ProviderIDArgs) (arr.ProviderTestResult, error) {
		return arr.TestProvider(ctx, c, in.Kind, in.ID)
	})

	register(s, svc, spec, toolMeta{
		name: svc + "_delete_provider",
		description: "Delete one indexer, download client, notification or import list from " + svc + ". " +
			"Its configuration goes with it, including any credential stored only there, and none of " +
			"it is recoverable. " + kindHint,
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in ProviderIDArgs) (Deleted, error) {
		if err := arr.DeleteProvider(ctx, c, in.Kind, in.ID); err != nil {
			return Deleted{ID: in.ID}, err
		}
		return Deleted{ID: in.ID, Deleted: true}, nil
	})
}
