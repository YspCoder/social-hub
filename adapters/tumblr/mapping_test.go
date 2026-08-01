package tumblr

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestLegacyAndMappingEdgeCases(t *testing.T) {
	legacy := tumblrPost{ID: "501", BlogName: "legacy", Title: "Title", Body: "Full body", Summary: "Short summary", State: "draft"}
	mapped := mapPost("main", legacy, testNow)
	if mapped.Text == nil || *mapped.Text != "Title\n\nFull body" || mapped.Status == nil || mapped.Status.State != socialhub.PublishStatePending || mapped.Status.UpdatedAt == nil || !mapped.Status.UpdatedAt.Equal(testNow) {
		t.Fatalf("legacy post=%#v", mapped)
	}
	legacy.Title, legacy.Body = "", ""
	if text := postText(legacy); text != "Short summary" {
		t.Fatalf("summary fallback=%q", text)
	}
	posts := mapPosts("main", []tumblrPost{{}, legacy}, testNow)
	if len(posts) != 1 || posts[0].ID != "501" {
		t.Fatalf("filtered posts=%#v", posts)
	}

	content := []json.RawMessage{
		json.RawMessage(`{"type":"video","url":"https://cdn.test/video.mp4"}`),
		json.RawMessage(`{"type":"image","media":"bad"}`),
		json.RawMessage(`{"type":"link","url":"https://example.test"}`),
		json.RawMessage(`not-json`),
	}
	media := mapPostMedia(content)
	if len(media) != 1 || media[0].Type != socialhub.MediaTypeVideo || media[0].URL != "https://cdn.test/video.mp4" {
		t.Fatalf("fallback media=%#v", media)
	}
	if decodeMediaObjects(nil) != nil || decodeMediaObjects(json.RawMessage(`bad`)) != nil {
		t.Fatal("invalid media decoded")
	}
	if unixPointer(0) != nil || unixFloatPointer(0) != nil || unixFloatPointer(math.Inf(1)) != nil || unixFloatPointer(math.NaN()) != nil || stringPointer(" ") != nil {
		t.Fatal("invalid pointer helper accepted")
	}
	value := unixFloatPointer(100.25)
	want := time.Unix(100, int64(250*time.Millisecond)).UTC()
	if value == nil || !value.Equal(want) {
		t.Fatalf("fractional timestamp=%v", value)
	}
}
