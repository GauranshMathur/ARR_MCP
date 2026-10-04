# Maintainerr rule and collection management — design

Status: draft for review · 2026-09-28 · builds on v1.6.0 (14 Maintainerr tools)

## Goal

Let an MCP client create, edit, dry-run and delete Maintainerr rule groups, change
collection settings, and add or remove items from a collection by hand, without
anyone having to read or write Maintainerr's numeric rule encoding.

**Success:** from a conversation you can say "add a rule group for Movies that
deletes anything unwatched for 90 days, after 30 days' grace", and review the
rules as readable YAML in the confirmation prompt. Then `test_rule` on a known title
shows whether it matches, and every change that brings a deletion closer asks first.

## Decisions already made

| Question | Decision |
|---|---|
| Scope | Rule groups and their collections, collection settings, manual membership |
| Rule format | Approach A: Maintainerr's own YAML (`/rules/yaml/encode`, `/rules/yaml/decode`) |
| New rule groups | Created **active**; `arrAction` and `deleteAfterDays` must be given explicitly, never defaulted |
| Deletion | `maintainerr_delete_rule` included, destructive tier |
| Tiers | A change that brings deletion closer is destructive; other changes are write |

## Verified API facts (live v3.27.0 + source at tag v3.27.0)

- `POST /api/rules/yaml/encode` `{rules: "<JSON string of RuleDto[]>", mediaType}` and
  `POST /api/rules/yaml/decode` `{yaml, mediaType}` are pure transforms. Neither writes to
  the database. Both answer `{code, result}`; decode's `result` is a JSON string
  `{mediaType, rules: RuleDto[]}`, and `skipped` counts rules it could not resolve.
  Encoding the live Movies group gives readable YAML, for example
  `firstValue: Seerr.mediaAddedAt / action: BEFORE / customValue: {type: custom_days, value: "15"}`.
- `POST /api/rules` creates a rule group **and** its collection. The body is `RuleGroupDto`:
  `libraryId, name, description, dataType, isActive, arrAction, useRules, rules: RuleDto[]`,
  `radarrSettingsId` or `sonarrSettingsId`, and collection settings nested under
  `collection` (`deleteAfterDays`, `overlayEnabled`, …). A missing `deleteAfterDays` is
  stored as null, which means "never".
- `PUT /api/rules` updates a group. **It does not fall back to stored values** for
  `collection.overlayEnabled`, `sortTitle`, `mediaServerSort`, `overlayTemplateId` or
  `keepLogsForMonths` (it resets the last to 6). Every update must therefore read the
  current group, apply only the changed fields, and send the whole object.
- `DELETE /api/rules/{id}` deletes the group and its collection, including the media
  server collection, and strips *arr membership tags. It answers `ReturnStatus`, where
  `code 0` means it refused.
- `POST /api/rules/test` `{rulegroupId, mediaId}` dry-runs an **existing** group
  against one item. It answers `{code: 1, result: IComparisonStatistics[]}`: per item,
  per section, and per rule, with first and second values and the reason for each.
- `POST /api/collections/media/add` `{action: 0|1, mediaId, collectionId, context:{id, type}}`
  adds (0) or removes (1) one item by hand. `context.type` comes from
  `GET /api/media-server/meta/{id}`. Failures are real 4xx/5xx statuses.
- `GET /api/rules/constants` lists applications and their properties with the
  comparisons each allows (`RulePossibility`: BIGGER … NOT_EXISTS).
- `GET /api/media-server/libraries` gives `{id, title, type}`. This is safe to return as is.
- `GET /api/settings/radarr|sonarr` gives `{id, serverName, url, apiKey}`. **Decode only
  id, serverName and url**, the same never-decode rule used for notification webhooks.

## Tools

All tools stay in `register_maintainerr.go`, `tools_maintainerr.go` and
`pkg/arr/maintainerr.go` (or a new `maintainerr_rules.go` if that file passes ~700 lines).

| Tool | Tier | Does |
|---|---|---|
| `maintainerr_list_libraries` | read | Media server libraries (id, title, type) |
| `maintainerr_list_arr_servers` | read | Radarr/Sonarr servers Maintainerr knows (id, serverName, url; never apiKey) |
| `maintainerr_list_rule_properties` | read | Per application: property `App.name`, humanName, value type, allowed comparisons. Optional `application` filter to keep it small. |
| `maintainerr_get_rule` *(existing)* | read | Gains a `rulesYaml` field, produced by encode |
| `maintainerr_test_rule` | read | Dry-run a group against one mediaServerId; returns per-rule results |
| `maintainerr_create_rule` | write | See below |
| `maintainerr_update_rule` | write | name, description, rulesYaml, ruleHandlerCronSchedule |
| `maintainerr_update_collection` | write | overlayEnabled, visibleOnHome, visibleOnRecommended |
| `maintainerr_remove_from_collection` | write | Manual removal (action 1) |
| `maintainerr_set_deletion_policy` | destructive | arrAction, deleteAfterDays, isActive |
| `maintainerr_add_to_collection` | destructive | Manual add (action 0): the item is now scheduled for the collection's action |
| `maintainerr_delete_rule` | destructive | Deletes the group and its collection |

### How "brings deletion closer" is implemented

The `register` helper fixes a tool's tier at registration, so the tier cannot depend
on the arguments. Rather than change that shared helper, the settings that can bring
a deletion closer (`arrAction`, `deleteAfterDays`, `isActive`) get their own tool,
`set_deletion_policy`, which is always destructive. It prompts even when a change is
safe, such as lengthening the grace period. That is the conservative side of the
rule, and every other edit tool is honestly write-tier. Manual add is destructive for
the same reason.

`create_rule` is write-tier even though it creates an active group with an explicit
action. A new collection is empty, and every item it later gains waits out the grace
period, which is visible and can be postponed or excluded. To keep that true,
**`create_rule` rejects `deleteAfterDays < 1`**. A group that acts immediately has to be
created and then changed with `set_deletion_policy`, which prompts.

### `maintainerr_create_rule` input

```
name            string   required
description     string
libraryId       string   required  — from list_libraries; dataType taken from the library type
arrAction       string   required  — DELETE, UNMONITOR, DO_NOTHING, … (names, not numbers)
deleteAfterDays int      required, 1..36500
arrServerId     int      required unless arrAction is DO_NOTHING — from list_arr_servers;
                           sent as radarrSettingsId for movie libraries, sonarrSettingsId for show
rulesYaml       string   required  — Maintainerr YAML; mediaType inferred from the library
overlayEnabled  bool     optional, default false
```

Flow: decode YAML → reject if `code 0` or `skipped > 0` (naming the count, because a
silently dropped condition widens the match) → `POST /api/rules` → return the new
group via `get_rule`, including `rulesYaml`.

## Error handling

- YAML decode `code 0` → error carrying Maintainerr's message, e.g. an unknown property.
- `skipped > 0` → error "N rule(s) could not be resolved", never a partial save.
- `ReturnStatus code 0` on create, update or delete → error with `result`/`message`.
- Unknown rule group id → the existing "no rule group with id N".
- Update: the read-modify-write sends the stored `rules` back unchanged when
  `rulesYaml` is omitted. This needs verifying: `RuleGroupDto.rules` accepts
  `RuleDbDto[]` on paper.

## Testing

- **Unit (TDD, `maintainerrRoutes` fake):** YAML decode then create body; skipped>0 is
  rejected; `deleteAfterDays` 0 is rejected; `arrServerId` is mapped to
  radarr/sonarrSettingsId by library type; update preserves `overlayEnabled`,
  `sortTitle`, `keepLogsForMonths` and the rules; the arr-server list never contains
  apiKey (fake body includes one); code 0 surfaces as an error; manual add sends
  context type from metadata.
- **Server:** tier lists (read, write, destructive), readonly hides every mutator, and
  destructive annotations on the three destructive tools.
- **Live, with your OK at the time:** create a throwaway group on the Movies library with
  `arrAction DO_NOTHING`, run `test_rule`, update it, confirm overlays and schedule
  survive, then delete it. Nothing else is written live.

## Out of scope

Collection-only groups (`useRules: false`), community rules, notification editing,
`/collections/handle`, `/api/settings` writes, cron schedule for the global rule
handler, and changing a group's library or data type (Maintainerr rebuilds the
collection when these change).
