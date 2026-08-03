package micropub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCommonPublishPreservesEndpointQueryAndMapsPendingReply(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/micropub" || request.URL.Query().Get("tenant") != "one" || request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("X-Request-ID") != "request-1" {
			t.Errorf("request=%s headers=%v", request.URL, request.Header)
		}
		if contentType := request.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
			t.Errorf("content type=%q", contentType)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("h") != "entry" || request.Form.Get("content") != "hello" || request.Form.Get("in-reply-to") != server.URL+"/parent" {
			t.Errorf("form=%v", request.Form)
		}
		writer.Header().Set("Location", server.URL+"/posts/1")
		writer.Header().Set("Link", `<https://sho.rt/1>; rel="shortlink", <https://social.example/1>; rel="syndication"`)
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"create"}, true, true, false)
	text, reply, visibility := "hello", server.URL+"/parent", "public"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &reply, Visibility: &visibility}, socialhub.WithRequestID("request-1"))
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != server.URL+"/posts/1" || post.Status == nil || post.Status.State != socialhub.PublishStatePending || post.Text == nil || *post.Text != text || len(post.Relations) != 1 || post.Relations[0].PostID != reply || len(post.Extensions["micropub.shortlinks"]) == 0 || len(post.Extensions["micropub.syndication"]) == 0 {
		t.Fatalf("post=%#v", post)
	}
}

func TestTypedCreateEntryJSONAndResponseLinks(t *testing.T) {
	published := time.Date(2026, 8, 1, 12, 30, 0, 0, time.FixedZone("PDT", -7*60*60))
	var received createPayload
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("tenant") != "one" || request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Idempotency-Key") != "entry-key" {
			t.Errorf("request=%s headers=%v", request.URL, request.Header)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Location", "https://site.example/posts/2")
		writer.Header().Set("Link", `<https://sho.rt/2>; rel="shortlink", <https://copy.example/2>; rel="syndication", <not a url>; rel="shortlink"`)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, true, true)
	result, err := client.CreateEntry(context.Background(), EntryCreateRequest{
		Name: "Article", Summary: "Summary", Content: Content{HTML: "<p>Hello</p>"}, Published: &published,
		Categories: []string{"indieweb", "go"}, Location: "geo:1,2", InReplyTo: []string{"https://source.example/post"},
		LikeOf: []string{"https://liked.example/post"}, RepostOf: []string{"https://repost.example/post"},
		Photos: []Photo{{Value: "https://media.example/photo.jpg", Alt: "Sunset"}, {Value: "https://media.example/plain.jpg"}},
		Videos: []string{"https://media.example/video.mp4"}, Audios: []string{"https://media.example/audio.mp3"},
		SyndicateTo: []string{"target-one"}, ExtraProperties: map[string][]json.RawMessage{"mood": {json.RawMessage(`"happy"`)}},
	}, socialhub.WithIdempotencyKey("entry-key"))
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://site.example/posts/2" || result.State != socialhub.PublishStatePublished || !slices.Equal(result.Shortlinks, []string{"https://sho.rt/2"}) || !slices.Equal(result.Syndication, []string{"https://copy.example/2"}) {
		t.Fatalf("result=%#v", result)
	}
	if !slices.Equal(received.Type, []string{"h-entry"}) || sourceText(received.Properties["content"][0]) != "<p>Hello</p>" || len(received.Properties["photo"]) != 2 || len(received.Properties["category"]) != 2 || len(received.Properties["mood"]) != 1 || len(received.Properties["mp-syndicate-to"]) != 1 {
		t.Fatalf("payload=%#v", received)
	}
	var publishedValue string
	if json.Unmarshal(received.Properties["published"][0], &publishedValue) != nil || publishedValue != published.Format(time.RFC3339) {
		t.Fatalf("published=%q", publishedValue)
	}
}

func TestUpdateDeleteUndeleteContracts(t *testing.T) {
	actions := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var action string
		_ = json.Unmarshal(payload["action"], &action)
		actions = append(actions, action)
		switch action {
		case "update":
			var replace map[string][]json.RawMessage
			var deleteValues map[string][]json.RawMessage
			_ = json.Unmarshal(payload["replace"], &replace)
			_ = json.Unmarshal(payload["delete"], &deleteValues)
			if sourceText(replace["content"][0]) != "updated" || sourceText(deleteValues["category"][0]) != "old" {
				t.Errorf("payload=%s", mustRaw(payload))
			}
			writer.WriteHeader(http.StatusNoContent)
		case "delete":
			writer.WriteHeader(http.StatusOK)
		case "undelete":
			writer.Header().Set("Location", "https://site.example/posts/restored")
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("action=%q", action)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"create", "update", "delete"}, true, true, true)
	postURL := "https://site.example/posts/3"
	updated, err := client.UpdateEntry(context.Background(), EntryUpdateRequest{
		URL: postURL, Replace: map[string][]json.RawMessage{"content": {json.RawMessage(`"updated"`)}},
		DeleteValues: map[string][]json.RawMessage{"category": {json.RawMessage(`"old"`)}},
	})
	if err != nil || updated.URL != postURL {
		t.Fatalf("update=%#v err=%v", updated, err)
	}
	deleted, err := client.DeleteEntry(context.Background(), postURL)
	if err != nil || deleted.URL != postURL {
		t.Fatalf("delete=%#v err=%v", deleted, err)
	}
	restored, err := client.UndeleteEntry(context.Background(), postURL)
	if err != nil || restored.URL != "https://site.example/posts/restored" {
		t.Fatalf("undelete=%#v err=%v", restored, err)
	}
	if !slices.Equal(actions, []string{"update", "delete", "undelete"}) {
		t.Fatalf("actions=%v", actions)
	}
}

func TestSourceFetcherAndPublishStatusMapping(t *testing.T) {
	postURL := "https://site.example/posts/4"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("tenant") != "one" || query.Get("q") != "source" || query.Get("url") != postURL {
			t.Errorf("query=%v", query)
		}
		if len(query["properties[]"]) > 0 && !slices.Equal(query["properties[]"], []string{"content", "published"}) {
			t.Errorf("properties=%v", query["properties[]"])
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"type":["h-entry"],"properties":{"content":[{"html":"<p>Hello</p>"}],"published":["2026-08-01T12:00:00Z"],"photo":[{"value":"https://media.example/photo.jpg","alt":"Alt"}],"video":["https://media.example/video.mp4"],"audio":["https://media.example/audio.mp3"],"in-reply-to":["https://source.example/post"],"repost-of":["https://source.example/repost"]}}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, []string{"update"}, true, false, false)
	entry, err := client.Source(context.Background(), postURL, []string{"content", "published"})
	if err != nil || len(entry.Raw) == 0 || len(entry.Properties) == 0 {
		t.Fatalf("entry=%#v err=%v", entry, err)
	}
	post, err := client.GetPost(context.Background(), postURL)
	if err != nil {
		t.Fatal(err)
	}
	if post.Text == nil || *post.Text != "<p>Hello</p>" || post.CreatedAt == nil || len(post.Media) != 3 || len(post.Relations) != 2 || len(post.Extensions["source"]) == 0 {
		t.Fatalf("post=%#v", post)
	}
	status, err := client.PublishStatus(context.Background(), postURL)
	if err != nil || status.State != socialhub.PublishStatePublished || status.UpdatedAt == nil || !status.UpdatedAt.Equal(testNow) {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != server.URL+"/" || user.ProfileURL == nil || *user.ProfileURL != server.URL+"/" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if _, err := client.GetUser(context.Background(), "someone"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list posts error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments error=%v", err)
	}
}

func TestCreateUpdateAndCommonPublishValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, nil, true, true, true)
	text, empty, reply, quote, private := "hello", "", "/relative", "https://quote.example/post", "private"
	common := []socialhub.CreatePostRequest{
		{}, {Text: &empty}, {Text: &text, MediaIDs: []string{"https://media.example/photo"}},
		{Text: &text, ReplyToID: &reply}, {Text: &text, QuotePostID: &quote}, {Text: &text, Visibility: &private},
	}
	for index, input := range common {
		if _, err := client.Publish(context.Background(), input); err == nil {
			t.Fatalf("common[%d] accepted", index)
		}
	}
	create := []EntryCreateRequest{
		{}, {Types: []string{"entry"}, Content: Content{Text: "x"}}, {Content: Content{Text: "x", HTML: "<b>x</b>"}},
		{Photos: []Photo{{Value: "/relative"}}}, {Categories: []string{""}},
		{ExtraProperties: map[string][]json.RawMessage{"content": {json.RawMessage(`"x"`)}}},
		{ExtraProperties: map[string][]json.RawMessage{"mp-slug": {json.RawMessage(`"x"`)}}},
		{ExtraProperties: map[string][]json.RawMessage{"mood": {json.RawMessage(`bad`)}}},
	}
	for index, input := range create {
		if _, err := client.CreateEntry(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("create[%d] error=%v", index, err)
		}
	}
	update := []EntryUpdateRequest{
		{}, {URL: "/relative", Add: map[string][]json.RawMessage{"category": {json.RawMessage(`"x"`)}}},
		{URL: "https://site.example/post"},
		{URL: "https://site.example/post", DeleteProperties: []string{"category"}, DeleteValues: map[string][]json.RawMessage{"category": {json.RawMessage(`"x"`)}}},
		{URL: "https://site.example/post", DeleteProperties: []string{"category", "category"}},
		{URL: "https://site.example/post", Replace: map[string][]json.RawMessage{"action": {json.RawMessage(`"x"`)}}},
	}
	for index, input := range update {
		if _, err := client.UpdateEntry(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("update[%d] error=%v", index, err)
		}
	}
}

func TestNonConformingSuccessResponses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		location string
		create   bool
	}{
		{"create status", http.StatusOK, "https://site.example/post", true},
		{"create location", http.StatusCreated, "", true},
		{"create relative", http.StatusCreated, "/post", true},
		{"update status", http.StatusAccepted, "", false},
		{"update location", http.StatusCreated, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := response{Status: test.status, Header: http.Header{}}
			if test.location != "" {
				result.Header.Set("Location", test.location)
			}
			if _, err := entryResult("test", result, "https://site.example/current", test.create); errorCode(err) != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestQueryEncodingKeepsBracketPropertyNames(t *testing.T) {
	values := url.Values{"properties[]": {"content", "published"}}
	if encoded := values.Encode(); !strings.Contains(encoded, "properties%5B%5D=content") {
		t.Fatalf("encoded=%q", encoded)
	}
}
