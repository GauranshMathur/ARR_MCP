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
