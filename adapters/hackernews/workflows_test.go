package hackernews

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTypedAndCommonReadContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "social-hub-hackernews-tests/1.0" || request.Header.Get("Accept") != "application/json" {
			http.Error(writer, "bad headers", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/api/topstories.json":
			writeJSON(writer, http.StatusOK, `[101,102,103]`)
		case "/api/item/101.json":
			writeJSON(writer, http.StatusOK, `{"id":101,"type":"story","by":"alice","time":1700000000,"title":"A story","text":"<p>Body</p>","url":"https://example.test/story","score":42,"descendants":2,"kids":[201,202]}`)
		case "/api/item/102.json":
			writeJSON(writer, http.StatusOK, `{"id":102,"type":"job","by":"yc","time":1700000100,"title":"A job","score":1}`)
		case "/api/item/103.json":
			writeJSON(writer, http.StatusOK, `{"id":103,"type":"poll","by":"bob","time":1700000200,"title":"A poll","parts":[301],"score":7}`)
		case "/api/item/201.json":
			writeJSON(writer, http.StatusOK, `{"id":201,"type":"comment","by":"carol","time":1700000300,"parent":101,"text":"First","kids":[203]}`)
		case "/api/item/202.json":
			writeJSON(writer, http.StatusOK, `{"id":202,"type":"comment","by":"dave","time":1700000400,"parent":101,"text":"Second"}`)
		case "/api/item/203.json":
			writeJSON(writer, http.StatusOK, `{"id":203,"type":"comment","by":"erin","time":1700000500,"parent":201,"text":"Nested"}`)
		case "/api/item/301.json":
			writeJSON(writer, http.StatusOK, `{"id":301,"type":"pollopt","poll":103,"text":"Yes","score":3}`)
		case "/api/user/alice.json":
			writeJSON(writer, http.StatusOK, `{"id":"alice","created":1600000000,"karma":9001,"about":"<p>About</p>","submitted":[201,101,102]}`)
		case "/api/maxitem.json":
			writeJSON(writer, http.StatusOK, `999`)
		case "/api/updates.json":
			writeJSON(writer, http.StatusOK, `{"items":[101,201],"profiles":["alice","bob"]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	item, err := client.GetItem(context.Background(), 101)
	if err != nil || item.Title != "A story" || len(item.Kids) != 2 {
		t.Fatalf("item=%#v err=%v", item, err)
	}
	maxID, err := client.MaxItemID(context.Background())
	if err != nil || maxID != 999 {
		t.Fatalf("max ID=%d err=%v", maxID, err)
	}
	feed, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedTop, MaxResults: 2})
	if err != nil || len(feed.Items) != 2 || feed.NextCursor == nil || *feed.NextCursor != "2" || !feed.HasMore || feed.Items[1].Type != ItemJob {
		t.Fatalf("feed=%#v err=%v", feed, err)
	}
	last, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedTop, Cursor: "2", MaxResults: 2})
	if err != nil || len(last.Items) != 1 || last.NextCursor != nil || last.HasMore || last.Items[0].Type != ItemPoll {
		t.Fatalf("last feed=%#v err=%v", last, err)
	}
	children, err := client.ListChildren(context.Background(), ChildrenRequest{ParentID: 101, MaxResults: 1})
	if err != nil || len(children.Items) != 1 || children.Items[0].ID != 201 || children.NextCursor == nil {
		t.Fatalf("children=%#v err=%v", children, err)
	}
	nested, err := client.ListChildren(context.Background(), ChildrenRequest{ParentID: 201})
	if err != nil || len(nested.Items) != 1 || nested.Items[0].ID != 203 {
		t.Fatalf("nested=%#v err=%v", nested, err)
	}
	profile, err := client.GetUserProfile(context.Background(), "alice")
	if err != nil || profile.Karma != 9001 || len(profile.Submitted) != 3 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	updates, err := client.GetUpdates(context.Background())
	if err != nil || len(updates.Items) != 2 || len(updates.Profiles) != 2 {
		t.Fatalf("updates=%#v err=%v", updates, err)
	}

	user, err := client.GetUser(context.Background(), "alice")
	if err != nil || user.ID != "alice" || user.ProfileURL == nil || user.Extensions["hackernews.user"] == nil {
		t.Fatalf("common user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "101")
	if err != nil || post.Text == nil || *post.Text != "A story\n\n<p>Body</p>" || post.URL == nil ||
		len(post.Metrics) != 2 || !post.Metrics[0].AsOf.Equal(testNow) || post.Extensions["hackernews.item"] == nil {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 2})
	if err != nil || len(posts.Items) != 2 || posts.NextCursor == nil || posts.Items[1].ID != "102" {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	userPosts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "alice", MaxResults: 1})
	if err != nil || len(userPosts.Items) != 1 || userPosts.Items[0].ID != "101" || userPosts.NextCursor == nil || *userPosts.NextCursor != "2" {
		t.Fatalf("user posts=%#v err=%v", userPosts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "101", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "201" || comments.Items[0].AuthorID == nil ||
		comments.NextCursor == nil || comments.Items[0].Extensions["hackernews.item"] == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestNullResourcesAndUnsupportedItemKinds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/item/404.json", "/api/user/missing.json":
			writeJSON(writer, http.StatusOK, `null`)
		case "/api/item/201.json":
			writeJSON(writer, http.StatusOK, `{"id":201,"type":"comment","parent":101}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	if _, err := client.GetItem(context.Background(), 404); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing item=%v", err)
	}
	if _, err := client.GetUserProfile(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing user=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "201"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment as post=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "201"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment children as post comments=%v", err)
	}
}
