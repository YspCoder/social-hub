package bluesky

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

const (
	parentURI       = "at://did:plc:bob/app.bsky.feed.post/parent1"
	createdURI      = "at://did:plc:alice/app.bsky.feed.post/custom-key"
	commonURI       = "at://did:plc:alice/app.bsky.feed.post/common1"
	commentURI      = "at://did:plc:alice/app.bsky.feed.post/comment1"
	existingLikeURI = "at://did:plc:alice/app.bsky.feed.like/existing-like"
	reactionTarget  = "at://did:plc:bob/app.bsky.feed.post/reaction1"
	freshTarget     = "at://did:plc:bob/app.bsky.feed.post/reaction2"
)

func TestPublishRecordsInteractionsAndDeletion(t *testing.T) {
	var createPostCalls atomic.Int32
	var createLikeCalls atomic.Int32
	var createRepostCalls atomic.Int32
	var deleteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/xrpc/app.bsky.feed.getPosts":
			uri := request.URL.Query().Get("uris")
			var view map[string]any
			switch uri {
			case parentURI:
				view = testPostView(parentURI, "cid-parent", "did:plc:bob", "bob.test", "parent")
				view["record"].(map[string]any)["reply"] = map[string]any{
					"root":   map[string]string{"uri": rootURI, "cid": "cid-root"},
					"parent": map[string]string{"uri": rootURI, "cid": "cid-root"},
				}
			case quoteURI:
				view = testPostView(quoteURI, "cid-quote", "did:plc:eve", "eve.test", "quoted")
			case createdURI:
				view = testPostView(createdURI, "cid-created", "did:plc:alice", "alice.test", "typed post")
			case reactionTarget:
				view = testPostView(reactionTarget, "cid-reaction", "did:plc:bob", "bob.test", "target")
				view["viewer"] = map[string]string{"like": existingLikeURI}
			case freshTarget:
				view = testPostView(freshTarget, "cid-fresh", "did:plc:bob", "bob.test", "fresh target")
				viewer := map[string]string{}
				if createLikeCalls.Load() > 0 {
					viewer["like"] = "at://did:plc:alice/app.bsky.feed.like/like1"
				}
				if createRepostCalls.Load() > 0 {
					viewer["repost"] = "at://did:plc:alice/app.bsky.feed.repost/repost1"
				}
				view["viewer"] = viewer
			default:
				writeTestJSON(t, writer, map[string]any{"posts": []any{}})
				return
			}
			writeTestJSON(t, writer, map[string]any{"posts": []any{view}})
		case "/xrpc/com.atproto.repo.createRecord":
			if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			var input struct {
				Repo       string         `json:"repo"`
				Collection string         `json:"collection"`
				RecordKey  string         `json:"rkey"`
				Record     map[string]any `json:"record"`
			}
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.Repo != "did:plc:alice" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch input.Collection {
			case collectionPost:
				createPostCalls.Add(1)
				text, _ := input.Record["text"].(string)
				switch text {
				case "typed post":
					if input.RecordKey != "custom-key" || input.Record["$type"] != collectionPost || input.Record["createdAt"] != testNow.Format("2006-01-02T15:04:05.999999999Z07:00") {
						writer.WriteHeader(http.StatusBadRequest)
						return
					}
					languages, _ := input.Record["langs"].([]any)
					reply, _ := input.Record["reply"].(map[string]any)
					root, _ := reply["root"].(map[string]any)
					parent, _ := reply["parent"].(map[string]any)
					embed, _ := input.Record["embed"].(map[string]any)
					if len(languages) != 2 || root["uri"] != rootURI || root["cid"] != "cid-root" || parent["uri"] != parentURI || parent["cid"] != "cid-parent" || embed["$type"] != "app.bsky.embed.recordWithMedia" {
						writer.WriteHeader(http.StatusBadRequest)
						return
					}
					writeTestJSON(t, writer, map[string]string{"uri": createdURI, "cid": "cid-created", "validationStatus": "valid"})
				case "common post":
					writeTestJSON(t, writer, map[string]string{"uri": commonURI, "cid": "cid-common"})
				case "comment text":
					reply, _ := input.Record["reply"].(map[string]any)
					parent, _ := reply["parent"].(map[string]any)
					if parent["uri"] != parentURI {
						writer.WriteHeader(http.StatusBadRequest)
						return
					}
					writeTestJSON(t, writer, map[string]string{"uri": commentURI, "cid": "cid-comment"})
				default:
					writer.WriteHeader(http.StatusBadRequest)
				}
			case collectionLike:
				createLikeCalls.Add(1)
				writeTestJSON(t, writer, map[string]string{"uri": "at://did:plc:alice/app.bsky.feed.like/like1", "cid": "cid-like"})
			case collectionRepost:
				createRepostCalls.Add(1)
				writeTestJSON(t, writer, map[string]string{"uri": "at://did:plc:alice/app.bsky.feed.repost/repost1", "cid": "cid-repost"})
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		case "/xrpc/com.atproto.repo.deleteRecord":
			var input deleteRecordRequest
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.Repo != "did:plc:alice" || input.RecordKey == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			deleteCalls.Add(1)
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	client.blobs["bafk-image"] = blobRef{
		Type: "blob", Ref: blobLink{Link: "bafk-image"}, MIMEType: "image/jpeg", Size: 123,
	}

	created, err := client.CreateRecord(context.Background(), PostRecordRequest{
		Text: "typed post", Languages: []string{"en", "zh-Hans"},
		Media:      []PostMedia{{MediaID: "bafk-image", Alt: "description", Width: 1200, Height: 800}},
		ReplyToURI: parentURI, QuoteURI: quoteURI, RecordKey: "custom-key",
	})
	if err != nil || created.ID != createdURI || created.Text == nil || *created.Text != "typed post" || len(created.Media) != 1 || created.Media[0].Width == nil || *created.Media[0].Width != 1200 || !hasRelation(*created, socialhub.RelationReply, parentURI) || !hasRelation(*created, socialhub.RelationQuote, quoteURI) {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	visibility, text := "public", "common post"
	common, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"bafk-image"}, Visibility: &visibility})
	if err != nil || common.ID != commonURI || len(common.Media) != 1 {
		t.Fatalf("common post=%#v error=%v", common, err)
	}
	status, err := client.PublishStatus(context.Background(), createdURI)
	if err != nil || status.ID != createdURI || status.State != socialhub.PublishStatePublished || status.UpdatedAt == nil {
		t.Fatalf("status=%#v error=%v", status, err)
	}

	existing := socialhub.ReactionRequest{ActorID: "did:plc:alice", TargetID: reactionTarget, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	if createLikeCalls.Load() != 0 {
		t.Fatal("existing like should be idempotent")
	}
	if err := client.RemoveReaction(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	freshLike := socialhub.ReactionRequest{TargetID: freshTarget, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), freshLike); err != nil {
		t.Fatal(err)
	}
	freshRepost := socialhub.ReactionRequest{TargetID: freshTarget, Kind: socialhub.ReactionRepost}
	if err := client.React(context.Background(), freshRepost); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), freshRepost); err != nil {
		t.Fatal(err)
	}
	if createLikeCalls.Load() != 1 || createRepostCalls.Load() != 1 {
		t.Fatalf("reaction creates like=%d repost=%d", createLikeCalls.Load(), createRepostCalls.Load())
	}

	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: rootURI, ParentID: stringPointer(parentURI), Text: "comment text"})
	if err != nil || comment.ID != commentURI || comment.PostID != rootURI || comment.ParentID == nil || *comment.ParentID != parentURI {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), commentURI); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), createdURI); err != nil {
		t.Fatal(err)
	}
	if createPostCalls.Load() != 3 || deleteCalls.Load() != 4 {
		t.Fatalf("post creates=%d deletes=%d", createPostCalls.Load(), deleteCalls.Load())
	}
}

func TestPublishValidationAndMediaShapes(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	client.blobs["image"] = blobRef{Type: "blob", Ref: blobLink{Link: "image"}, MIMEType: "image/png", Size: 1}
	client.blobs["video"] = blobRef{Type: "blob", Ref: blobLink{Link: "video"}, MIMEType: "video/mp4", Size: 1}

	videoEmbedValue, media, err := client.buildMediaEmbed([]PostMedia{{MediaID: "video", Alt: "clip", Width: 9, Height: 16}})
	if err != nil || len(media) != 1 || media[0].Type != socialhub.MediaTypeVideo {
		t.Fatalf("video embed=%#v media=%#v error=%v", videoEmbedValue, media, err)
	}
	if _, ok := videoEmbedValue.(videoEmbed); !ok {
		t.Fatalf("video embed type=%T", videoEmbedValue)
	}
	if _, _, err := client.buildMediaEmbed([]PostMedia{{MediaID: "video"}, {MediaID: "image"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("mixed media error=%v", err)
	}
	imageEmbedValue, media, err := client.buildMediaEmbed([]PostMedia{{MediaID: "image"}})
	if err != nil || len(media) != 1 {
		t.Fatalf("image embed=%#v media=%#v error=%v", imageEmbedValue, media, err)
	}
	if _, ok := imageEmbedValue.(imageEmbed); !ok {
		t.Fatalf("image embed type=%T", imageEmbedValue)
	}
	if got := combineEmbeds(nil, &strongRef{URI: quoteURI, CID: "cid"}); got == nil {
		t.Fatal("quote embed should be created")
	}

	private, text := "private", "text"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty publish error=%v", err)
	}
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &private}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("visibility error=%v", err)
	}
	invalid := []PostRecordRequest{
		{},
		{Text: "x", Languages: []string{"en", "zh", "fr", "de"}},
		{Text: "x", Languages: []string{"bad tag"}},
		{Text: "x", Media: []PostMedia{{MediaID: "1"}, {MediaID: "2"}, {MediaID: "3"}, {MediaID: "4"}, {MediaID: "5"}}},
		{Text: "x", Media: []PostMedia{{}}},
		{Text: "x", Media: []PostMedia{{MediaID: "image", Width: 10}}},
		{Text: "x", RecordKey: "invalid+key"},
		{Text: "x", ReplyToURI: "bad"},
		{Text: "x", QuoteURI: "at://did:plc:eve/app.bsky.feed.like/like1"},
	}
	for _, input := range invalid {
		if err := validatePostRecord(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if err := client.DeletePost(context.Background(), "at://did:plc:other/app.bsky.feed.post/one"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("foreign delete error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "did:plc:other", TargetID: freshTarget, Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("actor error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("target error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: freshTarget, Kind: "bookmark"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reaction kind error=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment validation error=%v", err)
	}
	empty := " "
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: rootURI, ParentID: &empty, Text: "x"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment parent error=%v", err)
	}
	if !validRecordKey("pre:fix_~1.2-3") || validRecordKey(".") || validRecordKey("alpha/beta") || validRecordKey("bad@key") || validRecordKey(strings.Repeat("a", 513)) {
		t.Fatal("record-key validation does not match AT Protocol syntax")
	}
}
