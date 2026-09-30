package arr

import (
	"context"
	"encoding/json"
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
