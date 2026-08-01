package wordpresscom

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPostAndReactionWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = request.ParseForm()
		switch request.URL.Path {
		case "/rest/v1.1/sites/123/posts/new":
			if request.PostForm.Get("content") == "common body" {
				if request.PostForm.Get("status") != "private" || request.PostForm.Get("featured_image") != "31" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
			} else if request.PostForm.Get("title") != "Typed" || request.PostForm.Get("status") != "future" || request.PostForm.Get("categories[]") != "Go" || request.PostForm.Get("tags[]") != "SDK" || request.PostForm.Get("discussion[comments_open]") != "true" || request.PostForm.Get("likes_enabled") != "false" || request.PostForm.Get("publicize") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, postJSON(40, request.PostForm.Get("status")))
		case "/rest/v1.1/sites/123/posts/40":
			if request.Method == http.MethodGet {
				writeJSON(writer, http.StatusOK, postJSON(40, "publish"))
				return
			}
			if request.PostForm.Get("excerpt") != "Updated" || request.PostForm.Get("categories") != "" || request.PostForm.Get("tags") != "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, postJSON(40, "draft"))
		case "/rest/v1.1/sites/123/posts/40/restore", "/rest/v1.1/sites/123/posts/40/delete":
			writeJSON(writer, http.StatusOK, postJSON(40, "publish"))
		case "/rest/v1.1/sites/123/posts/40/likes/new":
			writeJSON(writer, http.StatusOK, `{"success":true,"i_like":true,"site_ID":123,"post_ID":40}`)
		case "/rest/v1.1/sites/123/posts/40/likes/mine/delete":
			writeJSON(writer, http.StatusOK, `{"success":true,"i_like":false,"site_ID":123,"post_ID":40}`)
		case "/rest/v1.1/sites/123/posts/40/replies/new":
			if request.PostForm.Get("content") != "top-level" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"ID":50,"post":40,"author":{"ID":7},"raw_content":"top-level"}`)
		case "/rest/v1.1/sites/123/comments/50/replies/new":
			writeJSON(writer, http.StatusOK, `{"ID":51,"post":{"ID":40},"author":{"ID":7},"raw_content":"child"}`)
		case "/rest/v1.1/sites/123/comments/51/delete":
			writeJSON(writer, http.StatusOK, `{"ID":51}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"posts", "comments"})

	title, content, excerpt := "Typed", "Body", "Updated"
	status := PostFuture
	date := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	publicize, commentsOpen, likesEnabled := true, true, false
	post, err := client.CreatePost(context.Background(), PostWriteRequest{
		Title: &title, Content: &content, Status: &status, Date: &date, Categories: []string{"Go"}, Tags: []string{"SDK"},
		Publicize: &publicize, CommentsOpen: &commentsOpen, LikesEnabled: &likesEnabled,
	})
	if err != nil || post.ID != "40" || post.Status.State != socialhub.PublishStatePending {
		t.Fatalf("created=%#v err=%v", post, err)
	}
	updated, err := client.UpdatePost(context.Background(), "40", PostWriteRequest{Excerpt: &excerpt, Categories: []string{}, Tags: []string{}})
	if err != nil || updated.ID != "40" || updated.Status.Message != "draft" {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	mediaID, visibility, body := "31", "private", "common body"
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &body, MediaIDs: []string{mediaID}, Visibility: &visibility})
	if err != nil || published.ID != "40" || published.Visibility == nil || *published.Visibility != "private" {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	statusResult, err := client.PublishStatus(context.Background(), "40")
	if err != nil || statusResult.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", statusResult, err)
	}
	if restored, err := client.RestorePost(context.Background(), "40"); err != nil || restored.ID != "40" {
		t.Fatalf("restored=%#v err=%v", restored, err)
	}
	if err := client.DeletePost(context.Background(), "40"); err != nil {
		t.Fatal(err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "7", TargetID: "40", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "40", Text: "top-level"})
	if err != nil || comment.ID != "50" || comment.PostID != "40" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	parent := "50"
	reply, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "40", ParentID: &parent, Text: "child"})
	if err != nil || reply.ParentID == nil || *reply.ParentID != "50" {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := client.DeleteComment(context.Background(), "51"); err != nil {
		t.Fatal(err)
	}
}

func TestPostAndReactionValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"global"})
	text, badVisibility, relation := "text", "friends", "1"
	invalidStatus := PostStatus("invalid")
	future := PostFuture
	zero := time.Time{}
	badText := "bad\rtext"
	badID := "x"
	invalidCalls := []func() error{
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &relation})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"1", "2"}})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &badVisibility})
			return err
		},
		func() error { return client.DeletePost(context.Background(), "bad") },
		func() error { _, err := client.CreatePost(context.Background(), PostWriteRequest{}); return err },
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Status: &invalidStatus})
			return err
		},
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &badText})
			return err
		},
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &text, Status: &future})
			return err
		},
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &text, Date: &zero})
			return err
		},
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &text, FeaturedImageID: &badID})
			return err
		},
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &text, Tags: []string{"bad\nterm"}})
			return err
		},
		func() error {
			_, err := client.UpdatePost(context.Background(), "bad", PostWriteRequest{Content: &text})
			return err
		},
		func() error { _, err := client.RestorePost(context.Background(), "bad"); return err },
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "8", TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "7", TargetID: "bad", Kind: socialhub.ReactionRepost})
		},
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{})
			return err
		},
		func() error {
			parent := "bad"
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", ParentID: &parent, Text: "x"})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), "bad") },
	}
	for index, call := range invalidCalls {
		err := call()
		if !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	if _, err := postForm(PostWriteRequest{Content: &text}, false); err != nil {
		t.Fatalf("valid update form=%v", err)
	}
	if !validPostStatus(PostPrivate) || validPostStatus("bad") || !validText("ok") || validText("bad\x00") || validTerms([]string{""}) || !validTerms(nil) {
		t.Fatal("post helper contract failed")
	}
	values := url.Values{}
	setOptional(values, "x", nil)
	setOptional(values, "x", &text)
	if values.Get("x") != "text" {
		t.Fatalf("values=%v", values)
	}
}

func TestPostAndReactionBadResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"global"})
	text := "text"
	calls := []func() error{
		func() error {
			_, err := client.CreatePost(context.Background(), PostWriteRequest{Content: &text})
			return err
		},
		func() error {
			_, err := client.UpdatePost(context.Background(), "1", PostWriteRequest{Content: &text})
			return err
		},
		func() error { _, err := client.RestorePost(context.Background(), "1"); return err },
		func() error { return client.DeletePost(context.Background(), "1") },
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "1", Kind: socialhub.ReactionLike})
		},
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "1", Text: "x"})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), "1") },
	}
	for index, call := range calls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}
}
