package peertube

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

const testVideoUUID = "9c9de5e8-0a1e-484a-b099-e80766180a6d"

func TestFetchChannelsAndMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/api/v1/accounts/creator":
			writeJSON(writer, http.StatusOK, `{"id":7,"name":"creator","displayName":"Creator","description":"bio","host":"video.example","url":"/a/creator","avatars":[{"fileUrl":"/avatars/a.png"}],"followersCount":12}`)
		case "/api/v1/videos/" + testVideoUUID:
			writeJSON(writer, http.StatusOK, videoJSON())
		case "/api/v1/accounts/creator/videos":
			if request.URL.Query().Get("start") != "2" || request.URL.Query().Get("count") != "2" {
				t.Errorf("video query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"total":5,"data":[`+videoJSON()+`,`+secondVideoJSON()+`]}`)
		case "/api/v1/video-channels/channel/videos":
			if request.URL.Query().Get("sort") != "-views" || request.URL.Query().Get("search") != "federation" {
				t.Errorf("channel videos query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"total":1,"data":[`+videoJSON()+`]}`)
		case "/api/v1/videos/" + testVideoUUID + "/comment-threads":
			writeJSON(writer, http.StatusOK, `{"total":3,"totalNotDeletedComments":3,"data":[`+commentJSON(11, "Great video", "null")+`]}`)
		case "/api/v1/video-channels/channel":
			writeJSON(writer, http.StatusOK, channelJSON())
		case "/api/v1/video-channels":
			if request.URL.Query().Get("count") != "1" || request.URL.Query().Get("sort") != "-createdAt" {
				t.Errorf("channel query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"total":2,"data":[`+channelJSON()+`]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "7" || dereference(user.Username) != "creator@video.example" || dereference(user.AvatarURL) != server.URL+"/avatars/a.png" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), testVideoUUID)
	if err != nil || post.ID != testVideoUUID || len(post.Media) != 1 || post.Media[0].URL != server.URL+"/static/video-1080.mp4" || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	if post.AuthorID == nil || *post.AuthorID != "7" || len(post.Metrics) != 4 || dereference(post.Visibility) != "public" {
		t.Fatalf("post mapping=%#v", post)
	}

	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 2})
	if err != nil || len(posts.Items) != 2 || dereference(posts.NextCursor) != "4" || dereference(posts.PrevCursor) != "0" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	videos, err := client.ListVideos(context.Background(), VideoListRequest{ChannelHandle: "channel", MaxResults: 5, Sort: "-views", Search: "federation"})
	if err != nil || len(videos.Items) != 1 || videos.HasMore {
		t.Fatalf("videos=%#v err=%v", videos, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testVideoUUID, MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "11" || dereference(comments.NextCursor) != "1" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	channel, err := client.GetChannel(context.Background(), "channel")
	if err != nil || channel.ID != 3 || channel.Name != "channel" {
		t.Fatalf("channel=%#v err=%v", channel, err)
	}
	channels, err := client.ListChannels(context.Background(), ChannelListRequest{MaxResults: 1, Sort: "-createdAt"})
	if err != nil || len(channels.Items) != 1 || dereference(channels.NextCursor) != "1" {
		t.Fatalf("channels=%#v err=%v", channels, err)
	}
}

func TestRatingsAndCommentWorkflows(t *testing.T) {
	var ratings []string
	var rootComments, replies, deletes int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodPut && request.URL.Path == "/api/v1/videos/"+testVideoUUID+"/rate":
			var payload struct {
				Rating string `json:"rating"`
			}
			decodeJSON(t, request, &payload)
			ratings = append(ratings, payload.Rating)
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/videos/"+testVideoUUID+"/comment-threads":
			rootComments++
			writeJSON(writer, http.StatusOK, `{"comment":`+commentJSON(21, "root", "null")+`}`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/videos/"+testVideoUUID+"/comments/21":
			replies++
			writeJSON(writer, http.StatusOK, `{"comment":`+commentJSON(22, "reply", "21")+`}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/videos/"+testVideoUUID+"/comment-threads/21":
			writeJSON(writer, http.StatusOK, `{"comment":`+commentJSON(21, "root", "null")+`,"children":[{"comment":`+commentJSON(22, "reply", "21")+`}]}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/videos/"+testVideoUUID+"/comments/22":
			deletes++
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	reaction := socialhub.ReactionRequest{ActorID: "creator", TargetID: testVideoUUID, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ratings, []string{"like", "none"}) {
		t.Fatalf("ratings=%v", ratings)
	}
	root, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: testVideoUUID, Text: "root"})
	if err != nil || root.ID != "21" {
		t.Fatalf("root=%#v err=%v", root, err)
	}
	parent := "21"
	reply, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: testVideoUUID, ParentID: &parent, Text: "reply"})
	if err != nil || reply.ID != "22" || dereference(reply.ParentID) != "21" {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	thread, err := client.GetCommentThread(context.Background(), testVideoUUID, "21")
	if err != nil || thread.Comment.ID != 21 || len(thread.Children) != 1 {
		t.Fatalf("thread=%#v err=%v", thread, err)
	}
	if err := client.DeleteVideoComment(context.Background(), testVideoUUID, "22"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), "22"); errorCode(err) != socialhub.CodeUnsupported {
		t.Fatalf("common delete=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: testVideoUUID, Kind: socialhub.ReactionRepost}); errorCode(err) != socialhub.CodeUnsupported {
		t.Fatalf("repost=%v", err)
	}
	if rootComments != 1 || replies != 1 || deletes != 1 {
		t.Fatalf("root=%d replies=%d deletes=%d", rootComments, replies, deletes)
	}
}

func TestVideoUploadUpdateAndDelete(t *testing.T) {
	var uploaded, updated, deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/videos/upload":
			uploaded = true
			if err := request.ParseMultipartForm(2 << 20); err != nil {
				t.Errorf("parse upload: %v", err)
				return
			}
			file, header, err := request.FormFile("videofile")
			if err != nil {
				t.Errorf("video file: %v", err)
				return
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			if string(data) != "video-bytes" || header.Filename != "clip.mp4" || request.MultipartForm.Value["channelId"][0] != "3" || request.MultipartForm.Value["name"][0] != "Federated clip" || !slices.Equal(request.MultipartForm.Value["tags"], []string{"fediverse", "video"}) {
				t.Errorf("upload file=%q header=%#v values=%v", data, header, request.MultipartForm.Value)
			}
			writeJSON(writer, http.StatusOK, `{"video":{"id":42,"uuid":"`+testVideoUUID+`","shortUUID":"2y84q2MQUMWPbiEcxNXMgC"}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/api/v1/videos/"+testVideoUUID:
			updated = true
			mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
			if mediaType != "multipart/form-data" || request.ParseMultipartForm(1<<20) != nil {
				t.Errorf("update content type=%q", request.Header.Get("Content-Type"))
			}
			if request.MultipartForm.Value["name"][0] != "Updated clip" || request.MultipartForm.Value["privacy"][0] != "2" || request.MultipartForm.Value["downloadEnabled"][0] != "false" {
				t.Errorf("update values=%v", request.MultipartForm.Value)
			}
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/videos/"+testVideoUUID:
			deleted = true
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	privacy, category, commentsPolicy := 1, 15, 1
	description := "A PeerTube video"
	result, err := client.UploadVideo(context.Background(), UploadVideoRequest{
		Filename: "clip.mp4", MIME: "video/mp4", ChannelID: 3, Name: "Federated clip", Privacy: &privacy,
		Category: &category, Description: &description, Tags: []string{"fediverse", "video"}, CommentsPolicy: &commentsPolicy,
	}, strings.NewReader("video-bytes"))
	if err != nil || result.ID != 42 || result.UUID != testVideoUUID {
		t.Fatalf("upload=%#v err=%v", result, err)
	}
	name, updatePrivacy, download := "Updated clip", 2, false
	if err := client.UpdateVideo(context.Background(), testVideoUUID, UpdateVideoRequest{Name: &name, Privacy: &updatePrivacy, DownloadEnabled: &download}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteVideo(context.Background(), testVideoUUID); err != nil {
		t.Fatal(err)
	}
	if !uploaded || !updated || !deleted {
		t.Fatalf("uploaded=%t updated=%t deleted=%t", uploaded, updated, deleted)
	}
}

func TestCommonValidationBoundaries(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	start := testNow
	tests := []struct {
		name string
		call func() error
		code socialhub.ErrorCode
	}{
		{"user", func() error { _, err := client.GetUser(context.Background(), "bad/name"); return err }, socialhub.CodeInvalidArgument},
		{"video", func() error { _, err := client.GetVideo(context.Background(), "bad/id"); return err }, socialhub.CodeInvalidArgument},
		{"video filters", func() error {
			_, err := client.ListVideos(context.Background(), VideoListRequest{AccountName: "a", ChannelHandle: "b"})
			return err
		}, socialhub.CodeInvalidArgument},
		{"video sort", func() error {
			_, err := client.ListVideos(context.Background(), VideoListRequest{Sort: "random"})
			return err
		}, socialhub.CodeInvalidArgument},
		{"time range", func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start})
			return err
		}, socialhub.CodeUnsupported},
		{"comments", func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{})
			return err
		}, socialhub.CodeInvalidArgument},
		{"channel", func() error { _, err := client.GetChannel(context.Background(), "bad/name"); return err }, socialhub.CodeInvalidArgument},
		{"actor", func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "other", TargetID: testVideoUUID, Kind: socialhub.ReactionLike})
		}, socialhub.CodeInvalidArgument},
		{"comment", func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: testVideoUUID, Text: ""})
			return err
		}, socialhub.CodeInvalidArgument},
		{"delete comment", func() error { return client.DeleteVideoComment(context.Background(), "bad/id", "1") }, socialhub.CodeInvalidArgument},
		{"thread", func() error { _, err := client.GetCommentThread(context.Background(), "", "1"); return err }, socialhub.CodeInvalidArgument},
		{"upload", func() error {
			_, err := client.UploadVideo(context.Background(), UploadVideoRequest{}, bytes.NewReader(nil))
			return err
		}, socialhub.CodeInvalidArgument},
		{"update", func() error { return client.UpdateVideo(context.Background(), testVideoUUID, UpdateVideoRequest{}) }, socialhub.CodeInvalidArgument},
		{"delete", func() error { return client.DeleteVideo(context.Background(), "bad/id") }, socialhub.CodeInvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := errorCode(test.call()); code != test.code {
				t.Fatalf("code=%s want=%s", code, test.code)
			}
		})
	}
}

func videoJSON() string {
	return `{"id":42,"uuid":"` + testVideoUUID + `","shortUUID":"short-id","name":"PeerTube architecture","description":"Federated video","createdAt":"2026-08-01T01:00:00Z","publishedAt":"2026-08-01T02:00:00Z","updatedAt":"2026-08-01T03:00:00Z","duration":120,"views":100,"likes":8,"dislikes":1,"comments":3,"privacy":{"id":1,"label":"Public"},"state":{"id":1,"label":"Published"},"account":{"id":7,"name":"creator"},"channel":{"id":3,"name":"channel"},"files":[{"id":1,"fileUrl":"/static/video-720.mp4","size":100,"width":1280,"height":720},{"id":2,"fileUrl":"/static/video-1080.mp4","size":200,"width":1920,"height":1080}]}`
}

func secondVideoJSON() string {
	return `{"id":43,"uuid":"a65bc12f-9383-462e-81ae-8207e8b434ee","name":"Second video","createdAt":"2026-08-01T04:00:00Z","duration":30,"privacy":{"id":2},"state":{"id":2},"account":{"id":7,"name":"creator"}}`
}

func commentJSON(id int, text, parent string) string {
	return `{"id":` + strconv.Itoa(id) + `,"text":"` + text + `","threadId":11,"inReplyToCommentId":` + parent + `,"videoId":42,"createdAt":"2026-08-01T05:00:00Z","totalReplies":1,"account":{"id":7,"name":"creator"}}`
}

func channelJSON() string {
	return `{"id":3,"name":"channel","displayName":"Channel","host":"video.example","isLocal":true,"ownerAccount":{"id":7,"name":"creator"}}`
}

func decodeJSON(t *testing.T, request *http.Request, target any) {
	t.Helper()
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
