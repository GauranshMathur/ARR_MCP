package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	var serverKind, serverField string
	switch lib.Type {
	case "movie":
		serverKind, serverField = "radarr", "radarrSettingsId"
	case "show":
		serverKind, serverField = "sonarr", "sonarrSettingsId"
	default:
		return MaintainerrRuleDetail{}, fmt.Errorf("library %q has type %q, which maps to neither radarr nor sonarr", in.LibraryID, lib.Type)
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

	// The highest rule id that exists before the create is the floor a
	// recovered id must clear: Maintainerr's create answers success without
	// the new group's id, and a POST that reports success without truly
	// persisting must not be mistaken for one, by returning an older group
	// that merely shares its name.
	before, err := maintainerrHighestRuleID(ctx, c)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}

	body, err := c.WithTimeout(maintainerrSlowTimeout).Post(ctx, "/rules", group)
	if err != nil {
		return MaintainerrRuleDetail{}, err
	}
	if err := maintainerrCheck(body, "create the rule group"); err != nil {
		return MaintainerrRuleDetail{}, err
	}

	// From here Maintainerr has already reported success, so any failure to
	// read the result back must say so: a live rule group exists even when
	// this call cannot confirm which one it is.
	id, err := maintainerrFindRule(ctx, c, in.Name, lib.ID, before)
	if err != nil {
		return MaintainerrRuleDetail{}, fmt.Errorf("rule group %q was created but could not be read back: %w", in.Name, err)
	}
	detail, err := MaintainerrGetRule(ctx, c, id)
	if err != nil {
		return MaintainerrRuleDetail{}, fmt.Errorf("rule group %q was created but could not be read back: %w", in.Name, err)
	}
	return detail, nil
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

// maintainerrHighestRuleID returns the highest rule group id Maintainerr
// currently reports, 0 when there are none. maintainerrFindRule uses it as
// the floor a newly created group's id must clear.
func maintainerrHighestRuleID(ctx context.Context, c *Client) (int, error) {
	rules, err := MaintainerrListRules(ctx, c)
	if err != nil {
		return 0, err
	}
	id := 0
	for _, r := range rules {
		if r.ID > id {
			id = r.ID
		}
	}
	return id, nil
}

// maintainerrFindRule finds the group a create just made. Maintainerr answers
// a create without its id, and names need not be unique, so the newest group
// with this name on this library is taken -- but only when its id is greater
// than minID, the highest id that existed before the create: an id at or
// below minID belongs to an older group that merely shares the name, not the
// one just created.
func maintainerrFindRule(ctx context.Context, c *Client, name, libraryID string, minID int) (int, error) {
	rules, err := MaintainerrListRules(ctx, c)
	if err != nil {
		return 0, err
	}
	id := 0
	for _, r := range rules {
		if r.Name == name && r.LibraryID == libraryID && r.ID > minID && r.ID > id {
			id = r.ID
		}
	}
	if id == 0 {
		return 0, fmt.Errorf("maintainerr reported success but rule group %q was not found", name)
	}
	return id, nil
}
