package bluesky

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	rootURI   = "at://did:plc:bob/app.bsky.feed.post/root1"
	replyURI  = "at://did:plc:carol/app.bsky.feed.post/reply1"
	nestedURI = "at://did:plc:dave/app.bsky.feed.post/reply2"
	quoteURI  = "at://did:plc:eve/app.bsky.feed.post/quote1"
)

func testPostView(uri, cid, did, handle, text string) map[string]any {
	return map[string]any{
		"uri": uri, "cid": cid,
		"author":     map[string]any{"did": did, "handle": handle, "displayName": handle},
		"record":     map[string]any{"$type": collectionPost, "text": text, "createdAt": "2026-08-01T10:00:00Z"},
		"indexedAt":  "2026-08-01T10:00:01Z",
		"replyCount": 2, "repostCount": 3, "likeCount": 4, "quoteCount": 5, "bookmarkCount": 6,
	}
}

func TestFetchProfilesPostsFeedsAndThreads(t *testing.T) {
	root := testPostView(rootURI, "cid-root", "did:plc:bob", "bob.test", "root text")
	root["record"].(map[string]any)["reply"] = map[string]any{
		"root":   map[string]string{"uri": rootURI, "cid": "cid-root"},
		"parent": map[string]string{"uri": quoteURI, "cid": "cid-quote"},
	}
	root["embed"] = map[string]any{
		"$type":  "app.bsky.embed.recordWithMedia#view",
		"record": map[string]any{"record": map[string]any{"uri": quoteURI}},
		"media": map[string]any{
			"$type": "app.bsky.embed.images#view",
			"images": []any{map[string]any{
				"thumb": "https://cdn.test/thumb.jpg", "fullsize": "https://cdn.test/full.jpg", "alt": "accessible image",
				"aspectRatio": map[string]int{"width": 1200, "height": 800},
			}},
		},
	}
	reply := testPostView(replyURI, "cid-reply", "did:plc:carol", "carol.test", "first reply")
	reply["record"].(map[string]any)["reply"] = map[string]any{
		"root":   map[string]string{"uri": rootURI, "cid": "cid-root"},
		"parent": map[string]string{"uri": rootURI, "cid": "cid-root"},
	}
	nested := testPostView(nestedURI, "cid-nested", "did:plc:dave", "dave.test", "nested reply")
	nested["record"].(map[string]any)["reply"] = map[string]any{
		"root":   map[string]string{"uri": rootURI, "cid": "cid-root"},
		"parent": map[string]string{"uri": replyURI, "cid": "cid-reply"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/xrpc/app.bsky.actor.getProfile":
			if request.URL.Query().Get("actor") != "did:plc:alice" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"did": "did:plc:alice", "handle": "alice.test", "displayName": "Alice", "description": "profile",
				"avatar": "https://cdn.test/avatar.jpg", "banner": "https://cdn.test/banner.jpg",
				"followersCount": 10, "followsCount": 4, "postsCount": 20, "createdAt": "2025-01-01T00:00:00Z",
			})
		case "/xrpc/app.bsky.feed.getPosts":
			if request.URL.Query().Get("uris") != rootURI {
				writeTestJSON(t, writer, map[string]any{"posts": []any{}})
				return
			}
			writeTestJSON(t, writer, map[string]any{"posts": []any{root}})
		case "/xrpc/app.bsky.feed.getAuthorFeed":
			query := request.URL.Query()
			if query.Get("actor") != "did:plc:bob" || query.Get("cursor") != "author-cursor" || query.Get("limit") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"cursor": "author-next",
				"feed": []any{map[string]any{
					"post": root,
					"reason": map[string]any{
						"$type": "app.bsky.feed.defs#reasonRepost", "by": map[string]any{"did": "did:plc:alice", "handle": "alice.test"},
						"uri": "at://did:plc:alice/app.bsky.feed.repost/repost1", "cid": "cid-repost", "indexedAt": "2026-08-01T11:00:00Z",
					},
				}},
			})
		case "/xrpc/app.bsky.feed.getTimeline":
			query := request.URL.Query()
			if query.Get("cursor") != "home-cursor" || query.Get("limit") != "20" || query.Get("algorithm") != "reverse-chronological" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"feed": []any{map[string]any{"post": root}}})
		case "/xrpc/app.bsky.feed.getPostThread":
			query := request.URL.Query()
			if query.Get("uri") != rootURI || query.Get("parentHeight") != "0" || query.Get("depth") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"thread": map[string]any{
				"$type": "app.bsky.feed.defs#threadViewPost", "post": root,
				"replies": []any{
					map[string]any{"$type": "app.bsky.feed.defs#blockedPost", "blocked": true},
					map[string]any{"$type": "app.bsky.feed.defs#threadViewPost", "post": reply, "replies": []any{map[string]any{"post": nested}}},
				},
			}})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "did:plc:alice" || user.Username == nil || *user.Username != "alice.test" || user.DisplayName == nil || *user.DisplayName != "Alice" || user.ProfileURL == nil || *user.ProfileURL != "https://bsky.app/profile/alice.test" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), rootURI)
	if err != nil || post.ID != rootURI || post.Text == nil || *post.Text != "root text" || post.URL == nil || *post.URL != "https://bsky.app/profile/bob.test/post/root1" || len(post.Media) != 1 || post.Media[0].Width == nil || *post.Media[0].Width != 1200 || !hasRelation(*post, socialhub.RelationReply, quoteURI) || !hasRelation(*post, socialhub.RelationQuote, quoteURI) || len(post.Metrics) != 5 {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	authorFeed, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "did:plc:bob", Cursor: "author-cursor", MaxResults: 500})
	if err != nil || len(authorFeed.Items) != 1 || authorFeed.NextCursor == nil || *authorFeed.NextCursor != "author-next" || !authorFeed.HasMore || authorFeed.Items[0].ID != "at://did:plc:alice/app.bsky.feed.repost/repost1" || !hasRelation(authorFeed.Items[0], socialhub.RelationRepost, rootURI) || authorFeed.Items[0].AuthorID == nil || *authorFeed.Items[0].AuthorID != "did:plc:alice" {
		t.Fatalf("author feed=%#v error=%v", authorFeed, err)
	}
	home, err := client.Home(context.Background(), TimelineRequest{Cursor: "home-cursor", MaxResults: 20, Algorithm: "reverse-chronological"})
	if err != nil || len(home.Items) != 1 || home.HasMore {
		t.Fatalf("home=%#v error=%v", home, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: rootURI, MaxResults: 2})
	if err != nil || len(comments.Items) != 2 || comments.Items[0].ID != replyURI || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != rootURI || comments.Items[1].ID != nestedURI || comments.Items[1].ParentID == nil || *comments.Items[1].ParentID != replyURI {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}
}

func TestFetchValidationAndMalformedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/xrpc/app.bsky.actor.getProfile":
			writeTestJSON(t, writer, map[string]string{"did": "did:plc:alice"})
		case "/xrpc/app.bsky.feed.getPosts":
			writeTestJSON(t, writer, map[string]any{"posts": []any{}})
		case "/xrpc/app.bsky.feed.getPostThread":
			writeTestJSON(t, writer, map[string]any{"thread": map[string]any{"notFound": true}})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if _, err := client.GetUser(context.Background(), "alice.test"); err == nil {
		t.Fatal("profile missing handle should fail")
	}
	if _, err := client.GetPost(context.Background(), rootURI); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing post error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "not-an-at-uri"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid post error=%v", err)
	}
	now := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time range error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: -1}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("author limit error=%v", err)
	}
	if _, err := client.Home(context.Background(), TimelineRequest{MaxResults: -1}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("home limit error=%v", err)
	}
	for _, request := range []socialhub.ListCommentsRequest{
		{PostID: "bad"},
		{PostID: rootURI, Cursor: "cursor"},
		{PostID: rootURI, MaxResults: -1},
	} {
		if _, err := client.ListComments(context.Background(), request); err == nil {
			t.Fatalf("comment request=%#v should fail", request)
		}
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: rootURI}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing thread error=%v", err)
	}
}

func TestEmbedAndMappingVariants(t *testing.T) {
	gallery, quote := mapEmbed(json.RawMessage(`{"$type":"app.bsky.embed.gallery#view","items":[{"thumbnail":"https://cdn.test/t.jpg","fullsize":"https://cdn.test/f.jpg","alt":"gallery","aspectRatio":{"width":3,"height":2}}]}`))
	if quote != "" || len(gallery) != 1 || gallery[0].ID != "https://cdn.test/f.jpg" || gallery[0].Height == nil || *gallery[0].Height != 2 {
		t.Fatalf("gallery=%#v quote=%q", gallery, quote)
	}
	video, quote := mapEmbed(json.RawMessage(`{"$type":"app.bsky.embed.video#view","cid":"bafk-video","playlist":"https://video.test/list.m3u8","thumbnail":"https://cdn.test/video.jpg","alt":"video"}`))
	if quote != "" || len(video) != 1 || video[0].Type != socialhub.MediaTypeVideo || video[0].URL != "https://video.test/list.m3u8" {
		t.Fatalf("video=%#v quote=%q", video, quote)
	}
	_, quote = mapEmbed(json.RawMessage(`{"$type":"app.bsky.embed.record#view","record":{"record":{"uri":"` + quoteURI + `"}}}`))
	if quote != quoteURI {
		t.Fatalf("nested quote=%q", quote)
	}
	if media, quote := mapEmbed(json.RawMessage(`{"$type":"unknown"}`)); media != nil || quote != "" {
		t.Fatalf("unknown embed=%#v/%q", media, quote)
	}
	if media, quote := mapEmbed(json.RawMessage(`not-json`)); media != nil || quote != "" {
		t.Fatalf("invalid embed=%#v/%q", media, quote)
	}
	if media, quote := mapEmbed(nil); media != nil || quote != "" {
		t.Fatalf("empty embed=%#v/%q", media, quote)
	}

	bad := bskyPostView{URI: rootURI, CID: "cid", Author: bskyActor{DID: "did:plc:bob"}, Record: json.RawMessage(`not-json`)}
	if _, err := mapPost("main", bad, testNow); err == nil {
		t.Fatal("invalid post record should fail")
	}
	bad.Record = json.RawMessage(`{"text":"ok"}`)
	bad.CID = ""
	if _, err := mapPost("main", bad, testNow); err == nil {
		t.Fatal("missing post CID should fail")
	}
	if _, err := mapComment("main", rootURI, bskyPostView{Record: json.RawMessage(`bad`)}); err == nil {
		t.Fatal("invalid comment record should fail")
	}
}

func hasRelation(post socialhub.Post, relationType socialhub.RelationType, target string) bool {
	for _, relation := range post.Relations {
		if relation.Type == relationType && relation.PostID == target {
			return true
		}
	}
	return false
}
