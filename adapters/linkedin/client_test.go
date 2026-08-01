package linkedin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestLinkedInPostFetchAndReactionContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Linkedin-Version") != "202607" || request.Header.Get("X-Restli-Protocol-Version") != "2.0.0" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/userinfo":
			writeJSON(writer, `{"sub":"member-1","name":"Ada Lovelace","picture":"https://cdn.example/avatar.jpg","email":"ada@example.com","email_verified":true}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/posts":
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["author"] != "urn:li:organization:123" || body["lifecycleState"] != "PUBLISHED" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("X-RestLi-Id", "urn%3Ali%3Ashare%3Apost-1")
			writer.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && request.URL.Path == "/rest/posts/urn:li:share:post-1":
			writeJSON(writer, `{"id":"urn:li:share:post-1","author":"urn:li:organization:123","commentary":"hello","visibility":"PUBLIC","lifecycleState":"PUBLISHED","publishedAt":1785542400000,"content":{"media":{"id":"urn:li:image:image-1","title":"image"}}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/rest/posts":
			if request.URL.Query().Get("q") != "author" || request.URL.Query().Get("author") != "urn:li:organization:123" || request.URL.Query().Get("start") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"elements":[{"id":"urn:li:share:post-2","author":"urn:li:organization:123","commentary":"next","lifecycleState":"PUBLISHED"}],"paging":{"start":2,"count":1,"total":4,"links":[{"rel":"next","href":"next"}]}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/posts/urn:li:share:post-1":
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/rest/socialActions/urn:li:share:post-1/comments":
			writeJSON(writer, `{"elements":[{"id":77,"commentUrn":"urn:li:comment:(urn:li:activity:1,77)","actor":"urn:li:person:member-2","message":{"text":"nice"},"created":{"time":1785542401000}}],"paging":{"start":0,"count":1,"total":1}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/socialActions/urn:li:share:post-1/comments":
			var body struct {
				Actor   string `json:"actor"`
				Object  string `json:"object"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Actor != "urn:li:organization:123" || body.Object != "urn:li:share:post-1" || body.Message.Text != "thanks" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"78","actor":"urn:li:organization:123","message":{"text":"thanks"}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/rest/reactions":
			if request.URL.Query().Get("actor") != "urn:li:organization:123" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodDelete && request.URL.Path == "/rest/reactions/(actor:urn:li:organization:123,entity:urn:li:share:post-1)":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"openid", "profile", "r_organization_social", "w_organization_social"})

	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != "urn:li:person:member-1" || user.DisplayName == nil || *user.DisplayName != "Ada Lovelace" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	text := "hello"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"urn:li:image:image-1"}})
	if err != nil || post.ID != "urn:li:share:post-1" || len(post.Media) != 1 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	fetched, err := client.GetPost(context.Background(), post.ID)
	if err != nil || fetched.Text == nil || *fetched.Text != "hello" || fetched.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("fetched=%#v err=%v", fetched, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "3" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: post.ID})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "77" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: post.ID, Text: "thanks"})
	if err != nil || comment.ID != "78" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "urn:li:organization:123", TargetID: post.ID, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete comment error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}
