package mastodon

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMastodonContentAndInteractionContracts(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/accounts/verify_credentials":
			writeJSON(writer, accountJSON("account-1"))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/accounts/account-1":
			writeJSON(writer, accountJSON("account-1"))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/accounts/account-1/statuses":
			if request.URL.Query().Get("max_id") != "cursor" || request.URL.Query().Get("limit") != "40" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Link", "<"+server.URL+"/api/v1/accounts/account-1/statuses?max_id=next>; rel=\"next\", <"+server.URL+"/api/v1/accounts/account-1/statuses?since_id=previous>; rel=\"prev\"")
			writeJSON(writer, "["+statusJSON("status-2", "<p>page</p>")+"]")
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/statuses/status-1":
			writeJSON(writer, statusJSON("status-1", "<p>Hello <strong>Fediverse</strong></p>"))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/statuses/new-1":
			writeJSON(writer, statusJSON("new-1", "<p>new status</p>"))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/statuses/status-1/context":
			writeJSON(writer, `{"ancestors":[],"descendants":[`+statusJSON("comment-1", "<p>first</p>")+`,`+statusJSON("comment-2", "<p>second</p>")+`]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/timelines/home":
			if request.URL.Query().Get("max_id") != "home-cursor" || request.URL.Query().Get("limit") != "20" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Link", "<"+server.URL+"/api/v1/timelines/home?max_id=home-next>; rel=\"next\"")
			writeJSON(writer, "["+boostJSON()+"]")
		case request.Method == http.MethodGet && request.URL.Path == "/api/v2/instance":
			writeJSON(writer, `{"domain":"social.example","title":"Social Example","version":"4.5.0","source_url":"https://github.com/mastodon/mastodon","api_versions":{"mastodon":7},"configuration":{"statuses":{"max_characters":500,"max_media_attachments":4},"media_attachments":{"image_size_limit":16777216,"video_size_limit":103809024}}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/statuses":
			if request.ParseForm() != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch request.Form.Get("status") {
			case "new status":
				media := request.Form["media_ids[]"]
				if request.Header.Get("Idempotency-Key") != "publish-key" || len(media) != 2 || media[0] != "media-1" || media[1] != "media-2" || request.Form.Get("in_reply_to_id") != "parent-1" || request.Form.Get("quoted_status_id") != "quoted-1" || request.Form.Get("visibility") != "unlisted" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeJSON(writer, statusJSON("new-1", "<p>new status</p>"))
			case "comment text":
				if request.Form.Get("in_reply_to_id") != "status-1" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeJSON(writer, statusJSON("comment-1", "<p>comment text</p>"))
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		case request.Method == http.MethodDelete && (request.URL.Path == "/api/v1/statuses/new-1" || request.URL.Path == "/api/v1/statuses/comment-1"):
			if request.Header.Get("Content-Type") != "" || request.ContentLength > 0 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, statusJSON(strings.TrimPrefix(request.URL.Path, "/api/v1/statuses/"), "<p>deleted</p>"))
		case request.Method == http.MethodPost && isReactionPath(request.URL.Path):
			writeJSON(writer, statusJSON("status-1", "<p>reacted</p>"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, allTestScopes())

	credential, err := client.GetUser(context.Background(), "")
	if err != nil || credential.ID != "account-1" || credential.Username == nil || *credential.Username != "alice@social.example" || credential.AccountType == nil || *credential.AccountType != "bot" {
		t.Fatalf("credential=%#v error=%v", credential, err)
	}
	user, err := client.GetUser(context.Background(), "account-1")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Alice" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "status-1")
	if err != nil || post.Text == nil || *post.Text != "<p>Hello <strong>Fediverse</strong></p>" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeAnimation || post.Media[0].Duration == nil || *post.Media[0].Duration != 1500*time.Millisecond || len(post.Relations) != 2 {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor", MaxResults: 100})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next" || !page.HasMore {
		t.Fatalf("page=%#v error=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "status-1", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "root-1" {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}
	home, err := client.TimelineWorkflow().Home(context.Background(), TimelineRequest{Cursor: "home-cursor", MaxResults: 20})
	if err != nil || len(home.Items) != 1 || home.NextCursor == nil || *home.NextCursor != "home-next" || home.Items[0].Text == nil || *home.Items[0].Text != "<p>boosted</p>" || !hasRelation(home.Items[0], socialhub.RelationRepost, "original-1") {
		t.Fatalf("home=%#v error=%v", home, err)
	}
	instance, err := client.InstanceWorkflow().Instance(context.Background())
	if err != nil || instance.Domain != "social.example" || instance.MastodonAPIVersion != 7 || instance.MaxStatusCharacters != 500 || instance.VideoSizeLimit != 103809024 {
		t.Fatalf("instance=%#v error=%v", instance, err)
	}

	text, parent, quote, visibility := "new status", "parent-1", "quoted-1", "unlisted"
	created, err := client.Publish(context.Background(), socialhub.CreatePostRequest{
		Text: &text, MediaIDs: []string{"media-1", "media-2"}, ReplyToID: &parent, QuotePostID: &quote, Visibility: &visibility,
	}, socialhub.WithIdempotencyKey("publish-key"))
	if err != nil || created.ID != "new-1" {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	status, err := client.PublishStatus(context.Background(), created.ID)
	if err != nil || status.ID != "new-1" || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	for _, reaction := range []socialhub.ReactionRequest{
		{ActorID: "account-1", TargetID: "status-1", Kind: socialhub.ReactionLike},
		{ActorID: "account-1", TargetID: "status-1", Kind: socialhub.ReactionRepost},
	} {
		if err := client.React(context.Background(), reaction); err != nil {
			t.Fatal(err)
		}
		if err := client.RemoveReaction(context.Background(), reaction); err != nil {
			t.Fatal(err)
		}
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "status-1", Text: "comment text"})
	if err != nil || comment.ID != "comment-1" || comment.ParentID == nil || *comment.ParentID != "status-1" {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMastodonRequestValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, allTestScopes())
	now := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time range error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "one", Cursor: "cursor"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comment cursor error=%v", err)
	}
	if _, err := client.Home(context.Background(), TimelineRequest{MaxResults: -1}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("home limit error=%v", err)
	}
	text, visibility := "text", "followers"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &visibility}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("visibility error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "other", TargetID: "one", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("actor error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "account-1", TargetID: "one", Kind: "bookmark"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reaction error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("post ID error=%v", err)
	}
}

func accountJSON(id string) string {
	return `{"id":"` + id + `","username":"alice","acct":"alice@social.example","display_name":"Alice","bot":true,"discoverable":true,"created_at":"2025-01-01T00:00:00Z","note":"<p>profile</p>","url":"https://social.example/@alice","uri":"https://social.example/users/alice","avatar":"https://cdn.example/avatar.png","header":"https://cdn.example/header.png","followers_count":10,"following_count":4,"statuses_count":20,"last_status_at":"2026-08-01","fields":[]}`
}

func statusJSON(id, content string) string {
	return `{"id":"` + id + `","uri":"https://social.example/users/alice/statuses/` + id + `","url":"https://social.example/@alice/` + id + `","account":` + accountJSON("account-1") + `,"in_reply_to_id":"root-1","quoted_status_id":"quoted-1","content":"` + content + `","created_at":"2026-08-01T00:00:00Z","edited_at":"2026-08-01T00:01:00Z","replies_count":2,"reblogs_count":3,"favourites_count":4,"reblogged":false,"favourited":true,"bookmarked":false,"sensitive":false,"spoiler_text":"","visibility":"public","language":"en","media_attachments":[{"id":"media-1","type":"gifv","url":"https://cdn.example/animation.mp4","preview_url":"https://cdn.example/preview.jpg","description":"alt text","meta":{"original":{"width":640,"height":480,"duration":1.5}}}]}`
}

func boostJSON() string {
	return `{"id":"boost-1","uri":"https://social.example/users/alice/statuses/boost-1","url":"https://social.example/@alice/boost-1","account":` + accountJSON("account-1") + `,"content":"","created_at":"2026-08-01T00:00:00Z","visibility":"public","reblog":` + statusJSON("original-1", "<p>boosted</p>") + `,"media_attachments":[]}`
}

func isReactionPath(path string) bool {
	for _, suffix := range []string{"/favourite", "/unfavourite", "/reblog", "/unreblog"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func hasRelation(post socialhub.Post, relationType socialhub.RelationType, postID string) bool {
	for _, relation := range post.Relations {
		if relation.Type == relationType && relation.PostID == postID {
			return true
		}
	}
	return false
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(writer, value)
}
