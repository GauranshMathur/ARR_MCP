# Maintainerr Rule and Collection Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 11 Maintainerr tools so an MCP client can create, edit, dry-run and
delete rule groups written in Maintainerr's own YAML, change collection settings, and
add or remove collection items by hand.

**Architecture:** Every call goes through the existing `arr.Client` (`AuthNone`,
`/api` base path). Rules cross the MCP boundary only as YAML: Maintainerr's
`/rules/yaml/decode` and `/rules/yaml/encode` convert to and from its numeric
`RuleDto`, which never reaches the model. Every edit is a read-modify-write of the
whole rule group, because `PUT /api/rules` resets any field it is not sent.

**Tech Stack:** Go 1.x, `github.com/modelcontextprotocol/go-sdk/mcp`, `net/http/httptest` for fakes.

**Spec:** `docs/maintainerr-rule-management-design.md`

## Global Constraints

- Conventions in `AGENTS.md` are binding: TDD (watch every test fail for the right reason), never write to stdout, conventional commits, trimmed outputs wrapped in structs with a count.
- Register every tool through `register[In, Out]` in `pkg/server/tools.go`; every input embeds `InstanceArg`.
- Tiers: `list_libraries`, `list_arr_servers`, `list_rule_properties`, `test_rule` → `AccessRead`; `create_rule`, `update_rule`, `update_collection`, `remove_from_collection` → `AccessWrite`; `set_deletion_policy`, `add_to_collection`, `delete_rule` → `AccessDestructive`.
- `create_rule`: `arrAction` and `deleteAfterDays` are required; `deleteAfterDays` must be 1..36500; created with `isActive: true`.
- `set_deletion_policy`: `deleteAfterDays` must be 0..36500.
- Never decode `apiKey` from `/settings/radarr|sonarr` or `notifications` from a rule group into any struct returned to the model.
- A YAML decode with `skipped > 0` is an error; nothing is saved.
- Calls that wait on Maintainerr's execution lock, rule evaluation or the media server use `c.WithTimeout(maintainerrSlowTimeout)` (5 minutes, already defined in `pkg/arr/maintainerr.go`).
- Before claiming done: `go test ./... -race`, `go vet ./...`, `gofmt -l .` empty, `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run` 0 issues, `go run github.com/securego/gosec/v2/cmd/gosec@latest -quiet ./...` clean, and the Trivy command in AGENTS.md.

## Review Focus

1. **Server id from the wrong *arr**: Radarr and Sonarr ids overlap (both have id 1 live). A Sonarr id passed for a movie library must be rejected with the valid Radarr ids listed, not stored as radarrSettingsId. Test in Task 4.
2. **Two rule groups with the same name on the same library**: create must return the newest (highest id), not the older one. Test in Task 4.
3. **A stored rule that is not a JSON string**: an edit must fail before any PUT rather than write a corrupted rule set. Test in Task 5.
4. **An edit with nothing to change**: must error without calling PUT, since a no-op PUT still resets fields Maintainerr does not fall back on. Test in Task 5.
5. **An unknown media id on manual add/remove**: metadata answers 200 with an empty body; the tool must name the id and send no POST. Test in Task 7.

## File Structure

| File | Responsibility |
|---|---|
| `pkg/arr/maintainerr.go` (modify) | Shared helpers only: `maintainerrEnumName`, `maintainerrDecodeStatus`, `maintainerrValidateMediaID`; `MaintainerrGetRule` gains `RulesYAML`. |
| `pkg/arr/maintainerr_rules.go` (create) | Rule YAML, lookups, test, create, update, delete, manual membership. |
| `pkg/arr/maintainerr_rules_test.go` (create) | Unit tests for the above, using `maintainerrRoutes` from `maintainerr_test.go`. |
| `pkg/server/tools_maintainerr.go` (modify) | New input and output structs. |
| `pkg/server/register_maintainerr.go` (modify) | New registrations. |
| `pkg/server/register_maintainerr_test.go` (modify) | Tier lists, a routed fake, tool-level tests. |
| `README.md` (modify) | Tool count and Maintainerr table and scope. |

---

### Task 1: Rule YAML round-trip and `rulesYaml` on `get_rule`

**Files:**
- Modify: `pkg/arr/maintainerr.go`
- Create: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/arr/maintainerr_test.go` (`TestMaintainerrRulesDropNotificationSecrets` gains an encode route)
- Modify: `pkg/server/register_maintainerr_test.go` (routed fake; webhook test uses it)

**Interfaces:**
- Produces: `maintainerrDecodeStatus(body []byte, action string) (maintainerrReturnStatus, error)`; `maintainerrEncodeRules(ctx, c, rules []json.RawMessage, mediaType string) (string, error)`; `maintainerrDecodeRules(ctx, c, yaml, mediaType string) ([]json.RawMessage, error)`; `MaintainerrRuleDetail.RulesYAML string` (json `rulesYaml`); server test helper `routedArr(t, routes map[string]string) (*httptest.Server, *[]string, *[]string)`.

- [ ] **Step 1: Write the failing tests** in `pkg/arr/maintainerr_rules_test.go`:

```go
package arr

import (
	"context"
	"strings"
	"testing"
)

// The model reads and writes rules as Maintainerr's YAML; the numeric
// encoding must be translated on the way out.
func TestMaintainerrGetRuleRendersRulesAsYAML(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, map[string]string{
		"GET /api/rules/1": `{"id":1,"name":"Movies","collectionId":1,"dataType":"movie",
		  "rules":[{"id":96,"ruleJson":"{\"firstVal\":[6,1],\"action\":4,\"section\":0}","section":0,"isActive":true}],
		  "collection":{"id":1,"arrAction":0,"deleteAfterDays":14}}`,
		"POST /api/rules/yaml/encode": `{"code":1,"result":"mediaType: MOVIES\nrules: []\n"}`,
	})

	detail, err := MaintainerrGetRule(context.Background(), c, 1)
	if err != nil {
		t.Fatalf("MaintainerrGetRule: %v", err)
	}
	if detail.RulesYAML != "mediaType: MOVIES\nrules: []\n" {
		t.Errorf("rulesYaml = %q", detail.RulesYAML)
	}
	if got := strings.Join(*paths, ","); got != "GET /api/rules/1,POST /api/rules/yaml/encode" {
		t.Errorf("requests = %s", got)
	}
	// Maintainerr's encode endpoint takes the rules as a JSON *string*.
	want := `{"rules":"[{\"firstVal\":[6,1],\"action\":4,\"section\":0}]","mediaType":"movie"}`
	if (*bodies)[1] != want {
		t.Errorf("encode body = %s\nwant %s", (*bodies)[1], want)
	}
}

// A group with no conditions has nothing to encode; asking Maintainerr would
// fail the whole read.
func TestMaintainerrGetRuleSkipsEncodingWithoutRules(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/rules/4": `{"id":4,"name":"Manual","dataType":"movie","rules":[],"collection":{"arrAction":4}}`,
	})

	detail, err := MaintainerrGetRule(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("MaintainerrGetRule: %v", err)
	}
	if detail.RulesYAML != "" || len(*paths) != 1 {
		t.Errorf("rulesYaml = %q, requests = %v", detail.RulesYAML, *paths)
	}
}

func TestMaintainerrDecodeRulesReturnsRuleObjects(t *testing.T) {
	c, _, bodies := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/yaml/decode": `{"code":1,"result":"{\"mediaType\":\"movie\",\"rules\":[{\"action\":4,\"section\":0}]}"}`,
	})

	rules, err := maintainerrDecodeRules(context.Background(), c, "rules: []", "movie")
	if err != nil {
		t.Fatalf("maintainerrDecodeRules: %v", err)
	}
	if len(rules) != 1 || string(rules[0]) != `{"action":4,"section":0}` {
		t.Errorf("rules = %s", rules)
	}
	if (*bodies)[0] != `{"yaml":"rules: []","mediaType":"movie"}` {
		t.Errorf("decode body = %s", (*bodies)[0])
	}
}

// A dropped condition widens what a rule group matches, which on a DELETE
// collection means deleting more than was asked for.
func TestMaintainerrDecodeRulesRejectsSkippedRules(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/yaml/decode": `{"code":1,"skipped":2,"result":"{\"rules\":[{\"action\":4}]}"}`,
	})

	_, err := maintainerrDecodeRules(context.Background(), c, "rules: []", "movie")
	if err == nil || !strings.Contains(err.Error(), "could not resolve 2 rule(s)") {
		t.Errorf("error = %v, want skipped rejection", err)
	}
}

func TestMaintainerrDecodeRulesSurfacesParseErrors(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/yaml/decode": `{"code":0,"result":"Unknown property Radarr.nope"}`,
	})

	_, err := maintainerrDecodeRules(context.Background(), c, "bad", "movie")
	if err == nil || !strings.Contains(err.Error(), "Unknown property Radarr.nope") {
		t.Errorf("error = %v, want maintainerr's message", err)
	}
}

func TestMaintainerrDecodeRulesRejectsEmptyRuleSet(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/yaml/decode": `{"code":1,"result":"{\"rules\":[]}"}`,
	})

	if _, err := maintainerrDecodeRules(context.Background(), c, "rules: []", "movie"); err == nil {
		t.Error("an empty rule set was accepted")
	}
}
```

In `pkg/arr/maintainerr_test.go`, `TestMaintainerrRulesDropNotificationSecrets`, add to its routes map:

```go
		"POST /api/rules/yaml/encode": `{"code":1,"result":"rules: []"}`,
```

In `pkg/server/register_maintainerr_test.go` add the imports `io`, `net/http`, `net/http/httptest` and this helper:

```go
// routedArr serves a fixed body per "METHOD /path", recording each request's
// line and body. Maintainerr tools make several upstream calls per tool call,
// which fakeArr's single body cannot answer.
func routedArr(t *testing.T, routes map[string]string) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	var paths, bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		paths = append(paths, key)
		sent, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(sent))
		body, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &paths, &bodies
}
```

Then change `TestMaintainerrGetRuleToolOmitsNotificationWebhooks` to use it. Replace its `fakeArr(...)` line with:

```go
	srv, _, _ := routedArr(t, map[string]string{
		"GET /api/rules/1": `{"id":1,"name":"Movies","collectionId":1,"isActive":true,
		  "dataType":"movie","rules":[{"id":96,"ruleJson":"{\"action\":4}","section":0}],
		  "notifications":[{"agent":"discord","options":{"webhookUrl":"https://discord.com/api/webhooks/1/leaked-token"}}],
		  "collection":{"id":1,"arrAction":0,"deleteAfterDays":14}}`,
		"POST /api/rules/yaml/encode": `{"code":1,"result":"mediaType: MOVIES"}`,
	})
```

and add after the existing `arrAction` assertion:

```go
	if !strings.Contains(body, `"rulesYaml":"mediaType: MOVIES"`) {
		t.Errorf("result does not carry rulesYaml: %s", body)
	}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ ./pkg/server/ -run 'Maintainerr' 2>&1 | head -20`
Expected: build failure `undefined: maintainerrDecodeRules` and `detail.RulesYAML undefined`.

- [ ] **Step 3: Implement.** In `pkg/arr/maintainerr.go`:

Add `Skipped` to the status envelope:

```go
type maintainerrReturnStatus struct {
	Code    int    `json:"code"`
	Result  string `json:"result"`
	Message string `json:"message"`
	// Skipped counts rules a YAML decode could not resolve.
	Skipped int `json:"skipped"`
}
```

Replace `maintainerrCheck` with:

```go
// maintainerrDecodeStatus decodes Maintainerr's result envelope and turns a
// code-0 answer into an error naming the action that was refused.
func maintainerrDecodeStatus(body []byte, action string) (maintainerrReturnStatus, error) {
	var st maintainerrReturnStatus
	if err := unmarshal(body, &st); err != nil {
		return st, err
	}
	if st.Code == 1 {
		return st, nil
	}
	reason := st.Result
	if st.Message != "" {
		reason += ": " + st.Message
	}
	return st, fmt.Errorf("maintainerr refused to %s: %s", action, reason)
}

// maintainerrCheck turns a code-0 envelope into an error.
func maintainerrCheck(body []byte, action string) error {
	_, err := maintainerrDecodeStatus(body, action)
	return err
}
```

Add `RulesYAML` to `MaintainerrRuleDetail`:

```go
	RulesYAML       string                     `json:"rulesYaml,omitempty" jsonschema:"the conditions in Maintainerr's YAML, the format maintainerr_create_rule and maintainerr_update_rule take"`
```

At the end of `MaintainerrGetRule`, before `return out, nil`:

```go
	if len(r.Rules) > 0 {
		rules := make([]json.RawMessage, 0, len(r.Rules))
		for _, rule := range r.Rules {
			rules = append(rules, json.RawMessage(rule.RuleJSON))
		}
		yaml, err := maintainerrEncodeRules(ctx, c, rules, r.DataType)
		if err != nil {
			return MaintainerrRuleDetail{}, fmt.Errorf("rendering rule group %d as YAML: %w", id, err)
		}
		out.RulesYAML = yaml
	}
```

(add `"encoding/json"` to its imports).

Create `pkg/arr/maintainerr_rules.go`:

```go
package arr

import (
	"context"
	"encoding/json"
	"fmt"
)

// Rules cross the MCP boundary only as Maintainerr's YAML. Its encode and
// decode endpoints are pure transforms, so the numeric rule encoding
// (firstVal: [app, property], action indexes) never reaches the model, and
// Maintainerr's own parser is what validates a condition.

// maintainerrEncodeRules renders rule objects as Maintainerr's YAML. The
// endpoint takes the rules as a JSON string, not an array.
func maintainerrEncodeRules(ctx context.Context, c *Client, rules []json.RawMessage, mediaType string) (string, error) {
	encoded, err := json.Marshal(rules)
	if err != nil {
		return "", fmt.Errorf("encoding rules: %w", err)
	}
	body, err := c.Post(ctx, "/rules/yaml/encode", struct {
		Rules     string `json:"rules"`
		MediaType string `json:"mediaType"`
	}{string(encoded), mediaType})
	if err != nil {
		return "", err
	}
	st, err := maintainerrDecodeStatus(body, "render the rules as YAML")
	if err != nil {
		return "", err
	}
	return st.Result, nil
}

// maintainerrDecodeRules parses Maintainerr YAML into rule objects. A rule it
// could not resolve is dropped by Maintainerr, which would widen what the
// group matches, so any skipped rule fails the whole decode.
func maintainerrDecodeRules(ctx context.Context, c *Client, yaml, mediaType string) ([]json.RawMessage, error) {
	body, err := c.Post(ctx, "/rules/yaml/decode", struct {
		YAML      string `json:"yaml"`
		MediaType string `json:"mediaType"`
	}{yaml, mediaType})
	if err != nil {
		return nil, err
	}
	st, err := maintainerrDecodeStatus(body, "read rulesYaml")
	if err != nil {
		return nil, err
	}
	if st.Skipped > 0 {
		return nil, fmt.Errorf("maintainerr could not resolve %d rule(s) in rulesYaml; nothing was saved, "+
			"because dropping a condition widens what the rule group matches", st.Skipped)
	}
	var decoded struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := unmarshal([]byte(st.Result), &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Rules) == 0 {
		return nil, fmt.Errorf("rulesYaml contains no rules")
	}
	return decoded.Rules, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr/maintainerr.go pkg/arr/maintainerr_rules.go pkg/arr/maintainerr_rules_test.go pkg/arr/maintainerr_test.go pkg/server/register_maintainerr_test.go
git commit -m "feat(maintainerr): show rule groups as Maintainerr YAML"
```

---

### Task 2: Lookup tools — libraries, arr servers, rule properties

**Files:**
- Modify: `pkg/arr/maintainerr.go` (extract `maintainerrEnumName`)
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/tools_maintainerr.go`, `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Consumes: `maintainerrRoutes`, `routedArr` (Task 1).
- Produces: `MaintainerrLibrary{ID, Title, Type string}`, `MaintainerrListLibraries(ctx, c) ([]MaintainerrLibrary, error)`; `MaintainerrArrServer{ID int; Kind, ServerName, URL string}`, `MaintainerrListArrServers(ctx, c) ([]MaintainerrArrServer, error)`; `MaintainerrRuleProperty{Name, HumanName, ValueType string; Comparisons, ShowTypes []string}`, `MaintainerrListRuleProperties(ctx, c, application string) ([]MaintainerrRuleProperty, error)`; `maintainerrEnumName(names []string, v int) string`; `maintainerrRulePossibilities []string`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go` (add `"encoding/json"` to its imports):

```go
func TestMaintainerrListLibraries(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/media-server/libraries": `[{"id":"f13","title":"Movies","type":"movie"}]`,
	})

	libs, err := MaintainerrListLibraries(context.Background(), c)
	if err != nil || len(libs) != 1 || libs[0].ID != "f13" || libs[0].Type != "movie" {
		t.Errorf("libraries = %+v, %v", libs, err)
	}
}

// /settings/radarr and /settings/sonarr return each server's apiKey. Only the
// id, name and url may be decoded.
func TestMaintainerrListArrServersNeverCarriesAPIKeys(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/settings/radarr": `[{"id":1,"serverName":"Radarr","url":"http://r:7878","apiKey":"leaked-radarr-key"}]`,
		"GET /api/settings/sonarr": `[{"id":1,"serverName":"Sonarr","url":"http://s:8989","apiKey":"leaked-sonarr-key"},
		                             {"id":2,"serverName":"Sonarr Anime","url":"http://s2:8989","apiKey":"x"}]`,
	})

	servers, err := MaintainerrListArrServers(context.Background(), c)
	if err != nil {
		t.Fatalf("MaintainerrListArrServers: %v", err)
	}
	encoded, _ := json.Marshal(servers)
	if strings.Contains(string(encoded), "leaked") || strings.Contains(string(encoded), "apiKey") {
		t.Fatalf("api keys reached the result: %s", encoded)
	}
	if len(servers) != 3 || servers[0].Kind != "radarr" || servers[2].Kind != "sonarr" || servers[2].ID != 2 {
		t.Errorf("servers = %+v", servers)
	}
}

func TestMaintainerrRulePropertiesNameComparisons(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/rules/constants": `{"applications":[
		  {"id":1,"name":"Radarr","props":[{"id":0,"name":"addDate","humanName":"Date added",
		    "type":{"key":"1","possibilities":[5,6],"humanName":"date"}}]},
		  {"id":2,"name":"Sonarr","props":[{"id":0,"name":"addDate","humanName":"Date added",
		    "type":{"key":"1","possibilities":[5],"humanName":"date"},"showType":["show"]}]}]}`,
	})
	ctx := context.Background()

	all, err := MaintainerrListRuleProperties(ctx, c, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("all = %+v, %v", all, err)
	}
	p := all[0]
	if p.Name != "Radarr.addDate" || p.ValueType != "date" || strings.Join(p.Comparisons, ",") != "BEFORE,AFTER" {
		t.Errorf("property = %+v", p)
	}
	if strings.Join(all[1].ShowTypes, ",") != "show" {
		t.Errorf("showTypes = %v", all[1].ShowTypes)
	}

	sonarr, err := MaintainerrListRuleProperties(ctx, c, "sonarr")
	if err != nil || len(sonarr) != 1 || sonarr[0].Name != "Sonarr.addDate" {
		t.Errorf("filtered = %+v, %v", sonarr, err)
	}
	if _, err := MaintainerrListRuleProperties(ctx, c, "Plexx"); err == nil ||
		!strings.Contains(err.Error(), "Radarr, Sonarr") {
		t.Errorf("unknown application error = %v, want the known names", err)
	}
}
```

In `pkg/server/register_maintainerr_test.go`, add to `maintainerrReadTools`:

```go
	"maintainerr_list_libraries",
	"maintainerr_list_arr_servers",
	"maintainerr_list_rule_properties",
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ ./pkg/server/ -run 'Maintainerr' 2>&1 | head -10`
Expected: `undefined: MaintainerrListLibraries` (arr); after that compiles, the server test fails with `tool "maintainerr_list_libraries" not advertised`.

- [ ] **Step 3: Implement.** In `pkg/arr/maintainerr.go`, replace the body of `maintainerrArrAction` with a shared helper:

```go
// maintainerrEnumName names a Maintainerr enum value by index, keeping unknown
// values visible rather than guessing, since a newer Maintainerr may add some.
func maintainerrEnumName(names []string, v int) string {
	if v >= 0 && v < len(names) {
		return names[v]
	}
	return fmt.Sprintf("UNKNOWN(%d)", v)
}

// maintainerrArrAction names an arrAction value.
func maintainerrArrAction(v int) string { return maintainerrEnumName(maintainerrArrActions, v) }
```

Append to `pkg/arr/maintainerr_rules.go` (add `"strings"` to imports):

```go
// maintainerrRulePossibilities names Maintainerr's RulePossibility enum, the
// comparisons a condition can make.
var maintainerrRulePossibilities = []string{
	"BIGGER", "SMALLER", "EQUALS", "NOT_EQUALS", "CONTAINS", "BEFORE", "AFTER",
	"IN_LAST", "IN_NEXT", "NOT_CONTAINS", "CONTAINS_PARTIAL", "NOT_CONTAINS_PARTIAL",
	"CONTAINS_ALL", "NOT_CONTAINS_ALL", "COUNT_EQUALS", "COUNT_NOT_EQUALS",
	"COUNT_BIGGER", "COUNT_SMALLER", "EXISTS", "NOT_EXISTS",
}

// MaintainerrLibrary is a media server library a rule group can target.
type MaintainerrLibrary struct {
	ID    string `json:"id" jsonschema:"libraryId for maintainerr_create_rule"`
	Title string `json:"title"`
	Type  string `json:"type" jsonschema:"movie or show"`
}

// MaintainerrListLibraries lists the media server's libraries.
func MaintainerrListLibraries(ctx context.Context, c *Client) ([]MaintainerrLibrary, error) {
	return GetJSON[[]MaintainerrLibrary](ctx, c, "/media-server/libraries")
}

// MaintainerrArrServer is a Radarr or Sonarr server Maintainerr can act through.
type MaintainerrArrServer struct {
	ID         int    `json:"id" jsonschema:"arrServerId for maintainerr_create_rule; ids repeat across kinds"`
	Kind       string `json:"kind" jsonschema:"radarr for movie libraries, sonarr for show libraries"`
	ServerName string `json:"serverName"`
	URL        string `json:"url"`
}

// rawMaintainerrArrServer decodes only what may be shown. The endpoint also
// returns the server's apiKey; a field that is never decoded is never re-encoded.
type rawMaintainerrArrServer struct {
	ID         int    `json:"id"`
	ServerName string `json:"serverName"`
	URL        string `json:"url"`
}

// MaintainerrListArrServers lists Maintainerr's Radarr servers, then its Sonarr ones.
func MaintainerrListArrServers(ctx context.Context, c *Client) ([]MaintainerrArrServer, error) {
	var out []MaintainerrArrServer
	for _, kind := range []string{"radarr", "sonarr"} {
		raw, err := GetJSON[[]rawMaintainerrArrServer](ctx, c, "/settings/"+kind)
		if err != nil {
			return nil, err
		}
		for _, r := range raw {
			out = append(out, MaintainerrArrServer{ID: r.ID, Kind: kind, ServerName: r.ServerName, URL: r.URL})
		}
	}
	return out, nil
}

// MaintainerrRuleProperty is one value a rule condition can compare.
type MaintainerrRuleProperty struct {
	Name        string   `json:"name" jsonschema:"identifier for rulesYaml firstValue or lastValue, e.g. Radarr.addDate"`
	HumanName   string   `json:"humanName"`
	ValueType   string   `json:"valueType" jsonschema:"number, date, text, boolean, text list, ..."`
	Comparisons []string `json:"comparisons" jsonschema:"actions allowed with this property"`
	ShowTypes   []string `json:"showTypes,omitempty" jsonschema:"for show libraries, the levels the property applies to"`
}

type rawMaintainerrConstants struct {
	Applications []struct {
		Name  string `json:"name"`
		Props []struct {
			Name      string `json:"name"`
			HumanName string `json:"humanName"`
			Type      struct {
				HumanName     string `json:"humanName"`
				Possibilities []int  `json:"possibilities"`
			} `json:"type"`
			ShowType []string `json:"showType"`
		} `json:"props"`
	} `json:"applications"`
}

// MaintainerrListRuleProperties lists the properties rule conditions can use,
// optionally for one application (case-insensitive), since the full catalog
// runs to hundreds of entries.
func MaintainerrListRuleProperties(ctx context.Context, c *Client, application string) ([]MaintainerrRuleProperty, error) {
	raw, err := GetJSON[rawMaintainerrConstants](ctx, c, "/rules/constants")
	if err != nil {
		return nil, err
	}
	var (
		out   []MaintainerrRuleProperty
		names []string
		found bool
	)
	for _, app := range raw.Applications {
		names = append(names, app.Name)
		if application != "" && !strings.EqualFold(app.Name, application) {
			continue
		}
		found = true
		for _, p := range app.Props {
			comparisons := make([]string, 0, len(p.Type.Possibilities))
			for _, v := range p.Type.Possibilities {
				comparisons = append(comparisons, maintainerrEnumName(maintainerrRulePossibilities, v))
			}
			out = append(out, MaintainerrRuleProperty{
				Name: app.Name + "." + p.Name, HumanName: p.HumanName, ValueType: p.Type.HumanName,
				Comparisons: comparisons, ShowTypes: p.ShowType,
			})
		}
	}
	if !found {
		return nil, fmt.Errorf("unknown application %q; known applications: %s", application, strings.Join(names, ", "))
	}
	return out, nil
}
```

Append to `pkg/server/tools_maintainerr.go`:

```go
// RulePropertiesArgs narrows maintainerr_list_rule_properties.
type RulePropertiesArgs struct {
	InstanceArg
	Application string `json:"application,omitempty" jsonschema:"only this application, e.g. Radarr, Sonarr, Jellyfin, Seerr; omit for all"`
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
```

Add to `registerMaintainerr` in `pkg/server/register_maintainerr.go`, after `maintainerr_media_status`:

```go
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr/maintainerr.go pkg/arr/maintainerr_rules.go pkg/arr/maintainerr_rules_test.go pkg/server/tools_maintainerr.go pkg/server/register_maintainerr.go pkg/server/register_maintainerr_test.go
git commit -m "feat(maintainerr): list libraries, arr servers and rule properties"
```

---

### Task 3: `maintainerr_test_rule` dry-run

**Files:**
- Modify: `pkg/arr/maintainerr.go` (extract `maintainerrValidateMediaID`)
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/tools_maintainerr.go`, `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Produces: `maintainerrValidateMediaID(id string) error`; `MaintainerrRuleTest{RuleGroupID int; MediaServerID string; Matched bool; Sections []MaintainerrSectionCheck}`; `MaintainerrTestRule(ctx, c, ruleGroupID int, mediaServerID string) (MaintainerrRuleTest, error)`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go`:

```go
func TestMaintainerrTestRuleReportsPerConditionResults(t *testing.T) {
	c, _, bodies := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/test": `{"code":1,"result":[{"mediaServerId":"abc","result":true,
		  "sectionResults":[{"id":0,"result":true,"ruleResults":[
		    {"firstValueName":"Radarr.addDate","firstValue":"2026-01-01","action":"before",
		     "secondValueName":"custom_days","secondValue":"2026-06-01","result":true}]}]}]}`,
	})

	res, err := MaintainerrTestRule(context.Background(), c, 1, "abc")
	if err != nil {
		t.Fatalf("MaintainerrTestRule: %v", err)
	}
	if (*bodies)[0] != `{"rulegroupId":1,"mediaId":"abc"}` {
		t.Errorf("body = %s", (*bodies)[0])
	}
	if !res.Matched || len(res.Sections) != 1 || len(res.Sections[0].Rules) != 1 ||
		res.Sections[0].Rules[0].FirstValueName != "Radarr.addDate" || !res.Sections[0].Rules[0].Result {
		t.Errorf("result = %+v", res)
	}
}

// A failed test answers code 0 with a message string where results would be.
func TestMaintainerrTestRuleSurfacesFailures(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, map[string]string{
		"POST /api/rules/test": `{"code":0,"result":"Rule group not found"}`,
	})

	if _, err := MaintainerrTestRule(context.Background(), c, 9, "abc"); err == nil ||
		!strings.Contains(err.Error(), "Rule group not found") {
		t.Errorf("error = %v", err)
	}
}

func TestMaintainerrTestRuleRejectsPathLikeIDs(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{})

	if _, err := MaintainerrTestRule(context.Background(), c, 1, "../settings"); err == nil || len(*paths) != 0 {
		t.Errorf("error = %v, requests = %v", err, *paths)
	}
}
```

Add `"maintainerr_test_rule",` to `maintainerrReadTools` in `pkg/server/register_maintainerr_test.go`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ -run TestRule 2>&1 | head -5`
Expected: `undefined: MaintainerrTestRule`.

- [ ] **Step 3: Implement.** In `pkg/arr/maintainerr.go`, extract the guard from `MaintainerrMediaStatus`:

```go
// maintainerrValidateMediaID rejects ids that would not be a single path
// segment. A slash would let an id walk to another endpoint, such as
// /api/settings, which returns every secret in plaintext.
func maintainerrValidateMediaID(id string) error {
	if strings.Contains(id, "/") || id == "" || id == ".." {
		return fmt.Errorf("invalid mediaServerId %q: want a single media server item id", id)
	}
	return nil
}
```

and make `MaintainerrMediaStatus` begin with:

```go
	if err := maintainerrValidateMediaID(mediaServerID); err != nil {
		return MaintainerrItemStatus{}, err
	}
```

(removing its inline check and comment). Append to `pkg/arr/maintainerr_rules.go`:

```go
// MaintainerrRuleCheck is one condition's outcome in a dry run.
type MaintainerrRuleCheck struct {
	FirstValueName  string `json:"firstValueName"`
	FirstValue      any    `json:"firstValue"`
	Action          string `json:"action"`
	SecondValueName string `json:"secondValueName,omitempty"`
	SecondValue     any    `json:"secondValue,omitempty"`
	Operator        string `json:"operator,omitempty"`
	Result          bool   `json:"result"`
}

// MaintainerrSectionCheck is one section's outcome in a dry run.
type MaintainerrSectionCheck struct {
	ID       int                    `json:"id"`
	Operator string                 `json:"operator,omitempty"`
	Result   bool                   `json:"result"`
	Rules    []MaintainerrRuleCheck `json:"rules"`
}

// MaintainerrRuleTest reports whether a rule group would take one item.
type MaintainerrRuleTest struct {
	RuleGroupID   int                       `json:"ruleGroupId"`
	MediaServerID string                    `json:"mediaServerId"`
	Matched       bool                      `json:"matched" jsonschema:"true when the item would enter the collection"`
	Sections      []MaintainerrSectionCheck `json:"sections"`
}

// MaintainerrTestRule dry-runs a saved rule group against one item. It
// refreshes Maintainerr's caches and queries every service the rules use,
// so it runs on the long timeout.
func MaintainerrTestRule(ctx context.Context, c *Client, ruleGroupID int, mediaServerID string) (MaintainerrRuleTest, error) {
	out := MaintainerrRuleTest{RuleGroupID: ruleGroupID, MediaServerID: mediaServerID}
	if err := maintainerrValidateMediaID(mediaServerID); err != nil {
		return out, err
	}
	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/rules/test", struct {
		RuleGroupID int    `json:"rulegroupId"`
		MediaID     string `json:"mediaId"`
	}{ruleGroupID, mediaServerID})
	if err != nil {
		return out, err
	}
	var raw struct {
		Code   int             `json:"code"`
		Result json.RawMessage `json:"result"`
	}
	if err := unmarshal(body, &raw); err != nil {
		return out, err
	}
	if raw.Code != 1 {
		var msg string
		_ = json.Unmarshal(raw.Result, &msg)
		return out, fmt.Errorf("maintainerr could not test rule group %d: %s", ruleGroupID, msg)
	}
	var stats []struct {
		Result         bool `json:"result"`
		SectionResults []struct {
			ID          int                    `json:"id"`
			Operator    string                 `json:"operator"`
			Result      bool                   `json:"result"`
			RuleResults []MaintainerrRuleCheck `json:"ruleResults"`
		} `json:"sectionResults"`
	}
	if err := unmarshal(raw.Result, &stats); err != nil {
		return out, err
	}
	if len(stats) == 0 {
		return out, fmt.Errorf("maintainerr returned no result for %q", mediaServerID)
	}
	out.Matched = stats[0].Result
	for _, s := range stats[0].SectionResults {
		out.Sections = append(out.Sections, MaintainerrSectionCheck{
			ID: s.ID, Operator: s.Operator, Result: s.Result, Rules: s.RuleResults,
		})
	}
	return out, nil
}
```

Append to `pkg/server/tools_maintainerr.go`:

```go
// TestRuleArgs dry-runs a rule group against one item.
type TestRuleArgs struct {
	InstanceArg
	RuleGroupID   int    `json:"ruleGroupId" jsonschema:"rule group id from maintainerr_list_rules"`
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id to test, e.g. from maintainerr_collection_media"`
}
```

Register in `pkg/server/register_maintainerr.go` after `maintainerr_list_rule_properties`:

```go
	register(s, svc, spec, toolMeta{
		name: "maintainerr_test_rule",
		description: "Dry-run a saved Maintainerr rule group against one media item: whether it would " +
			"enter the collection, and each condition's values and result. Changes nothing.",
		access: AccessRead,
	}, func(ctx context.Context, c *arr.Client, in TestRuleArgs) (arr.MaintainerrRuleTest, error) {
		return arr.MaintainerrTestRule(ctx, c, in.RuleGroupID, in.MediaServerID)
	})
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both. If `TestMaintainerrToolsAreAllAdvertised` reports `maintainerr_test_rule` missing, the `any` fields broke schema generation (the server logs `building schema`); change `FirstValue`/`SecondValue` to `string` and format with `fmt.Sprint` while decoding.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr pkg/server
git commit -m "feat(maintainerr): dry-run a rule group against one item"
```

---

### Task 4: `maintainerr_create_rule`

**Files:**
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/tools_maintainerr.go`, `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Consumes: `maintainerrDecodeRules` (Task 1), `MaintainerrListLibraries`, `MaintainerrListArrServers` (Task 2), `MaintainerrListRules`, `MaintainerrGetRule` (existing).
- Produces: `MaintainerrNewRule{Name, Description, LibraryID, ArrAction string; DeleteAfterDays, ArrServerID int; RulesYAML string; OverlayEnabled bool}`; `MaintainerrCreateRule(ctx, c, in MaintainerrNewRule) (MaintainerrRuleDetail, error)`; `maintainerrArrActionIndex(name string) (int, error)`; `maintainerrDeleteAfterMaxDays = 36500`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go`:

```go
// createRoutes answers every call a successful create makes. The rule list
// holds an older group with the same name, which the create must not pick.
func createRoutes() map[string]string {
	return map[string]string{
		"GET /api/media-server/libraries": `[{"id":"lib-m","title":"Movies","type":"movie"},{"id":"lib-s","title":"Shows","type":"show"}]`,
		"GET /api/settings/radarr":        `[{"id":1,"serverName":"Radarr","url":"http://r","apiKey":"k"}]`,
		"GET /api/settings/sonarr":        `[{"id":1,"serverName":"Sonarr","url":"http://s","apiKey":"k"},{"id":2,"serverName":"Anime","url":"http://s2","apiKey":"k"}]`,
		"POST /api/rules/yaml/decode":     `{"code":1,"result":"{\"rules\":[{\"action\":5,\"section\":0}]}"}`,
		"POST /api/rules":                 `{"code":1,"result":"Success"}`,
		"GET /api/rules": `[{"id":3,"name":"Old unwatched","libraryId":"lib-m","collectionId":3},
		                    {"id":8,"name":"Old unwatched","libraryId":"lib-m","collectionId":8}]`,
		"GET /api/rules/8": `{"id":8,"name":"Old unwatched","libraryId":"lib-m","collectionId":8,"dataType":"movie",
		  "rules":[{"id":1,"ruleJson":"{\"action\":5}","section":0}],"collection":{"arrAction":0,"deleteAfterDays":30}}`,
		"POST /api/rules/yaml/encode": `{"code":1,"result":"rules: []"}`,
	}
}

func validNewRule() MaintainerrNewRule {
	return MaintainerrNewRule{
		Name: "Old unwatched", LibraryID: "lib-m", ArrAction: "delete",
		DeleteAfterDays: 30, ArrServerID: 1, RulesYAML: "rules: []",
	}
}

func TestMaintainerrCreateRuleSendsAnExplicitActiveGroup(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, createRoutes())

	detail, err := MaintainerrCreateRule(context.Background(), c, validNewRule())
	if err != nil {
		t.Fatalf("MaintainerrCreateRule: %v", err)
	}
	// Two groups share the name; the newest is the one just created.
	if detail.ID != 8 {
		t.Errorf("returned group %d, want 8", detail.ID)
	}
	var sent map[string]any
	for i, p := range *paths {
		if p == "POST /api/rules" {
			_ = json.Unmarshal([]byte((*bodies)[i]), &sent)
		}
	}
	if sent == nil {
		t.Fatalf("no POST /api/rules in %v", *paths)
	}
	// useRules must be explicit: Maintainerr saves rules only when it is true.
	for k, want := range map[string]any{
		"name": "Old unwatched", "libraryId": "lib-m", "dataType": "movie",
		"isActive": true, "useRules": true, "arrAction": float64(0), "radarrSettingsId": float64(1),
	} {
		if sent[k] != want {
			t.Errorf("%s = %v, want %v", k, sent[k], want)
		}
	}
	if _, ok := sent["sonarrSettingsId"]; ok {
		t.Error("a movie group was sent a sonarrSettingsId")
	}
	col, _ := sent["collection"].(map[string]any)
	if col["deleteAfterDays"] != float64(30) {
		t.Errorf("collection = %v", col)
	}
}

func TestMaintainerrCreateRuleMapsShowServersToSonarr(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, createRoutes())
	in := validNewRule()
	in.LibraryID, in.ArrServerID = "lib-s", 2

	_, _ = MaintainerrCreateRule(context.Background(), c, in)
	for i, p := range *paths {
		if p == "POST /api/rules" && !strings.Contains((*bodies)[i], `"sonarrSettingsId":2`) {
			t.Errorf("body = %s, want sonarrSettingsId 2", (*bodies)[i])
		}
	}
}

// Radarr and Sonarr ids overlap, so an id is only valid for the kind the
// library needs. Nothing may be posted when it is wrong.
func TestMaintainerrCreateRuleRejectsAServerOfTheWrongKind(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, createRoutes())
	in := validNewRule()
	in.ArrServerID = 2 // exists only as a Sonarr id

	_, err := MaintainerrCreateRule(context.Background(), c, in)
	if err == nil || !strings.Contains(err.Error(), "no radarr server with id 2") {
		t.Errorf("error = %v", err)
	}
	for _, p := range *paths {
		if p == "POST /api/rules" {
			t.Fatal("rule group was created with an invalid server")
		}
	}
}

func TestMaintainerrCreateRuleValidatesBeforeCallingMaintainerr(t *testing.T) {
	cases := map[string]func(*MaintainerrNewRule){
		"grace period of 0 acts on the next run": func(r *MaintainerrNewRule) { r.DeleteAfterDays = 0 },
		"grace period over the maximum":          func(r *MaintainerrNewRule) { r.DeleteAfterDays = 36501 },
		"unknown arrAction":                      func(r *MaintainerrNewRule) { r.ArrAction = "PURGE" },
		"missing arrAction":                      func(r *MaintainerrNewRule) { r.ArrAction = "" },
		"missing server for a real action":       func(r *MaintainerrNewRule) { r.ArrServerID = 0 },
		"missing name":                           func(r *MaintainerrNewRule) { r.Name = " " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c, paths, _ := maintainerrRoutes(t, createRoutes())
			in := validNewRule()
			mutate(&in)
			if _, err := MaintainerrCreateRule(context.Background(), c, in); err == nil {
				t.Fatal("accepted")
			}
			if len(*paths) != 0 {
				t.Errorf("called maintainerr: %v", *paths)
			}
		})
	}
}

// DO_NOTHING never reaches an *arr, so it needs no server.
func TestMaintainerrCreateRuleAllowsDoNothingWithoutAServer(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, createRoutes())
	in := validNewRule()
	in.ArrAction, in.ArrServerID = "DO_NOTHING", 0

	if _, err := MaintainerrCreateRule(context.Background(), c, in); err != nil {
		t.Errorf("MaintainerrCreateRule: %v", err)
	}
}

func TestMaintainerrCreateRuleNamesUnknownLibraries(t *testing.T) {
	c, _, _ := maintainerrRoutes(t, createRoutes())
	in := validNewRule()
	in.LibraryID = "nope"

	_, err := MaintainerrCreateRule(context.Background(), c, in)
	if err == nil || !strings.Contains(err.Error(), "lib-m (Movies)") {
		t.Errorf("error = %v, want the valid libraries listed", err)
	}
}

func TestMaintainerrCreateRuleSurfacesARefusal(t *testing.T) {
	routes := createRoutes()
	routes["POST /api/rules"] = `{"code":0,"result":"Operator is required for every rule after the first"}`
	c, _, _ := maintainerrRoutes(t, routes)

	_, err := MaintainerrCreateRule(context.Background(), c, validNewRule())
	if err == nil || !strings.Contains(err.Error(), "Operator is required") {
		t.Errorf("error = %v", err)
	}
}
```

Add `"maintainerr_create_rule",` to `maintainerrWriteTools`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ -run CreateRule 2>&1 | head -5`
Expected: `undefined: MaintainerrNewRule`.

- [ ] **Step 3: Implement.** Append to `pkg/arr/maintainerr_rules.go`:

```go
// maintainerrDeleteAfterMaxDays is Maintainerr's DELETE_AFTER_MAX_DAYS.
const maintainerrDeleteAfterMaxDays = 36500

// maintainerrArrActionIndex turns an arrAction name into Maintainerr's enum
// value. The name is required: an omitted action is DELETE upstream.
func maintainerrArrActionIndex(name string) (int, error) {
	for i, n := range maintainerrArrActions {
		if strings.EqualFold(n, name) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown arrAction %q; want one of: %s", name, strings.Join(maintainerrArrActions, ", "))
}

// MaintainerrNewRule describes a rule group to create.
type MaintainerrNewRule struct {
	Name            string
	Description     string
	LibraryID       string
	ArrAction       string
	DeleteAfterDays int
	ArrServerID     int
	RulesYAML       string
	OverlayEnabled  bool
}

// MaintainerrCreateRule creates an active rule group and its collection.
// Validation that needs no network runs first, so a bad request sends
// nothing. The grace period must be at least a day: this is a write-tier
// call, and an immediate action belongs to maintainerr_set_deletion_policy,
// which asks first.
func MaintainerrCreateRule(ctx context.Context, c *Client, in MaintainerrNewRule) (MaintainerrRuleDetail, error) {
	if strings.TrimSpace(in.Name) == "" {
		return MaintainerrRuleDetail{}, fmt.Errorf("name is required")
	}
	action, err := maintainerrArrActionIndex(in.ArrAction)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	if in.DeleteAfterDays < 1 || in.DeleteAfterDays > maintainerrDeleteAfterMaxDays {
		return MaintainerrRuleDetail{}, fmt.Errorf("deleteAfterDays must be 1 to %d; to act sooner, create the "+
			"group and then use maintainerr_set_deletion_policy", maintainerrDeleteAfterMaxDays)
	}
	doNothing := maintainerrArrActions[action] == "DO_NOTHING"
	if in.ArrServerID == 0 && !doNothing {
		return MaintainerrRuleDetail{}, fmt.Errorf("arrServerId is required unless arrAction is DO_NOTHING; " +
			"list them with maintainerr_list_arr_servers")
	}

	libs, err := MaintainerrListLibraries(ctx, c)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	var lib *MaintainerrLibrary
	valid := make([]string, 0, len(libs))
	for i := range libs {
		valid = append(valid, libs[i].ID+" ("+libs[i].Title+")")
		if libs[i].ID == in.LibraryID {
			lib = &libs[i]
		}
	}
	if lib == nil {
		return MaintainerrRuleDetail{}, fmt.Errorf("unknown libraryId %q; libraries: %s", in.LibraryID, strings.Join(valid, ", "))
	}

	serverKind, serverField := "radarr", "radarrSettingsId"
	if lib.Type != "movie" {
		serverKind, serverField = "sonarr", "sonarrSettingsId"
	}
	if in.ArrServerID != 0 {
		if err := maintainerrCheckArrServer(ctx, c, serverKind, in.ArrServerID); err != nil {
			return MaintainerrRuleDetail{}, err
		}
	}

	rules, err := maintainerrDecodeRules(ctx, c, in.RulesYAML, lib.Type)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	group := map[string]any{
		"libraryId": lib.ID, "name": in.Name, "description": in.Description, "dataType": lib.Type,
		// useRules must be sent: Maintainerr saves the rules only when it is true.
		"isActive": true, "useRules": true, "arrAction": action, "rules": rules,
		"collection": map[string]any{"deleteAfterDays": in.DeleteAfterDays, "overlayEnabled": in.OverlayEnabled},
	}
	if in.ArrServerID != 0 {
		group[serverField] = in.ArrServerID
	}
	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/rules", group)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	if err := maintainerrCheck(body, "create the rule group"); err != nil {
		return MaintainerrRuleDetail{}, err
	}
	id, err := maintainerrFindRule(ctx, c, in.Name, lib.ID)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	return MaintainerrGetRule(ctx, c, id)
}

// maintainerrCheckArrServer confirms a server id exists for the kind a
// library needs. Radarr and Sonarr ids overlap, so an id alone proves nothing.
func maintainerrCheckArrServer(ctx context.Context, c *Client, kind string, id int) error {
	servers, err := MaintainerrListArrServers(ctx, c)
	if err != nil {
		return err
	}
	var valid []string
	for _, s := range servers {
		if s.Kind != kind {
			continue
		}
		if s.ID == id {
			return nil
		}
		valid = append(valid, fmt.Sprintf("%d (%s)", s.ID, s.ServerName))
	}
	return fmt.Errorf("no %s server with id %d; %s servers: %s", kind, id, kind, strings.Join(valid, ", "))
}

// maintainerrFindRule finds the group a create just made. Maintainerr answers
// a create without its id, and names need not be unique, so the newest group
// with this name on this library is taken.
func maintainerrFindRule(ctx context.Context, c *Client, name, libraryID string) (int, error) {
	rules, err := MaintainerrListRules(ctx, c)
	if err != nil {
		return 0, err
	}
	id := 0
	for _, r := range rules {
		if r.Name == name && r.LibraryID == libraryID && r.ID > id {
			id = r.ID
		}
	}
	if id == 0 {
		return 0, fmt.Errorf("maintainerr reported success but rule group %q was not found", name)
	}
	return id, nil
}
```

Append to `pkg/server/tools_maintainerr.go`:

```go
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
```

Register after `maintainerr_test_rule`:

```go
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr pkg/server
git commit -m "feat(maintainerr): create rule groups from YAML"
```

---

### Task 5: Read-modify-write updates — `update_rule`, `update_collection`, `set_deletion_policy`

**Files:**
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/tools_maintainerr.go`, `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Consumes: `maintainerrDecodeRules` (Task 1), `maintainerrArrActionIndex`, `maintainerrDeleteAfterMaxDays` (Task 4), `maintainerrGetOne`, `MaintainerrGetRule` (existing).
- Produces: `MaintainerrRulePatch{Name, Description, RulesYAML, RuleHandlerCronSchedule *string}`, `MaintainerrUpdateRule(ctx, c, id int, p MaintainerrRulePatch) (MaintainerrRuleDetail, error)`; `MaintainerrCollectionPatch{OverlayEnabled, VisibleOnHome, VisibleOnRecommended *bool}`, `MaintainerrUpdateCollection(ctx, c, collectionID int, p MaintainerrCollectionPatch) (MaintainerrRuleDetail, error)`; `MaintainerrDeletionPolicy{ArrAction *string; DeleteAfterDays *int; IsActive *bool}`, `MaintainerrSetDeletionPolicy(ctx, c, collectionID int, p MaintainerrDeletionPolicy) (MaintainerrRuleDetail, error)`; `maintainerrRuleGroupForCollection(ctx, c, collectionID int) (int, error)`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go`:

```go
// storedGroup is a rule group as GET /api/rules/{id} returns it: the settings
// PUT reads from the top level (arrAction, the *arr links) sit in collection,
// and every rule is a row whose ruleJson is a string.
const storedGroup = `{"id":2,"name":"Shows","description":"d","libraryId":"lib-s","collectionId":5,
  "isActive":true,"dataType":"show","useRules":true,"ruleHandlerCronSchedule":null,
  "rules":[{"id":103,"ruleJson":"{\"action\":2,\"section\":0}","ruleGroupId":2,"section":0,"isActive":true}],
  "notifications":[{"id":1,"agent":"discord","options":{"webhookUrl":"https://x/keep-me"}}],
  "collection":{"id":5,"arrAction":3,"deleteAfterDays":14,"isActive":true,"overlayEnabled":true,
    "sortTitle":"Zz","keepLogsForMonths":12,"sonarrSettingsId":2,"tagInArr":true,"listExclusions":true,
    "visibleOnHome":true}}`

func updateRoutes() map[string]string {
	return map[string]string{
		"GET /api/rules/collection/5":  `{"id":2,"collectionId":5,"name":"Shows"}`,
		"GET /api/rules/2":             storedGroup,
		"PUT /api/rules":               `{"code":1,"result":"Success"}`,
		"POST /api/rules/yaml/decode":  `{"code":1,"result":"{\"rules\":[{\"action\":9,\"section\":0}]}"}`,
		"POST /api/rules/yaml/encode":  `{"code":1,"result":"rules: []"}`,
	}
}

// sentPut returns the body of the one PUT /api/rules, decoded.
func sentPut(t *testing.T, paths, bodies []string) map[string]any {
	t.Helper()
	for i, p := range paths {
		if p == "PUT /api/rules" {
			var m map[string]any
			if err := json.Unmarshal([]byte(bodies[i]), &m); err != nil {
				t.Fatalf("PUT body: %v", err)
			}
			return m
		}
	}
	t.Fatalf("no PUT /api/rules in %v", paths)
	return nil
}

// PUT /api/rules resets what it is not sent: arrAction to DELETE, the Sonarr
// link to null, overlays off, log retention to 6 months. A rename must send
// all of it back as it was.
func TestMaintainerrUpdateRulePreservesEverythingItDoesNotChange(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, updateRoutes())
	name := "TV"

	if _, err := MaintainerrUpdateRule(context.Background(), c, 2, MaintainerrRulePatch{Name: &name}); err != nil {
		t.Fatalf("MaintainerrUpdateRule: %v", err)
	}
	sent := sentPut(t, *paths, *bodies)
	for k, want := range map[string]any{
		"name": "TV", "description": "d", "arrAction": float64(3), "sonarrSettingsId": float64(2),
		"tagInArr": true, "listExclusions": true, "isActive": true, "useRules": true,
	} {
		if sent[k] != want {
			t.Errorf("%s = %v, want %v", k, sent[k], want)
		}
	}
	col := sent["collection"].(map[string]any)
	if col["overlayEnabled"] != true || col["sortTitle"] != "Zz" || col["keepLogsForMonths"] != float64(12) {
		t.Errorf("collection = %v", col)
	}
	if !strings.Contains(fmtAny(sent["notifications"]), "keep-me") {
		t.Error("notifications were not sent back; Maintainerr would drop them")
	}
	// Stored rules go back as rule objects, not as rows with a ruleJson string.
	rules := sent["rules"].([]any)
	if len(rules) != 1 || rules[0].(map[string]any)["action"] != float64(2) {
		t.Errorf("rules = %v", rules)
	}
}

func fmtAny(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestMaintainerrUpdateRuleReplacesRulesFromYAML(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, updateRoutes())
	yaml := "rules: []"

	if _, err := MaintainerrUpdateRule(context.Background(), c, 2, MaintainerrRulePatch{RulesYAML: &yaml}); err != nil {
		t.Fatalf("MaintainerrUpdateRule: %v", err)
	}
	for i, p := range *paths {
		if p == "POST /api/rules/yaml/decode" && !strings.Contains((*bodies)[i], `"mediaType":"show"`) {
			t.Errorf("decode body = %s, want the group's dataType", (*bodies)[i])
		}
	}
	rules := sentPut(t, *paths, *bodies)["rules"].([]any)
	if len(rules) != 1 || rules[0].(map[string]any)["action"] != float64(9) {
		t.Errorf("rules = %v", rules)
	}
}

// A no-op PUT still resets the fields Maintainerr does not fall back on.
func TestMaintainerrUpdatesWithNothingToChangeSendNothing(t *testing.T) {
	ctx := context.Background()
	c, paths, _ := maintainerrRoutes(t, updateRoutes())

	if _, err := MaintainerrUpdateRule(ctx, c, 2, MaintainerrRulePatch{}); err == nil {
		t.Error("empty rule patch accepted")
	}
	if _, err := MaintainerrUpdateCollection(ctx, c, 5, MaintainerrCollectionPatch{}); err == nil {
		t.Error("empty collection patch accepted")
	}
	if _, err := MaintainerrSetDeletionPolicy(ctx, c, 5, MaintainerrDeletionPolicy{}); err == nil {
		t.Error("empty policy accepted")
	}
	if len(*paths) != 0 {
		t.Errorf("requests sent: %v", *paths)
	}
}

// A stored rule that is not a JSON string cannot be sent back as a rule
// object; writing anyway would corrupt the group.
func TestMaintainerrUpdateRefusesUnreadableStoredRules(t *testing.T) {
	routes := updateRoutes()
	routes["GET /api/rules/2"] = strings.Replace(storedGroup,
		`"ruleJson":"{\"action\":2,\"section\":0}"`, `"ruleJson":{"action":2}`, 1)
	c, paths, _ := maintainerrRoutes(t, routes)
	name := "TV"

	if _, err := MaintainerrUpdateRule(context.Background(), c, 2, MaintainerrRulePatch{Name: &name}); err == nil {
		t.Error("update proceeded with an unreadable stored rule")
	}
	for _, p := range *paths {
		if p == "PUT /api/rules" {
			t.Fatal("PUT sent")
		}
	}
}

func TestMaintainerrUpdateCollectionResolvesTheRuleGroup(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, updateRoutes())
	off := false

	if _, err := MaintainerrUpdateCollection(context.Background(), c, 5, MaintainerrCollectionPatch{OverlayEnabled: &off}); err != nil {
		t.Fatalf("MaintainerrUpdateCollection: %v", err)
	}
	col := sentPut(t, *paths, *bodies)["collection"].(map[string]any)
	if col["overlayEnabled"] != false || col["visibleOnHome"] != true {
		t.Errorf("collection = %v", col)
	}
}

// arrAction lives in two places; PUT reads the top-level one, so both change.
func TestMaintainerrSetDeletionPolicyWritesTopLevelAndCollection(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, updateRoutes())
	action, days, active := "DELETE", 0, false

	_, err := MaintainerrSetDeletionPolicy(context.Background(), c, 5, MaintainerrDeletionPolicy{
		ArrAction: &action, DeleteAfterDays: &days, IsActive: &active,
	})
	if err != nil {
		t.Fatalf("MaintainerrSetDeletionPolicy: %v", err)
	}
	sent := sentPut(t, *paths, *bodies)
	col := sent["collection"].(map[string]any)
	if sent["arrAction"] != float64(0) || col["arrAction"] != float64(0) ||
		col["deleteAfterDays"] != float64(0) || sent["isActive"] != false || col["isActive"] != false {
		t.Errorf("sent = %v", sent)
	}
}

func TestMaintainerrSetDeletionPolicyValidates(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, updateRoutes())
	bad, over := "PURGE", 36501

	if _, err := MaintainerrSetDeletionPolicy(context.Background(), c, 5, MaintainerrDeletionPolicy{ArrAction: &bad}); err == nil {
		t.Error("unknown action accepted")
	}
	if _, err := MaintainerrSetDeletionPolicy(context.Background(), c, 5, MaintainerrDeletionPolicy{DeleteAfterDays: &over}); err == nil {
		t.Error("days over the maximum accepted")
	}
	if len(*paths) != 0 {
		t.Errorf("requests sent: %v", *paths)
	}
}

func TestMaintainerrUpdateCollectionNamesUnknownCollections(t *testing.T) {
	routes := updateRoutes()
	routes["GET /api/rules/collection/5"] = ``
	c, _, _ := maintainerrRoutes(t, routes)
	on := true

	_, err := MaintainerrUpdateCollection(context.Background(), c, 5, MaintainerrCollectionPatch{OverlayEnabled: &on})
	if err == nil || !strings.Contains(err.Error(), "no rule group for collection with id 5") {
		t.Errorf("error = %v", err)
	}
}
```

Add `"maintainerr_update_rule",` and `"maintainerr_update_collection",` to `maintainerrWriteTools`, and `"maintainerr_set_deletion_policy",` to `maintainerrDestructiveTools`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ -run 'Update|DeletionPolicy' 2>&1 | head -5`
Expected: `undefined: MaintainerrUpdateRule`.

- [ ] **Step 3: Implement.** Append to `pkg/arr/maintainerr_rules.go`:

```go
// maintainerrHoisted are the settings PUT /api/rules reads from the top level
// of its body, while GET /api/rules/{id} returns them inside collection. Left
// unsent, Maintainerr resets them: arrAction to DELETE, the *arr links to null.
var maintainerrHoisted = []string{
	"arrAction", "listExclusions", "cleanupLeftoverFolders", "forceSeerr",
	"tautulliWatchedPercentOverride", "radarrSettingsId", "sonarrSettingsId",
	"sportarrSettingsId", "radarrQualityProfileId", "sonarrQualityProfileId",
	"sportarrQualityProfileId", "tagInArr", "keepInMaintainerrOnly",
}

// maintainerrStoredRules turns stored rule rows back into rule objects. PUT
// saves each rule as JSON.stringify(rule), so sending the rows would store the
// rows themselves as rules.
func maintainerrStoredRules(v any) ([]json.RawMessage, error) {
	rows, _ := v.([]any)
	out := make([]json.RawMessage, 0, len(rows))
	for i, row := range rows {
		m, _ := row.(map[string]any)
		s, ok := m["ruleJson"].(string)
		if !ok || !json.Valid([]byte(s)) {
			return nil, fmt.Errorf("stored rule %d is not readable; nothing was changed", i)
		}
		out = append(out, json.RawMessage(s))
	}
	return out, nil
}

// maintainerrUpdateRule reads a rule group whole, lets edit change it, and
// writes it back whole. Maintainerr has no partial update, and PUT resets
// several fields it is not sent, so nothing here may send less than it read.
func maintainerrUpdateRule(ctx context.Context, c *Client, id int, edit func(group, collection map[string]any) error) (MaintainerrRuleDetail, error) {
	group, err := maintainerrGetOne[map[string]any](ctx, c, "/rules/"+itoa(id), "rule group", id)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	collection, ok := group["collection"].(map[string]any)
	if !ok {
		return MaintainerrRuleDetail{}, fmt.Errorf("rule group %d has no collection", id)
	}
	for _, k := range maintainerrHoisted {
		if v, ok := collection[k]; ok {
			group[k] = v
		}
	}
	rules, err := maintainerrStoredRules(group["rules"])
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	group["rules"] = rules
	if err := edit(group, collection); err != nil {
		return MaintainerrRuleDetail{}, err
	}
	body, err := c.WithTimeout(maintainerrSlowTimeout).Put(ctx, "/rules", group)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	if err := maintainerrCheck(body, "update the rule group"); err != nil {
		return MaintainerrRuleDetail{}, err
	}
	return MaintainerrGetRule(ctx, c, id)
}

// maintainerrRuleGroupForCollection finds the rule group that owns a
// collection. Collection and rule group ids need not match.
func maintainerrRuleGroupForCollection(ctx context.Context, c *Client, collectionID int) (int, error) {
	g, err := maintainerrGetOne[struct {
		ID int `json:"id"`
	}](ctx, c, "/rules/collection/"+itoa(collectionID), "rule group for collection", collectionID)
	return g.ID, err
}

// MaintainerrRulePatch lists rule group fields to change; nil leaves one alone.
type MaintainerrRulePatch struct {
	Name                    *string
	Description             *string
	RulesYAML               *string
	RuleHandlerCronSchedule *string
}

// MaintainerrUpdateRule changes a rule group's name, description, conditions
// or schedule. An empty schedule returns the group to the global one.
func MaintainerrUpdateRule(ctx context.Context, c *Client, id int, p MaintainerrRulePatch) (MaintainerrRuleDetail, error) {
	if p == (MaintainerrRulePatch{}) {
		return MaintainerrRuleDetail{}, fmt.Errorf("nothing to change: set name, description, rulesYaml or ruleHandlerCronSchedule")
	}
	return maintainerrUpdateRule(ctx, c, id, func(group, _ map[string]any) error {
		if p.Name != nil {
			group["name"] = *p.Name
		}
		if p.Description != nil {
			group["description"] = *p.Description
		}
		if p.RuleHandlerCronSchedule != nil {
			if *p.RuleHandlerCronSchedule == "" {
				group["ruleHandlerCronSchedule"] = nil
			} else {
				group["ruleHandlerCronSchedule"] = *p.RuleHandlerCronSchedule
			}
		}
		if p.RulesYAML != nil {
			dataType, _ := group["dataType"].(string)
			rules, err := maintainerrDecodeRules(ctx, c, *p.RulesYAML, dataType)
			if err != nil {
				return err
			}
			group["rules"] = rules
		}
		return nil
	})
}

// MaintainerrCollectionPatch lists collection display settings to change.
type MaintainerrCollectionPatch struct {
	OverlayEnabled       *bool
	VisibleOnHome        *bool
	VisibleOnRecommended *bool
}

// MaintainerrUpdateCollection changes settings that do not affect deletion.
func MaintainerrUpdateCollection(ctx context.Context, c *Client, collectionID int, p MaintainerrCollectionPatch) (MaintainerrRuleDetail, error) {
	if p == (MaintainerrCollectionPatch{}) {
		return MaintainerrRuleDetail{}, fmt.Errorf("nothing to change: set overlayEnabled, visibleOnHome or visibleOnRecommended")
	}
	id, err := maintainerrRuleGroupForCollection(ctx, c, collectionID)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	return maintainerrUpdateRule(ctx, c, id, func(_, col map[string]any) error {
		for k, v := range map[string]*bool{
			"overlayEnabled": p.OverlayEnabled, "visibleOnHome": p.VisibleOnHome,
			"visibleOnRecommended": p.VisibleOnRecommended,
		} {
			if v != nil {
				col[k] = *v
			}
		}
		return nil
	})
}

// MaintainerrDeletionPolicy lists the settings that decide when and whether a
// collection acts; nil leaves one alone.
type MaintainerrDeletionPolicy struct {
	ArrAction       *string
	DeleteAfterDays *int
	IsActive        *bool
}

// MaintainerrSetDeletionPolicy changes what a collection does to its items,
// after how long, and whether it runs at all. arrAction and isActive exist at
// both levels of the group, and PUT reads the top-level copies, so both change.
func MaintainerrSetDeletionPolicy(ctx context.Context, c *Client, collectionID int, p MaintainerrDeletionPolicy) (MaintainerrRuleDetail, error) {
	if p == (MaintainerrDeletionPolicy{}) {
		return MaintainerrRuleDetail{}, fmt.Errorf("nothing to change: set arrAction, deleteAfterDays or isActive")
	}
	action := -1
	if p.ArrAction != nil {
		a, err := maintainerrArrActionIndex(*p.ArrAction)
		if err != nil {
			return MaintainerrRuleDetail{}, err
		}
		action = a
	}
	if p.DeleteAfterDays != nil && (*p.DeleteAfterDays < 0 || *p.DeleteAfterDays > maintainerrDeleteAfterMaxDays) {
		return MaintainerrRuleDetail{}, fmt.Errorf("deleteAfterDays must be 0 to %d", maintainerrDeleteAfterMaxDays)
	}
	id, err := maintainerrRuleGroupForCollection(ctx, c, collectionID)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	return maintainerrUpdateRule(ctx, c, id, func(group, col map[string]any) error {
		if action >= 0 {
			group["arrAction"], col["arrAction"] = action, action
		}
		if p.DeleteAfterDays != nil {
			col["deleteAfterDays"] = *p.DeleteAfterDays
		}
		if p.IsActive != nil {
			group["isActive"], col["isActive"] = *p.IsActive, *p.IsActive
		}
		return nil
	})
}
```

Append to `pkg/server/tools_maintainerr.go`:

```go
// UpdateRuleArgs changes a rule group; omitted fields are left alone.
type UpdateRuleArgs struct {
	InstanceArg
	ID                      int     `json:"id" jsonschema:"rule group id from maintainerr_list_rules"`
	Name                    *string `json:"name,omitempty"`
	Description             *string `json:"description,omitempty"`
	RulesYAML               *string `json:"rulesYaml,omitempty" jsonschema:"replaces every condition; start from maintainerr_get_rule's rulesYaml"`
	RuleHandlerCronSchedule *string `json:"ruleHandlerCronSchedule,omitempty" jsonschema:"cron expression; empty string returns to the global schedule"`
}

// UpdateCollectionArgs changes collection display settings.
type UpdateCollectionArgs struct {
	InstanceArg
	CollectionID         int   `json:"collectionId" jsonschema:"collection id from maintainerr_list_collections"`
	OverlayEnabled       *bool `json:"overlayEnabled,omitempty" jsonschema:"show the deletion date on posters"`
	VisibleOnHome        *bool `json:"visibleOnHome,omitempty"`
	VisibleOnRecommended *bool `json:"visibleOnRecommended,omitempty"`
}

// DeletionPolicyArgs changes when and whether a collection acts.
type DeletionPolicyArgs struct {
	InstanceArg
	CollectionID    int     `json:"collectionId" jsonschema:"collection id from maintainerr_list_collections"`
	ArrAction       *string `json:"arrAction,omitempty" jsonschema:"DELETE, UNMONITOR_DELETE_ALL, UNMONITOR_DELETE_EXISTING, UNMONITOR, DO_NOTHING, DELETE_SHOW_IF_EMPTY, UNMONITOR_SHOW_IF_EMPTY or CHANGE_QUALITY_PROFILE"`
	DeleteAfterDays *int    `json:"deleteAfterDays,omitempty" jsonschema:"grace period in days, 0 to 36500; 0 acts on the next collection run"`
	IsActive        *bool   `json:"isActive,omitempty" jsonschema:"false stops the rule group and its collection entirely"`
}
```

Register after `maintainerr_create_rule`:

```go
	register(s, svc, spec, toolMeta{
		name: "maintainerr_update_rule",
		description: "Change a Maintainerr rule group's name, description, conditions (rulesYaml replaces " +
			"all of them) or schedule. Every other setting is kept. To change the action, grace period or " +
			"active state, use maintainerr_set_deletion_policy.",
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
	}, func(ctx context.Context, c *arr.Client, in UpdateCollectionArgs) (arr.MaintainerrRuleDetail, error) {
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr pkg/server
git commit -m "feat(maintainerr): edit rule groups and collection settings"
```

---

### Task 6: `maintainerr_delete_rule`

**Files:**
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Consumes: `maintainerrCheck` (Task 1), `Deleted` and `IDArgs` (existing, `pkg/server/tools.go`).
- Produces: `MaintainerrDeleteRule(ctx, c, id int) error`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go`:

```go
func TestMaintainerrDeleteRule(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"DELETE /api/rules/2": `{"code":1,"result":"Success"}`,
		"DELETE /api/rules/3": `{"code":0,"result":"Delete Failed","message":"media server refused"}`,
	})
	ctx := context.Background()

	if err := MaintainerrDeleteRule(ctx, c, 2); err != nil {
		t.Errorf("delete 2: %v", err)
	}
	if err := MaintainerrDeleteRule(ctx, c, 3); err == nil || !strings.Contains(err.Error(), "media server refused") {
		t.Errorf("delete 3 error = %v", err)
	}
	if (*paths)[0] != "DELETE /api/rules/2" {
		t.Errorf("requests = %v", *paths)
	}
}
```

Add `"maintainerr_delete_rule",` to `maintainerrDestructiveTools`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ -run DeleteRule 2>&1 | head -3`
Expected: `undefined: MaintainerrDeleteRule`.

- [ ] **Step 3: Implement.** Append to `pkg/arr/maintainerr_rules.go`:

```go
// MaintainerrDeleteRule deletes a rule group and its collection, including the
// media server collection. It waits on the media server, so it runs on the
// long timeout.
func MaintainerrDeleteRule(ctx context.Context, c *Client, id int) error {
	body, err := c.WithTimeout(maintainerrSlowTimeout).Delete(ctx, "/rules/"+itoa(id))
	if err != nil {
		return err
	}
	return maintainerrCheck(body, "delete the rule group")
}
```

Register after `maintainerr_set_deletion_policy`:

```go
	register(s, svc, spec, toolMeta{
		name: "maintainerr_delete_rule",
		description: "Delete a Maintainerr rule group and its collection, including the collection on the " +
			"media server. Items in it are no longer scheduled; nothing is deleted from disk.",
		access: AccessDestructive,
	}, func(ctx context.Context, c *arr.Client, in IDArgs) (Deleted, error) {
		err := arr.MaintainerrDeleteRule(ctx, c, in.ID)
		return Deleted{ID: in.ID, Deleted: err == nil}, err
	})
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr pkg/server
git commit -m "feat(maintainerr): delete rule groups"
```

---

### Task 7: Manual collection membership

**Files:**
- Modify: `pkg/arr/maintainerr_rules.go`, `pkg/arr/maintainerr_rules_test.go`
- Modify: `pkg/server/tools_maintainerr.go`, `pkg/server/register_maintainerr.go`, `pkg/server/register_maintainerr_test.go`

**Interfaces:**
- Consumes: `maintainerrValidateMediaID` (Task 3), `CollectionIDArgs` pattern (existing).
- Produces: `MaintainerrSetCollectionMembership(ctx, c, collectionID int, mediaServerID string, add bool) error`.

- [ ] **Step 1: Write the failing tests.** Append to `pkg/arr/maintainerr_rules_test.go`:

```go
func TestMaintainerrMembershipSendsTheItemsContext(t *testing.T) {
	c, paths, bodies := maintainerrRoutes(t, map[string]string{
		"GET /api/media-server/meta/abc":   `{"id":"abc","title":"S1","type":"season","index":1,"parentIndex":null}`,
		"POST /api/collections/media/add": `{"id":2}`,
	})
	ctx := context.Background()

	if err := MaintainerrSetCollectionMembership(ctx, c, 2, "abc", true); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := MaintainerrSetCollectionMembership(ctx, c, 2, "abc", false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := strings.Join(*paths, ","); got != "GET /api/media-server/meta/abc,POST /api/collections/media/add,"+
		"GET /api/media-server/meta/abc,POST /api/collections/media/add" {
		t.Errorf("requests = %s", got)
	}
	if (*bodies)[1] != `{"action":0,"collectionId":2,"context":{"id":"abc","index":1,"type":"season"},"mediaId":"abc"}` {
		t.Errorf("add body = %s", (*bodies)[1])
	}
	if !strings.HasPrefix((*bodies)[3], `{"action":1,`) {
		t.Errorf("remove body = %s", (*bodies)[3])
	}
}

// Metadata answers an unknown id with 200 and an empty body.
func TestMaintainerrMembershipNamesUnknownItems(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{
		"GET /api/media-server/meta/nope": ``,
	})

	err := MaintainerrSetCollectionMembership(context.Background(), c, 2, "nope", true)
	if err == nil || !strings.Contains(err.Error(), `no media server item "nope"`) {
		t.Errorf("error = %v", err)
	}
	if len(*paths) != 1 {
		t.Errorf("requests = %v, want only the metadata read", *paths)
	}
}

func TestMaintainerrMembershipRejectsPathLikeIDs(t *testing.T) {
	c, paths, _ := maintainerrRoutes(t, map[string]string{})

	if err := MaintainerrSetCollectionMembership(context.Background(), c, 2, "../settings", true); err == nil || len(*paths) != 0 {
		t.Errorf("error = %v, requests = %v", err, *paths)
	}
}
```

Add `"maintainerr_remove_from_collection",` to `maintainerrWriteTools` and `"maintainerr_add_to_collection",` to `maintainerrDestructiveTools`. Also extend `TestMaintainerrRemoveExclusionIsDestructive` into a loop over `maintainerrDestructiveTools`: replace its body's `if tool.Name != "maintainerr_remove_exclusion"` check and final `t.Fatal` with:

```go
	seen := map[string]bool{}
	for _, tool := range res.Tools {
		if !has(maintainerrDestructiveTools, tool.Name) {
			continue
		}
		seen[tool.Name] = true
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Errorf("%s annotations = %+v, want destructive", tool.Name, tool.Annotations)
		}
	}
	for _, name := range maintainerrDestructiveTools {
		if !seen[name] {
			t.Errorf("%s not advertised", name)
		}
	}
```

and rename the test to `TestMaintainerrDestructiveToolsAreAnnotated`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pkg/arr/ -run Membership 2>&1 | head -3`
Expected: `undefined: MaintainerrSetCollectionMembership`.

- [ ] **Step 3: Implement.** Append to `pkg/arr/maintainerr_rules.go` (add `"bytes"` to imports):

```go
// MaintainerrSetCollectionMembership adds an item to a collection by hand, or
// removes it. Maintainerr needs the item's type and position to resolve shows,
// seasons and episodes, so the item's metadata is read first. Both calls reach
// the media server, so they run on the long timeout.
func MaintainerrSetCollectionMembership(ctx context.Context, c *Client, collectionID int, mediaServerID string, add bool) error {
	if err := maintainerrValidateMediaID(mediaServerID); err != nil {
		return err
	}
	slow := c.WithTimeout(maintainerrSlowTimeout)
	raw, err := slow.Get(ctx, "/media-server/meta/"+mediaServerID)
	if err != nil {
		return err
	}
	var meta struct {
		Type        string `json:"type"`
		Index       *int   `json:"index"`
		ParentIndex *int   `json:"parentIndex"`
	}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := unmarshal(raw, &meta); err != nil {
			return err
		}
	}
	if meta.Type == "" {
		return fmt.Errorf("no media server item %q", mediaServerID)
	}
	itemContext := map[string]any{"id": mediaServerID, "type": meta.Type}
	if meta.Index != nil {
		itemContext["index"] = *meta.Index
	}
	if meta.ParentIndex != nil {
		itemContext["parentIndex"] = *meta.ParentIndex
	}
	action := 1
	if add {
		action = 0
	}
	_, err = slow.Post(ctx, "/collections/media/add", map[string]any{
		"action": action, "mediaId": mediaServerID, "collectionId": collectionID, "context": itemContext,
	})
	return err
}
```

Append to `pkg/server/tools_maintainerr.go`:

```go
// CollectionMembershipArgs identifies one item in one collection.
type CollectionMembershipArgs struct {
	InstanceArg
	CollectionID  int    `json:"collectionId" jsonschema:"collection id from maintainerr_list_collections"`
	MediaServerID string `json:"mediaServerId" jsonschema:"media server item id"`
}

// MembershipChanged reports a manual collection change.
type MembershipChanged struct {
	CollectionID  int    `json:"collectionId"`
	MediaServerID string `json:"mediaServerId"`
	InCollection  bool   `json:"inCollection"`
}
```

Register after `maintainerr_delete_rule`:

```go
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
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./pkg/arr/ ./pkg/server/ -race 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add pkg/arr pkg/server
git commit -m "feat(maintainerr): add and remove collection items by hand"
```

---

### Task 8: Docs, full gate, protocol probe and live check

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: every tool from Tasks 1–7.

- [ ] **Step 1: Update README.md.** Change `**260 tools**` to `**271 tools**`, and `### Maintainerr (14)` to `### Maintainerr (25)`. Replace the whole paragraph under that heading (it begins `Rule-driven library cleanup.` and ends `for what is left out.`) with:

```markdown
Rule-driven library cleanup. A rule group fills a collection; each item in it is acted
on (usually deleted from disk through Sonarr or Radarr) `deleteAfterDays` after it
entered. These tools show what is about to go, keep or delay it, and create and edit
the rule groups themselves, written in Maintainerr's own YAML. Anything that brings a
deletion closer is destructive-tier. See [Scope](#maintainerr) for what is left out.
``` Append these rows to the Maintainerr tool table, before `maintainerr_remove_exclusion`:

```markdown
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
```

In the Scope section's `### Maintainerr` paragraph, replace everything from `The tools stop at keeping and delaying media.` to the end of that paragraph with:

```markdown
Rule groups are created and edited through Maintainerr's YAML encode and decode
endpoints, so the model never handles its numeric rule encoding, and every edit reads
the whole group back and writes it whole, because Maintainerr's update resets fields it
is not sent. Left out on purpose: running collection handling (`/collections/handle`,
which deletes due media immediately), collection-only groups without rules, and every
write under `/api/settings`, which returns every stored credential in plaintext.
```

- [ ] **Step 2: Run the full gate**

```bash
go build -o arr-mcp ./cmd/arr-mcp && go vet ./... && gofmt -l . && go test ./... -race
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run
go run github.com/securego/gosec/v2/cmd/gosec@latest -quiet ./...
trivy fs --ignorefile .trivyignore.yaml --scanners vuln,secret,misconfig --severity CRITICAL,HIGH,MEDIUM .
```

Expected: tests `ok`, `gofmt -l` empty, `0 issues.`, gosec silent, Trivy only KSV-0013 and KSV-0125 on `deploy/kubernetes/deployment.yaml` (pre-existing).

- [ ] **Step 3: Protocol probe, reads only.** With the scratchpad copy of the homelab config (`maintainerr` included), run the AGENTS.md stdio probe plus `tools/call` for `maintainerr_list_libraries`, `maintainerr_list_arr_servers`, `maintainerr_list_rule_properties {"application":"Radarr"}`, `maintainerr_get_rule {"id":1}`. Expected: `tools/list` reports 271 tools, 25 of them `maintainerr_`; `get_rule` carries `rulesYaml`; `grep -ci 'apikey\|webhook'` over the tool results (not `tools/list`) is 0.

- [ ] **Step 4: Live write check — ask the user first.** Only after an explicit OK, against the live instance through the probe: `maintainerr_create_rule` on the Movies library with `arrAction: DO_NOTHING`, `deleteAfterDays: 30`, and rulesYaml copied from `get_rule {"id":1}`; `maintainerr_test_rule` on it with one Movies item id; `maintainerr_update_rule` renaming it; `maintainerr_get_rule` confirming overlayEnabled, the schedule and the rules survived; `maintainerr_delete_rule`. Confirm `maintainerr_list_rules` is back to the three original groups.

- [ ] **Step 5: Commit**

```bash
rm -f arr-mcp
git add README.md
git commit -m "docs: document Maintainerr rule management tools"
```
