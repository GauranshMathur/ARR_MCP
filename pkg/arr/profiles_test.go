package arr

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// schemaProfile is the shape GET /qualityprofile/schema returns, cut down to
// the entries the tests need. It keeps the two things that make the endpoint
// awkward: a nested quality *group* with its own id, and Radarr's language
// object, which Sonarr does not send.
const schemaProfile = `{
  "name": "",
  "upgradeAllowed": false,
  "cutoff": 0,
  "items": [
    {"quality":{"id":0,"name":"Unknown","source":"unknown","resolution":0},"items":[],"allowed":false},
    {"quality":{"id":1,"name":"SDTV","source":"television","resolution":480},"items":[],"allowed":false},
    {"name":"WEB 480p","items":[
      {"quality":{"id":12,"name":"WEBRip-480p"},"items":[],"allowed":false},
      {"quality":{"id":8,"name":"WEBDL-480p"},"items":[],"allowed":false}],
     "allowed":false,"id":1000},
    {"quality":{"id":4,"name":"HDTV-720p"},"items":[],"allowed":false},
    {"quality":{"id":9,"name":"HDTV-1080p"},"items":[],"allowed":false}
  ],
  "minFormatScore": 0,
  "cutoffFormatScore": 0,
  "minUpgradeFormatScore": 1,
  "formatItems": [
    {"format":1,"name":"x264","score":0},
    {"format":2,"name":"x265","score":0}
  ],
  "language": {"id":-2,"name":"Original"}
}`

// storedProfile is what GET /qualityprofile/{id} returns for an existing
// profile, with a key this package does not model so the read-modify-write can
// be checked for dropping it.
const storedProfile = `{
  "id": 7,
  "name": "WEB-1080p",
  "upgradeAllowed": true,
  "cutoff": 1000,
  "items": [
    {"quality":{"id":1,"name":"SDTV"},"items":[],"allowed":false},
    {"name":"WEB 480p","items":[
      {"quality":{"id":12,"name":"WEBRip-480p"},"items":[],"allowed":true},
      {"quality":{"id":8,"name":"WEBDL-480p"},"items":[],"allowed":true}],
     "allowed":true,"id":1000},
    {"quality":{"id":9,"name":"HDTV-1080p"},"items":[],"allowed":true}
  ],
  "minFormatScore": 0,
  "cutoffFormatScore": 10000,
  "minUpgradeFormatScore": 1,
  "formatItems": [
    {"format":1,"name":"x264","score":0},
    {"format":2,"name":"x265","score":-100}
  ],
  "someFutureSetting": "must survive"
}`

// bodyOf decodes a recorded request body into a map for assertions.
func bodyOf(t *testing.T, e exchange) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(e.body), &body); err != nil {
		t.Fatalf("decoding request body %q: %v", e.body, err)
	}
	return body
}

// itemsOf pulls the quality list out of a decoded profile body.
func itemsOf(t *testing.T, body map[string]any) []any {
	t.Helper()
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("body has no items array: %v", body)
	}
	return items
}

// allowedNames lists the entries a profile body turns on, groups by their group
// name and single qualities by their own.
func allowedNames(t *testing.T, body map[string]any) []string {
	t.Helper()
	var out []string
	var walk func(items []any)
	walk = func(items []any) {
		for _, raw := range items {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name, _ := item["name"].(string)
			if name == "" {
				if q, ok := item["quality"].(map[string]any); ok {
					name, _ = q["name"].(string)
				}
			}
			if allowed, _ := item["allowed"].(bool); allowed {
				out = append(out, name)
			}
			if nested, ok := item["items"].([]any); ok {
				walk(nested)
			}
		}
	}
	walk(itemsOf(t, body))
	return out
}

// --- reading a quality profile ---

func TestGetQualityProfileNamesTheCutoffAndTheAllowedQualities(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	got, err := GetQualityProfile(context.Background(), sonarrClient(srv.URL), 7)
	if err != nil {
		t.Fatalf("GetQualityProfile: %v", err)
	}
	if got.Name != "WEB-1080p" || !got.UpgradeAllowed {
		t.Errorf("profile identity lost: %+v", got)
	}
	// The cutoff is stored as an id; a caller cannot act on 1000.
	if got.Cutoff != "WEB 480p" {
		t.Errorf("cutoff = %q, want the group name WEB 480p", got.Cutoff)
	}
	if got.CutoffFormatScore != 10000 {
		t.Errorf("cutoffFormatScore = %d, want 10000", got.CutoffFormatScore)
	}
	if len(got.Items) != 3 {
		t.Fatalf("items = %+v, want three entries", got.Items)
	}
	group := got.Items[1]
	if group.Name != "WEB 480p" || !group.Allowed {
		t.Errorf("group entry = %+v", group)
	}
	if len(group.Members) != 2 || group.Members[0] != "WEBRip-480p" {
		t.Errorf("group members = %v, want the two WEB 480p qualities", group.Members)
	}
	// Only the scored formats are worth the context; a real profile carries
	// sixty formats sitting at zero.
	if len(got.FormatScores) != 1 || got.FormatScores[0].Name != "x265" || got.FormatScores[0].Score != -100 {
		t.Errorf("formatScores = %+v, want only the scored x265", got.FormatScores)
	}
}

func TestGetQualityProfileResolvesACutoffInsideAGroup(t *testing.T) {
	profile := strings.Replace(storedProfile, `"cutoff": 1000`, `"cutoff": 8`, 1)
	srv, _ := routedService(t, map[string]string{"/api/v3/qualityprofile/7": profile})

	got, err := GetQualityProfile(context.Background(), sonarrClient(srv.URL), 7)
	if err != nil {
		t.Fatalf("GetQualityProfile: %v", err)
	}
	if got.Cutoff != "WEBDL-480p" {
		t.Errorf("cutoff = %q, want WEBDL-480p", got.Cutoff)
	}
}

// --- creating a quality profile ---

func TestCreateQualityProfileStartsFromTheSchema(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualityprofile/schema": schemaProfile,
		"/api/v3/qualityprofile":        storedProfile,
	})

	upgrade := true
	_, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "HD", Allowed: []string{"HDTV-720p", "HDTV-1080p"},
		Cutoff: "HDTV-720p", UpgradeAllowed: &upgrade,
	})
	if err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/qualityprofile/schema")
	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/qualityprofile"))
	if body["name"] != "HD" {
		t.Errorf("name = %v, want HD", body["name"])
	}
	if body["upgradeAllowed"] != true {
		t.Errorf("upgradeAllowed = %v, want true", body["upgradeAllowed"])
	}
	if body["cutoff"] != float64(4) {
		t.Errorf("cutoff = %v, want the HDTV-720p quality id 4", body["cutoff"])
	}
	allowed := allowedNames(t, body)
	if len(allowed) != 2 || allowed[0] != "HDTV-720p" || allowed[1] != "HDTV-1080p" {
		t.Errorf("allowed = %v, want exactly the two named qualities", allowed)
	}
}

// The id a profile's cutoff takes is the quality's own id, not the id of the
// quality definition that wraps it: WEBDL-480p is definition 4 and quality 8.
// Sending the definition id points the cutoff at a different quality entirely.
func TestCreateQualityProfileSendsTheQualityIDNotTheDefinitionID(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualityprofile/schema": schemaProfile,
		"/api/v3/qualityprofile":        storedProfile,
		// If the implementation ever reaches for the definition table, it will
		// find WEBDL-480p sitting at definition id 4.
		"/api/v3/qualitydefinition": `[{"id":4,"title":"WEBDL-480p","quality":{"id":8,"name":"WEBDL-480p"}}]`,
	})

	if _, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "SD", Allowed: []string{"WEBDL-480p"}, Cutoff: "WEBDL-480p",
	}); err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/qualityprofile"))
	if body["cutoff"] != float64(8) {
		t.Errorf("cutoff = %v, want the quality id 8 rather than the definition id 4", body["cutoff"])
	}
	for _, e := range *seen {
		if strings.Contains(e.path, "qualitydefinition") {
			t.Errorf("the profile's own items already carry the ids; %s was not needed", e.path)
		}
	}
}

func TestCreateQualityProfileDefaultsTheCutoffToTheBestAllowedQuality(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualityprofile/schema": schemaProfile,
		"/api/v3/qualityprofile":        storedProfile,
	})

	if _, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "HD", Allowed: []string{"HDTV-1080p", "HDTV-720p"},
	}); err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/qualityprofile"))
	// The service orders items worst first, so the last allowed one is the best.
	if body["cutoff"] != float64(9) {
		t.Errorf("cutoff = %v, want the HDTV-1080p quality id 9", body["cutoff"])
	}
}

func TestCreateQualityProfileAllowsAWholeGroupByName(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualityprofile/schema": schemaProfile,
		"/api/v3/qualityprofile":        storedProfile,
	})

	if _, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "SD", Allowed: []string{"WEB 480p"},
	}); err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/qualityprofile"))
	allowed := allowedNames(t, body)
	if len(allowed) != 3 {
		t.Errorf("allowed = %v, want the group and both of its members", allowed)
	}
	if body["cutoff"] != float64(1000) {
		t.Errorf("cutoff = %v, want the group id 1000", body["cutoff"])
	}
}

func TestCreateQualityProfileRejectsAnUnknownQualityName(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/schema": schemaProfile})

	_, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "HD", Allowed: []string{"Blueray-1080p"},
	})
	if err == nil {
		t.Fatal("expected an error for a misspelled quality name")
	}
	if !strings.Contains(err.Error(), "HDTV-1080p") {
		t.Errorf("error %q does not list the quality names that would have worked", err)
	}
	for _, e := range *seen {
		if e.method == http.MethodPost {
			t.Errorf("a profile was posted despite the unknown quality: %v", e)
		}
	}
}

func TestCreateQualityProfileRequiresAnAllowedQuality(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := CreateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileCreate{
		Name: "Empty",
	}); err == nil {
		t.Fatal("expected an error when no qualities are allowed")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for a profile with no qualities", len(*seen))
	}
}

// Radarr's schema carries a language object Sonarr's does not. A typed round
// trip would drop it and the created profile would lose the setting.
func TestCreateQualityProfileKeepsSchemaFieldsItDoesNotModel(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/qualityprofile/schema": schemaProfile,
		"/api/v3/qualityprofile":        storedProfile,
	})

	if _, err := CreateQualityProfile(context.Background(), radarrClient(srv.URL), QualityProfileCreate{
		Name: "HD", Allowed: []string{"HDTV-1080p"},
	}); err != nil {
		t.Fatalf("CreateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/qualityprofile"))
	lang, ok := body["language"].(map[string]any)
	if !ok || lang["name"] != "Original" {
		t.Errorf("the schema's language was dropped: %v", body["language"])
	}
	if body["minUpgradeFormatScore"] != float64(1) {
		t.Errorf("minUpgradeFormatScore = %v, want the schema's 1", body["minUpgradeFormatScore"])
	}
}

// --- updating a quality profile ---

func TestUpdateQualityProfilePreservesUnknownUpstreamKeys(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	name := "WEB-1080p renamed"
	if _, err := UpdateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileUpdate{
		ID: 7, Name: &name,
	}); err != nil {
		t.Fatalf("UpdateQualityProfile: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/qualityprofile/7")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/qualityprofile/7"))
	if body["name"] != "WEB-1080p renamed" {
		t.Errorf("name = %v, want the new name", body["name"])
	}
	if body["someFutureSetting"] != "must survive" {
		t.Errorf("the read-modify-write dropped a key it does not model: %v", body)
	}
	if body["cutoff"] != float64(1000) {
		t.Errorf("cutoff = %v, want the stored 1000 left alone", body["cutoff"])
	}
}

func TestUpdateQualityProfileAppliesFormatScoresByName(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	if _, err := UpdateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileUpdate{
		ID: 7, FormatScores: map[string]int{"x264": 50},
	}); err != nil {
		t.Fatalf("UpdateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/qualityprofile/7"))
	items, ok := body["formatItems"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("formatItems = %v, want both formats kept", body["formatItems"])
	}
	scores := map[string]float64{}
	for _, raw := range items {
		item := raw.(map[string]any)
		scores[item["name"].(string)] = item["score"].(float64)
	}
	if scores["x264"] != 50 {
		t.Errorf("x264 score = %v, want 50", scores["x264"])
	}
	if scores["x265"] != -100 {
		t.Errorf("x265 score = %v, want the stored -100 left alone", scores["x265"])
	}
}

func TestUpdateQualityProfileRejectsAnUnknownFormatName(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	_, err := UpdateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileUpdate{
		ID: 7, FormatScores: map[string]int{"h265": 50},
	})
	if err == nil {
		t.Fatal("expected an error for a custom format the profile does not score")
	}
	if !strings.Contains(err.Error(), "x265") {
		t.Errorf("error %q does not list the format names that would have worked", err)
	}
	for _, e := range *seen {
		if e.method == http.MethodPut {
			t.Errorf("the profile was written despite the unknown format: %v", e)
		}
	}
}

func TestUpdateQualityProfileTogglesAllowedQualitiesByName(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	if _, err := UpdateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileUpdate{
		ID: 7, Allow: []string{"SDTV"}, Disallow: []string{"HDTV-1080p"},
	}); err != nil {
		t.Fatalf("UpdateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/qualityprofile/7"))
	allowed := allowedNames(t, body)
	want := map[string]bool{"SDTV": true, "WEB 480p": true, "WEBRip-480p": true, "WEBDL-480p": true}
	for _, name := range allowed {
		if !want[name] {
			t.Errorf("%q is allowed but should not be; allowed = %v", name, allowed)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("these should still be allowed: %v (allowed = %v)", want, allowed)
	}
}

func TestUpdateQualityProfileSetsTheFormatScoreThresholds(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": storedProfile})

	minimum, cutoff := 100, 500
	if _, err := UpdateQualityProfile(context.Background(), sonarrClient(srv.URL), QualityProfileUpdate{
		ID: 7, MinFormatScore: &minimum, CutoffFormatScore: &cutoff,
	}); err != nil {
		t.Fatalf("UpdateQualityProfile: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/qualityprofile/7"))
	if body["minFormatScore"] != float64(100) || body["cutoffFormatScore"] != float64(500) {
		t.Errorf("format score thresholds = %v / %v", body["minFormatScore"], body["cutoffFormatScore"])
	}
}

// A profile a series or movie still uses cannot be deleted; the service says so
// and the caller needs to see that rather than a bare failure.
func TestDeleteQualityProfileSurfacesTheServiceRefusal(t *testing.T) {
	srv, _ := fakeService(t, http.StatusBadRequest,
		`[{"errorMessage":"Quality profile is in use by a series"}]`)

	err := DeleteQualityProfile(context.Background(), sonarrClient(srv.URL), 7)
	if err == nil {
		t.Fatal("expected an error when the service refuses the delete")
	}
	if !strings.Contains(err.Error(), "in use by a series") {
		t.Errorf("error %q does not explain why the profile was kept", err)
	}
}

func TestDeleteQualityProfileCallsTheProfileRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/qualityprofile/7": `{}`})

	if err := DeleteQualityProfile(context.Background(), sonarrClient(srv.URL), 7); err != nil {
		t.Fatalf("DeleteQualityProfile: %v", err)
	}
	find(t, *seen, http.MethodDelete, "/api/v3/qualityprofile/7")
}

// --- custom formats ---

const storedFormat = `{
  "id": 3,
  "name": "DV HDR10",
  "includeCustomFormatWhenRenaming": false,
  "specifications": [
    {"name":"DV HDR10","implementation":"ReleaseTitleSpecification",
     "implementationName":"Release Title","infoLink":"https://wiki.servarr.com/x",
     "negate":false,"required":true,
     "fields":[{"order":0,"name":"value","label":"Regular Expression",
       "helpText":"Custom Format RegEx is Case Insensitive","value":"\\bDV\\b",
       "type":"textbox","advanced":false,"privacy":"normal","isFloat":false}]}
  ],
  "someFutureSetting": "must survive"
}`

func TestGetCustomFormatFlattensSpecificationFields(t *testing.T) {
	srv, _ := routedService(t, map[string]string{"/api/v3/customformat/3": storedFormat})

	got, err := GetCustomFormat(context.Background(), sonarrClient(srv.URL), 3)
	if err != nil {
		t.Fatalf("GetCustomFormat: %v", err)
	}
	if got.Name != "DV HDR10" {
		t.Errorf("name = %q, want DV HDR10", got.Name)
	}
	if len(got.Specifications) != 1 {
		t.Fatalf("specifications = %+v, want one rule", got.Specifications)
	}
	spec := got.Specifications[0]
	if spec.Implementation != "ReleaseTitleSpecification" || !spec.Required || spec.Negate {
		t.Errorf("specification = %+v", spec)
	}
	if spec.Fields["value"] != `\bDV\b` {
		t.Errorf("fields = %v, want the regular expression under value", spec.Fields)
	}
	// The label, help text and privacy of every field are UI furniture.
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshalling format: %v", err)
	}
	if strings.Contains(string(raw), "Case Insensitive") {
		t.Errorf("the field help text reached the projection: %s", raw)
	}
}

func TestCreateCustomFormatSendsFieldsAsNameValuePairs(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/customformat": storedFormat})

	_, err := CreateCustomFormat(context.Background(), sonarrClient(srv.URL), CustomFormatCreate{
		Name: "x266", IncludeWhenRenaming: true,
		Specifications: []CustomFormatSpecification{{
			Name: "x266", Implementation: "ReleaseTitleSpecification", Required: true,
			Fields: map[string]any{"value": `\bx266\b`},
		}},
	})
	if err != nil {
		t.Fatalf("CreateCustomFormat: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/customformat"))
	if body["name"] != "x266" || body["includeCustomFormatWhenRenaming"] != true {
		t.Errorf("format body = %v", body)
	}
	specs, ok := body["specifications"].([]any)
	if !ok || len(specs) != 1 {
		t.Fatalf("specifications = %v, want one rule", body["specifications"])
	}
	spec := specs[0].(map[string]any)
	if spec["implementation"] != "ReleaseTitleSpecification" || spec["required"] != true {
		t.Errorf("specification = %v", spec)
	}
	// The service reads a specification's settings from an array of named
	// fields, which is the shape it returns them in; an object is rejected.
	fields, ok := spec["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("fields = %v, want an array of named fields", spec["fields"])
	}
	field := fields[0].(map[string]any)
	if field["name"] != "value" || field["value"] != `\bx266\b` {
		t.Errorf("field = %v, want name/value pair", field)
	}
}

func TestCreateCustomFormatRequiresANameAndAnImplementation(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := CreateCustomFormat(context.Background(), sonarrClient(srv.URL), CustomFormatCreate{
		Name: "x266",
		Specifications: []CustomFormatSpecification{{
			Name: "x266", Fields: map[string]any{"value": "x266"},
		}},
	}); err == nil {
		t.Fatal("expected an error for a specification with no implementation")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for an incomplete format", len(*seen))
	}
}

func TestUpdateCustomFormatPreservesUnknownUpstreamKeys(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/customformat/3": storedFormat})

	include := true
	if _, err := UpdateCustomFormat(context.Background(), sonarrClient(srv.URL), CustomFormatUpdate{
		ID: 3, IncludeWhenRenaming: &include,
	}); err != nil {
		t.Fatalf("UpdateCustomFormat: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/customformat/3")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/customformat/3"))
	if body["includeCustomFormatWhenRenaming"] != true {
		t.Errorf("includeCustomFormatWhenRenaming = %v, want true", body["includeCustomFormatWhenRenaming"])
	}
	if body["someFutureSetting"] != "must survive" {
		t.Errorf("the read-modify-write dropped a key it does not model: %v", body)
	}
	if body["name"] != "DV HDR10" {
		t.Errorf("name = %v, want the stored name left alone", body["name"])
	}
	// The rules were not named, so the stored ones must come back untouched.
	specs, ok := body["specifications"].([]any)
	if !ok || len(specs) != 1 {
		t.Fatalf("specifications = %v, want the stored rule kept", body["specifications"])
	}
}

func TestUpdateCustomFormatReplacesTheSpecificationsWhenGiven(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/customformat/3": storedFormat})

	if _, err := UpdateCustomFormat(context.Background(), sonarrClient(srv.URL), CustomFormatUpdate{
		ID: 3, Specifications: []CustomFormatSpecification{{
			Name: "DV", Implementation: "ReleaseTitleSpecification",
			Fields: map[string]any{"value": `\bDoVi\b`},
		}},
	}); err != nil {
		t.Fatalf("UpdateCustomFormat: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/customformat/3"))
	specs := body["specifications"].([]any)
	if len(specs) != 1 {
		t.Fatalf("specifications = %v", specs)
	}
	if specs[0].(map[string]any)["name"] != "DV" {
		t.Errorf("specification = %v, want the replacement", specs[0])
	}
}

func TestDeleteCustomFormatCallsTheFormatRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/customformat/3": `{}`})

	if err := DeleteCustomFormat(context.Background(), sonarrClient(srv.URL), 3); err != nil {
		t.Fatalf("DeleteCustomFormat: %v", err)
	}
	find(t, *seen, http.MethodDelete, "/api/v3/customformat/3")
}

// --- root folders ---

func TestAddRootFolderPostsThePath(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/rootfolder": `{"id":4,"path":"/NAS/Anime","accessible":true}`,
	})

	got, err := AddRootFolder(context.Background(), sonarrClient(srv.URL), "/NAS/Anime")
	if err != nil {
		t.Fatalf("AddRootFolder: %v", err)
	}
	if got.ID != 4 || got.Path != "/NAS/Anime" {
		t.Errorf("root folder = %+v", got)
	}
	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/rootfolder"))
	if body["path"] != "/NAS/Anime" {
		t.Errorf("body = %v, want the path", body)
	}
}

func TestAddRootFolderRequiresAPath(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := AddRootFolder(context.Background(), sonarrClient(srv.URL), "  "); err == nil {
		t.Fatal("expected an error for an empty root folder path")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times with no path", len(*seen))
	}
}

func TestDeleteRootFolderCallsTheRootFolderRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/rootfolder/4": `{}`})

	if err := DeleteRootFolder(context.Background(), sonarrClient(srv.URL), 4); err != nil {
		t.Fatalf("DeleteRootFolder: %v", err)
	}
	find(t, *seen, http.MethodDelete, "/api/v3/rootfolder/4")
}

// --- naming configuration ---

const sonarrNaming = `{
  "id": 1,
  "renameEpisodes": true,
  "replaceIllegalCharacters": false,
  "colonReplacementFormat": 4,
  "multiEpisodeStyle": 5,
  "standardEpisodeFormat": "{Series Title} - S{season:00}E{episode:00}",
  "seasonFolderFormat": "Season {season:00}"
}`

const radarrNaming = `{
  "id": 1,
  "renameMovies": true,
  "replaceIllegalCharacters": false,
  "colonReplacementFormat": "smart",
  "standardMovieFormat": "{Movie Title} ({Release Year})",
  "movieFolderFormat": "{Movie Title} ({Release Year})"
}`

// colonReplacementFormat is an integer on Sonarr and a string on Radarr, so no
// Go struct decodes both. Writing the config back through a map is what keeps
// the setting from being reset by a typed round trip.
func TestUpdateNamingConfigOnlyChangesTheFieldsGiven(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/config/naming":   sonarrNaming,
		"/api/v3/config/naming/1": sonarrNaming,
	})

	format := "{Series TitleYear} - S{season:00}E{episode:00}"
	if _, err := UpdateNamingConfig(context.Background(), sonarrClient(srv.URL), NamingConfigUpdate{
		StandardEpisodeFormat: &format,
	}); err != nil {
		t.Fatalf("UpdateNamingConfig: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/config/naming")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/config/naming/1"))
	if body["standardEpisodeFormat"] != format {
		t.Errorf("standardEpisodeFormat = %v, want the new format", body["standardEpisodeFormat"])
	}
	if body["colonReplacementFormat"] != float64(4) {
		t.Errorf("colonReplacementFormat = %v, want Sonarr's integer 4 kept", body["colonReplacementFormat"])
	}
	if body["renameEpisodes"] != true || body["seasonFolderFormat"] != "Season {season:00}" {
		t.Errorf("an untouched setting changed: %v", body)
	}
}

// The two services name the same switch differently, so one argument has to
// land on whichever key the instance actually has.
func TestUpdateNamingConfigMapsRenameOntoTheServicesOwnKey(t *testing.T) {
	for _, tc := range []struct {
		service, config, key string
		client               func(string) *Client
	}{
		{"sonarr", sonarrNaming, "renameEpisodes", sonarrClient},
		{"radarr", radarrNaming, "renameMovies", radarrClient},
	} {
		t.Run(tc.service, func(t *testing.T) {
			srv, seen := routedService(t, map[string]string{
				"/api/v3/config/naming":   tc.config,
				"/api/v3/config/naming/1": tc.config,
			})

			rename := false
			if _, err := UpdateNamingConfig(context.Background(), tc.client(srv.URL), NamingConfigUpdate{
				RenameFiles: &rename,
			}); err != nil {
				t.Fatalf("UpdateNamingConfig: %v", err)
			}

			body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/config/naming/1"))
			if body[tc.key] != false {
				t.Errorf("%s = %v, want false", tc.key, body[tc.key])
			}
			if len(body) != len(bodyOf(t, exchange{body: tc.config})) {
				t.Errorf("the update invented a key: %v", body)
			}
		})
	}
}

func TestUpdateNamingConfigRejectsASettingTheInstanceDoesNotHave(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/config/naming":   radarrNaming,
		"/api/v3/config/naming/1": radarrNaming,
	})

	format := "Season {season:00}"
	_, err := UpdateNamingConfig(context.Background(), radarrClient(srv.URL), NamingConfigUpdate{
		SeasonFolderFormat: &format,
	})
	if err == nil {
		t.Fatal("expected an error for a Sonarr-only setting on Radarr")
	}
	if !strings.Contains(err.Error(), "seasonFolderFormat") {
		t.Errorf("error %q does not name the setting that does not exist", err)
	}
	for _, e := range *seen {
		if e.method == http.MethodPut {
			t.Errorf("the config was written despite the unusable setting: %v", e)
		}
	}
}

// --- media management configuration ---

const sonarrMediaManagement = `{
  "id": 1,
  "autoUnmonitorPreviouslyDownloadedEpisodes": false,
  "recycleBin": "",
  "recycleBinCleanupDays": 7,
  "downloadPropersAndRepacks": "preferAndUpgrade",
  "createEmptySeriesFolders": false,
  "deleteEmptyFolders": false,
  "fileDate": "none",
  "rescanAfterRefresh": "always",
  "episodeTitleRequired": "always",
  "minimumFreeSpaceWhenImporting": 100,
  "copyUsingHardlinks": true,
  "importExtraFiles": false,
  "extraFileExtensions": "srt",
  "enableMediaInfo": true,
  "someFutureSetting": "must survive"
}`

func TestGetMediaManagementConfigReportsTheFileHandlingPolicy(t *testing.T) {
	srv, _ := routedService(t, map[string]string{
		"/api/v3/config/mediamanagement": sonarrMediaManagement,
	})

	got, err := GetMediaManagementConfig(context.Background(), sonarrClient(srv.URL))
	if err != nil {
		t.Fatalf("GetMediaManagementConfig: %v", err)
	}
	if !got.CopyUsingHardlinks || got.ExtraFileExtensions != "srt" {
		t.Errorf("config = %+v", got)
	}
	if got.DownloadPropersAndRepacks != "preferAndUpgrade" {
		t.Errorf("downloadPropersAndRepacks = %q", got.DownloadPropersAndRepacks)
	}
	if got.EpisodeTitleRequired != "always" {
		t.Errorf("episodeTitleRequired = %q, want the Sonarr-only setting kept", got.EpisodeTitleRequired)
	}
}

func TestUpdateMediaManagementConfigOnlyChangesTheFieldsGiven(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/config/mediamanagement":   sonarrMediaManagement,
		"/api/v3/config/mediamanagement/1": sonarrMediaManagement,
	})

	bin := "/NAS/.recycle"
	hardlinks := false
	if _, err := UpdateMediaManagementConfig(context.Background(), sonarrClient(srv.URL),
		MediaManagementConfigUpdate{RecycleBin: &bin, CopyUsingHardlinks: &hardlinks}); err != nil {
		t.Fatalf("UpdateMediaManagementConfig: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/config/mediamanagement")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/config/mediamanagement/1"))
	if body["recycleBin"] != "/NAS/.recycle" || body["copyUsingHardlinks"] != false {
		t.Errorf("body = %v", body)
	}
	if body["someFutureSetting"] != "must survive" {
		t.Errorf("the read-modify-write dropped a key it does not model: %v", body)
	}
	if body["episodeTitleRequired"] != "always" || body["extraFileExtensions"] != "srt" {
		t.Errorf("an untouched setting changed: %v", body)
	}
}

func TestUpdateMediaManagementConfigMapsAutoUnmonitorOntoTheServicesOwnKey(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/config/mediamanagement":   sonarrMediaManagement,
		"/api/v3/config/mediamanagement/1": sonarrMediaManagement,
	})

	unmonitor := true
	if _, err := UpdateMediaManagementConfig(context.Background(), sonarrClient(srv.URL),
		MediaManagementConfigUpdate{AutoUnmonitorPreviouslyDownloaded: &unmonitor}); err != nil {
		t.Fatalf("UpdateMediaManagementConfig: %v", err)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/config/mediamanagement/1"))
	if body["autoUnmonitorPreviouslyDownloadedEpisodes"] != true {
		t.Errorf("autoUnmonitorPreviouslyDownloadedEpisodes = %v, want true", body)
	}
	if _, invented := body["autoUnmonitorPreviouslyDownloadedMovies"]; invented {
		t.Errorf("the update invented Radarr's key on a Sonarr instance: %v", body)
	}
}

// --- delay profiles ---

const storedDelayProfile = `{
  "id": 1,
  "enableUsenet": true,
  "enableTorrent": true,
  "preferredProtocol": "usenet",
  "usenetDelay": 0,
  "torrentDelay": 0,
  "bypassIfHighestQuality": true,
  "bypassIfAboveCustomFormatScore": false,
  "minimumCustomFormatScore": 0,
  "order": 2147483647,
  "tags": [],
  "someFutureSetting": "must survive"
}`

func TestUpdateDelayProfilePreservesUnknownUpstreamKeys(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/delayprofile/1": storedDelayProfile})

	delay := 60
	torrents := false
	if _, err := UpdateDelayProfile(context.Background(), sonarrClient(srv.URL), DelayProfileUpdate{
		ID: 1, UsenetDelay: &delay, EnableTorrent: &torrents,
	}); err != nil {
		t.Fatalf("UpdateDelayProfile: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/delayprofile/1")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/delayprofile/1"))
	if body["usenetDelay"] != float64(60) || body["enableTorrent"] != false {
		t.Errorf("body = %v", body)
	}
	if body["order"] != float64(2147483647) || body["someFutureSetting"] != "must survive" {
		t.Errorf("the read-modify-write dropped a key it does not model: %v", body)
	}
	if body["bypassIfHighestQuality"] != true {
		t.Errorf("an untouched setting changed: %v", body)
	}
}

func TestUpdateDelayProfileRejectsAnUnknownProtocol(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/delayprofile/1": storedDelayProfile})

	protocol := "bittorrent"
	_, err := UpdateDelayProfile(context.Background(), sonarrClient(srv.URL), DelayProfileUpdate{
		ID: 1, PreferredProtocol: &protocol,
	})
	if err == nil {
		t.Fatal("expected an error for a protocol the service does not know")
	}
	if !strings.Contains(err.Error(), "torrent") {
		t.Errorf("error %q does not list the protocols that would have worked", err)
	}
	for _, e := range *seen {
		if e.method == http.MethodPut {
			t.Errorf("the profile was written despite the unknown protocol: %v", e)
		}
	}
}

// --- release profiles ---

// The shipped Sonarr web UI seeds a new release profile with exactly these
// values, so the arrays go out empty rather than null.
func TestCreateReleaseProfileSendsTheDefaultsTheUISends(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/releaseprofile": `{"id":2,"name":"No x265","enabled":true,"ignored":["x265"],"tags":[]}`,
	})

	got, err := CreateReleaseProfile(context.Background(), sonarrClient(srv.URL), ReleaseProfileCreate{
		Name: "No x265", Ignored: []string{"x265"},
	})
	if err != nil {
		t.Fatalf("CreateReleaseProfile: %v", err)
	}
	if got.ID != 2 || len(got.Ignored) != 1 {
		t.Errorf("release profile = %+v", got)
	}

	body := bodyOf(t, find(t, *seen, http.MethodPost, "/api/v3/releaseprofile"))
	if body["enabled"] != true {
		t.Errorf("enabled = %v, want a new profile to be on", body["enabled"])
	}
	if body["indexerId"] != float64(0) {
		t.Errorf("indexerId = %v, want 0 for every indexer", body["indexerId"])
	}
	for _, key := range []string{"required", "ignored", "tags"} {
		if _, ok := body[key].([]any); !ok {
			t.Errorf("%s = %v, want an array rather than null", key, body[key])
		}
	}
}

func TestCreateReleaseProfileRequiresATerm(t *testing.T) {
	srv, seen := routedService(t, nil)

	if _, err := CreateReleaseProfile(context.Background(), sonarrClient(srv.URL), ReleaseProfileCreate{
		Name: "Empty",
	}); err == nil {
		t.Fatal("expected an error for a profile with no required or ignored terms")
	}
	if len(*seen) != 0 {
		t.Errorf("upstream contacted %d times for a profile with no terms", len(*seen))
	}
}

func TestUpdateReleaseProfilePreservesUnknownUpstreamKeys(t *testing.T) {
	srv, seen := routedService(t, map[string]string{
		"/api/v3/releaseprofile/2": `{"id":2,"name":"No x265","enabled":true,
		  "required":[],"ignored":["x265"],"indexerId":0,"tags":[],
		  "someFutureSetting":"must survive"}`,
	})

	enabled := false
	if _, err := UpdateReleaseProfile(context.Background(), sonarrClient(srv.URL), ReleaseProfileUpdate{
		ID: 2, Enabled: &enabled,
	}); err != nil {
		t.Fatalf("UpdateReleaseProfile: %v", err)
	}

	find(t, *seen, http.MethodGet, "/api/v3/releaseprofile/2")
	body := bodyOf(t, find(t, *seen, http.MethodPut, "/api/v3/releaseprofile/2"))
	if body["enabled"] != false {
		t.Errorf("enabled = %v, want false", body["enabled"])
	}
	if body["someFutureSetting"] != "must survive" {
		t.Errorf("the read-modify-write dropped a key it does not model: %v", body)
	}
	ignored, ok := body["ignored"].([]any)
	if !ok || len(ignored) != 1 {
		t.Errorf("ignored = %v, want the stored terms left alone", body["ignored"])
	}
}

func TestDeleteReleaseProfileCallsTheReleaseProfileRoute(t *testing.T) {
	srv, seen := routedService(t, map[string]string{"/api/v3/releaseprofile/2": `{}`})

	if err := DeleteReleaseProfile(context.Background(), sonarrClient(srv.URL), 2); err != nil {
		t.Fatalf("DeleteReleaseProfile: %v", err)
	}
	find(t, *seen, http.MethodDelete, "/api/v3/releaseprofile/2")
}
