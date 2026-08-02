package discourse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchPublishReactAndTypedWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Api-Key") != "api-key" || request.Header.Get("Api-Username") != "system" || request.Header.Get("Accept") != "application/json" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /u/system.json":
			writeJSON(writer, http.StatusOK, `{"user":{"id":7,"username":"system","name":"System User","avatar_template":"/user_avatar/forum/system/{size}/1.png","created_at":"2020-01-01T00:00:00Z","trust_level":4,"admin":true,"post_count":20,"topic_count":3}}`)
		case "GET /posts/10.json":
			writeJSON(writer, http.StatusOK, postFixture(10, 9, 1, "Root post"))
		case "GET /posts/11.json":
			writeJSON(writer, http.StatusOK, postFixture(11, 9, 2, "Parent reply"))
		case "GET /posts/10/replies.json":
			writeJSON(writer, http.StatusOK, `[`+postFixture(11, 9, 2, "First reply")+`,`+postFixture(12, 9, 3, "Second reply")+`]`)
		case "GET /posts.json":
			if request.URL.Query().Get("before") != "21" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"latest_posts":[`+postFixture(20, 9, 4, "Latest")+`,`+postFixture(18, 8, 3, "Older")+`]}`)
		case "GET /t/9.json":
			writeJSON(writer, http.StatusOK, `{"id":9,"title":"A topic","slug":"a-topic","category_id":2,"posts_count":2,"reply_count":1,"views":100,"like_count":5,"created_at":"2026-08-01T10:00:00Z","last_posted_at":"2026-08-02T10:00:00Z","visible":true,"closed":false,"archived":false,"archetype":"regular","post_stream":{"posts":[`+postFixture(10, 9, 1, "Root post")+`],"stream":[10,11]}}`)
		case "POST /posts.json":
			var input createPostPayload
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch {
			case input.TopicID == 9 && input.ReplyToPostNumber > 0 && input.Raw != "":
				writeJSON(writer, http.StatusOK, postFixture(30, 9, input.ReplyToPostNumber+1, input.Raw))
			case input.Title == "New topic" && input.Category == 2:
				writeJSON(writer, http.StatusOK, postFixture(31, 10, 1, input.Raw))
			case input.Archetype == "private_message" && input.TargetRecipients == "alice,bob":
				writeJSON(writer, http.StatusOK, postFixture(32, 11, 1, input.Raw))
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		case "POST /post_actions.json":
			var input postActionPayload
			_ = json.NewDecoder(request.Body).Decode(&input)
			if input.ID != 10 || input.PostActionTypeID != 2 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, postFixture(10, 9, 1, "Root post"))
		case "DELETE /posts/30.json":
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "7" || user.Username == nil || *user.Username != "system" || user.DisplayName == nil || *user.DisplayName != "System User" || user.AccountType == nil || *user.AccountType != "admin" || user.AvatarURL == nil || *user.AvatarURL != server.URL+"/user_avatar/forum/system/120/1.png" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "10")
	if err != nil || post.ID != "10" || post.AuthorID == nil || *post.AuthorID != "7" || post.Text == nil || *post.Text != "Root post" || post.URL == nil || post.Status.State != socialhub.PublishStatePublished || len(post.Metrics) != 3 || post.Metrics[1].Value != 4 || len(post.Extensions["discourse.post"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "11" || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "10" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	latest, err := client.ListLatestPosts(context.Background(), "21")
	if err != nil || len(latest.Items) != 2 || latest.NextCursor == nil || *latest.NextCursor != "18" || !latest.HasMore {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	topic, err := client.GetTopic(context.Background(), "9")
	if err != nil || topic.ID != "9" || topic.Title != "A topic" || topic.CategoryID != "2" || len(topic.Posts) != 1 || len(topic.PostIDs) != 2 || topic.PostIDs[1] != "11" || len(topic.Raw) == 0 {
		t.Fatalf("topic=%#v err=%v", topic, err)
	}

	text, target := "A reply", "10"
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &target})
	if err != nil || published.ID != "30" || published.Text == nil || *published.Text != text {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	status, err := client.PublishStatus(context.Background(), "10")
	if err != nil || status.ID != "10" || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	parent := "11"
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "10", ParentID: &parent, Text: "Nested"})
	if err != nil || comment.ID != "30" || comment.PostID != "10" || comment.ParentID == nil || *comment.ParentID != "11" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeletePost(context.Background(), "30"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), "30"); err != nil {
		t.Fatal(err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "system", TargetID: "10", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove reaction=%v", err)
	}

	createdTopic, err := client.CreateTopic(context.Background(), CreateTopicRequest{Title: "New topic", Raw: "Body", CategoryID: "2"})
	if err != nil || createdTopic.ID != "10" || createdTopic.Posts[0].ID != "31" || createdTopic.PostIDs[0] != "31" {
		t.Fatalf("created topic=%#v err=%v", createdTopic, err)
	}
	message, err := client.CreatePrivateMessage(context.Background(), CreatePrivateMessageRequest{
		Title: "Hello", Raw: "Private body", Recipients: []string{"alice", "bob", "alice"},
	})
	if err != nil || message.TopicID != "11" || len(message.Recipients) != 2 || message.FirstPost.ID != "32" {
		t.Fatalf("message=%#v err=%v", message, err)
	}
}

func TestWorkflowValidationAndBadResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/posts/10.json":
			writeJSON(writer, http.StatusOK, postFixture(99, 9, 1, "wrong"))
		case "/u/system.json":
			writeJSON(writer, http.StatusOK, `{"user":{}}`)
		case "/posts/10/replies.json":
			writeJSON(writer, http.StatusOK, `[{"id":0}]`)
		case "/posts.json":
			writeJSON(writer, http.StatusOK, `{"latest_posts":[{"id":0}]}`)
		case "/t/9.json":
			writeJSON(writer, http.StatusOK, `{"id":8}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)

	invalidCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "bad/user"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10", Cursor: "1"})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10", MaxResults: -1})
			return err
		},
		func() error { _, err := client.ListLatestPosts(context.Background(), "bad"); return err },
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{})
			return err
		},
		func() error {
			text := "text"
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
			return err
		},
		func() error {
			text, target := "text", "10"
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &target, MediaIDs: []string{"1"}})
			return err
		},
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{})
			return err
		},
		func() error {
			parent := "bad"
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "10", ParentID: &parent, Text: "x"})
			return err
		},
		func() error { return client.DeletePost(context.Background(), "bad") },
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "other", TargetID: "10", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "10", Kind: socialhub.ReactionRepost})
		},
		func() error { _, err := client.CreateTopic(context.Background(), CreateTopicRequest{}); return err },
		func() error {
			_, err := client.CreateTopic(context.Background(), CreateTopicRequest{Title: "x", Raw: "x", CategoryID: "bad"})
			return err
		},
		func() error { _, err := client.GetTopic(context.Background(), "bad"); return err },
		func() error {
			_, err := client.CreatePrivateMessage(context.Background(), CreatePrivateMessageRequest{})
			return err
		},
		func() error {
			_, err := client.CreatePrivateMessage(context.Background(), CreatePrivateMessageRequest{Title: "x", Raw: "x", Recipients: []string{"bad/user"}})
			return err
		},
	}
	for index, call := range invalidCalls {
		err := call()
		if !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}

	badCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), ""); return err },
		func() error { _, err := client.GetPost(context.Background(), "10"); return err },
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "10"})
			return err
		},
		func() error { _, err := client.ListLatestPosts(context.Background(), ""); return err },
		func() error { _, err := client.GetTopic(context.Background(), "9"); return err },
	}
	for index, call := range badCalls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}
}

func postFixture(id, topicID int64, postNumber int, raw string) string {
	return `{"id":` + strconv.FormatInt(id, 10) + `,"name":"System User","username":"system","avatar_template":"/avatar/{size}.png","created_at":"2026-08-02T01:00:00Z","updated_at":"2026-08-02T02:00:00Z","raw":` + strconv.Quote(raw) + `,"cooked":"<p>` + raw + `</p>","post_number":` + strconv.Itoa(postNumber) + `,"reply_count":2,"reads":12,"readers_count":10,"topic_id":` + strconv.FormatInt(topicID, 10) + `,"topic_slug":"topic","topic_title":"Topic","user_id":7,"post_url":"/t/topic/` + strconv.FormatInt(topicID, 10) + `/` + strconv.Itoa(postNumber) + `","actions_summary":[{"id":2,"count":4,"acted":true}]}`
}

func TestTypeDecodeFailuresAndMappingHelpers(t *testing.T) {
	for index, target := range []json.Unmarshaler{&discoursePost{}, &discourseUser{}, &discourseUpload{}, &topicResponse{}} {
		if err := target.UnmarshalJSON([]byte(`{`)); err == nil {
			t.Fatalf("decoder %d accepted invalid JSON", index)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	deleted := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	mapped := client.mapPost(discoursePost{ID: 1, DeletedAt: &deleted, Raw: json.RawMessage(`{"id":1}`)})
	if mapped.Status.State != socialhub.PublishStateFailed || mapped.Text != nil || likeCount(nil) != 0 || stringPointer("") != nil || client.absoluteURL("") != nil {
		t.Fatalf("mapping helpers=%#v", mapped)
	}
	if intPointer(2) == nil {
		t.Fatal("pointer helpers failed")
	}
}
