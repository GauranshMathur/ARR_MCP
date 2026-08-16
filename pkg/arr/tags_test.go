package arr

import (
	"context"
	"testing"
)

func TestListTagsReturnsLabelsAndIDs(t *testing.T) {
	srv, got := fakeService(t, 200, `[{"label":"kids","id":3},{"label":"anime","id":4}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	tags, err := ListTags(context.Background(), c)
	if err != nil {
		t.Fatalf("ListTags returned error: %v", err)
	}
	if got.method != "GET" {
		t.Errorf("method = %q, want GET", got.method)
	}
	if got.path != "/api/v3/tag" {
		t.Errorf("path = %q, want /api/v3/tag", got.path)
	}
	if len(tags) != 2 || tags[0].Label != "kids" || tags[0].ID != 3 {
		t.Errorf("tags = %+v, want kids/3 first", tags)
	}
}

// tag/detail lists every tagged id, which is thousands of integers on a real
// library. Only the counts answer the question "is this tag still in use?".
func TestListTagDetailsCountsUsageInsteadOfListingIDs(t *testing.T) {
	srv, got := fakeService(t, 200, `[{
	  "id":1,"label":"kids",
	  "seriesIds":[5,6,7],"indexerIds":[1],"downloadClientIds":[],
	  "importListIds":[2,3],"notificationIds":[9],"restrictionIds":[4],
	  "delayProfileIds":[],"autoTagIds":[8]
	}]`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	details, err := ListTagDetails(context.Background(), c)
	if err != nil {
		t.Fatalf("ListTagDetails returned error: %v", err)
	}
	if got.path != "/api/v3/tag/detail" {
		t.Errorf("path = %q, want /api/v3/tag/detail", got.path)
	}
	if len(details) != 1 {
		t.Fatalf("details = %d, want 1", len(details))
	}
	d := details[0]
	if d.MediaCount != 3 || d.IndexerCount != 1 || d.ImportListCount != 2 {
		t.Errorf("detail = %+v, want media=3 indexer=1 importList=2", d)
	}
	if d.ReleaseProfileCount != 1 {
		t.Errorf("restrictionIds must count as release profiles, got %+v", d)
	}
}

// Radarr names the same fields movieIds and releaseProfileIds.
func TestListTagDetailsHandlesRadarrFieldNames(t *testing.T) {
	srv, _ := fakeService(t, 200, `[{"id":1,"label":"4k","movieIds":[1,2],"releaseProfileIds":[7]}]`)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	details, err := ListTagDetails(context.Background(), c)
	if err != nil {
		t.Fatalf("ListTagDetails returned error: %v", err)
	}
	if len(details) != 1 || details[0].MediaCount != 2 || details[0].ReleaseProfileCount != 1 {
		t.Errorf("details = %+v, want media=2 releaseProfile=1", details)
	}
}

func TestCreateTagPostsTheLabel(t *testing.T) {
	srv, got := fakeService(t, 201, `{"label":"kids","id":11}`)
	c := NewClient(srv.URL, SonarrSpec, Credentials{APIKey: "k"})

	tag, err := CreateTag(context.Background(), c, "kids")
	if err != nil {
		t.Fatalf("CreateTag returned error: %v", err)
	}
	if got.method != "POST" || got.path != "/api/v3/tag" {
		t.Errorf("request = %s %s, want POST /api/v3/tag", got.method, got.path)
	}
	if !contains(got.body, `"label":"kids"`) {
		t.Errorf("body = %q, want it to carry the label", got.body)
	}
	if tag.ID != 11 {
		t.Errorf("tag = %+v, want the created id 11", tag)
	}
}

func TestDeleteTagTargetsTheTagID(t *testing.T) {
	srv, got := fakeService(t, 200, ``)
	c := NewClient(srv.URL, RadarrSpec, Credentials{APIKey: "k"})

	if err := DeleteTag(context.Background(), c, 7); err != nil {
		t.Fatalf("DeleteTag returned error: %v", err)
	}
	if got.method != "DELETE" || got.path != "/api/v3/tag/7" {
		t.Errorf("request = %s %s, want DELETE /api/v3/tag/7", got.method, got.path)
	}
}
