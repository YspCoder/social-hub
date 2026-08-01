package tumblr

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestNPFPublishEditFetchAndMultipart(t *testing.T) {
	multipartSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/blog/example.tumblr.com/posts":
			if strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data") {
				if err := request.ParseMultipartForm(1 << 20); err != nil {
					http.Error(writer, err.Error(), http.StatusBadRequest)
					return
				}
				var payload npfPayload
				if err := json.Unmarshal([]byte(request.FormValue("json")), &payload); err != nil || len(payload.Content) != 2 {
					http.Error(writer, "bad multipart JSON", http.StatusBadRequest)
					return
				}
				var block map[string]json.RawMessage
				_ = json.Unmarshal(payload.Content[1], &block)
				var media map[string]string
				_ = json.Unmarshal(block["media"], &media)
				file, header, err := request.FormFile("1")
				if err != nil {
					http.Error(writer, "missing file", http.StatusBadRequest)
					return
				}
				defer file.Close()
				data, _ := io.ReadAll(file)
				if media["identifier"] != "1" || media["type"] != "image/png" || header.Filename != "image.png" || string(data) != "PNG" {
					http.Error(writer, "bad file", http.StatusBadRequest)
					return
				}
				multipartSeen = true
				writeEnvelope(t, writer, map[string]any{"id": "403"})
				return
			}
			var payload npfPayload
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || len(payload.Content) == 0 {
				http.Error(writer, "bad NPF", http.StatusBadRequest)
				return
			}
			id := "401"
			if payload.ParentPostID == "300" && payload.ParentTumblelogUUID == "t:source" && payload.ReblogKey == "rk-source" {
				id = "402"
			}
			writeEnvelope(t, writer, map[string]any{"id": id})
		case request.Method == http.MethodPut && request.URL.Path == "/blog/example.tumblr.com/posts/404":
			var payload npfPayload
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.State != NPFDraft || len(payload.Content) != 1 {
				http.Error(writer, "bad edit", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{"id": 404})
		case request.Method == http.MethodGet && request.URL.Path == "/blog/example.tumblr.com/posts/404":
			if request.URL.Query().Get("post_format") != "npf" {
				http.Error(writer, "bad format", http.StatusBadRequest)
				return
			}
			post := testPost("404", 1_754_046_400)
			post["blog"] = map[string]any{"uuid": "t:example", "name": "example"}
			post["reblog_key"] = "rk-404"
			post["tags"] = []string{"go", "sdk"}
			post["layout"] = []any{map[string]any{"type": "rows", "display": []any{map[string]any{"blocks": []int{0}}}}}
			post["trail"] = []any{map[string]any{"blog": map[string]any{"name": "source"}}}
			post["queued_state"] = ""
			post["publish_on"] = ""
			post["interactability_reblog"] = "everyone"
			writeEnvelope(t, writer, post)
		case request.Method == http.MethodPost && request.URL.Path == "/blog/example.tumblr.com/post/delete":
			var body map[string]string
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["id"] != "404" {
				http.Error(writer, "bad delete", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true, []string{"basic", "write"})

	text := "hello from common publisher"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil || post.ID != "401" || post.Text == nil || *post.Text != text || post.Status == nil || post.Status.State != socialhub.PublishStatePublished || post.CreatedAt == nil || !post.CreatedAt.Equal(testNow) {
		t.Fatalf("published=%#v error=%v", post, err)
	}
	textBlock := json.RawMessage(`{"type":"text","text":"reblog commentary"}`)
	reblog, err := client.CreateNPF(context.Background(), "", NPFPostRequest{
		Content: []json.RawMessage{textBlock}, State: NPFQueue, PublishOn: timePointer(testNow.Add(time.Hour)), Date: timePointer(testNow.Add(-time.Hour)),
		Tags: []string{"go", "sdk"}, SourceURL: "https://source.test/post", InteractabilityReblog: "everyone",
		Reblog: &NPFReblogTarget{BlogUUID: "t:source", PostID: "300", ReblogKey: "rk-source", ExcludeTrailItems: []int{0}},
	})
	if err != nil || reblog.ID != "402" || reblog.State != NPFQueue {
		t.Fatalf("reblog=%#v error=%v", reblog, err)
	}
	imageBlock := json.RawMessage(`{"type":"image"}`)
	uploaded, err := client.CreateNPF(context.Background(), "", NPFPostRequest{
		Content: []json.RawMessage{textBlock, imageBlock},
		Uploads: []NPFMediaUpload{{BlockIndex: 1, Filename: " image.png ", MIME: " image/png ", Size: 3, Reader: strings.NewReader("PNG")}},
	})
	if err != nil || uploaded.ID != "403" || !multipartSeen {
		t.Fatalf("multipart=%#v seen=%v error=%v", uploaded, multipartSeen, err)
	}
	edited, err := client.EditNPF(context.Background(), "", "404", NPFPostRequest{Content: []json.RawMessage{textBlock}, State: NPFDraft})
	if err != nil || edited.ID != "404" || edited.State != NPFDraft {
		t.Fatalf("edited=%#v error=%v", edited, err)
	}
	npfPost, err := client.GetNPF(context.Background(), "", "404")
	if err != nil || npfPost.ID != "404" || npfPost.BlogUUID != "t:example" || npfPost.BlogName != "example" || npfPost.ReblogKey != "rk-404" || len(npfPost.Content) != 4 || len(npfPost.Layout) != 1 || len(npfPost.Trail) != 1 || len(npfPost.Raw) == 0 {
		t.Fatalf("NPF post=%#v error=%v", npfPost, err)
	}
	status, err := client.PublishStatus(context.Background(), "404")
	if err != nil || status.ID != "404" || status.State != socialhub.PublishStatePublished || status.UpdatedAt == nil {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), "404"); err != nil {
		t.Fatal(err)
	}
}

func TestNPFAndCommonPublishValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, []string{"basic", "write"})
	text, blank, private, id := "text", " ", "private", "1"
	commonInvalid := []socialhub.CreatePostRequest{
		{}, {Text: &blank}, {Text: &text, MediaIDs: []string{"media"}}, {Text: &text, ReplyToID: &id}, {Text: &text, QuotePostID: &id}, {Text: &text, Visibility: &private},
	}
	for index, input := range commonInvalid {
		if _, err := client.Publish(context.Background(), input); err == nil {
			t.Fatalf("common validation %d accepted", index)
		}
	}
	if err := client.DeletePost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad delete=%v", err)
	}
	if _, err := client.EditNPF(context.Background(), "", "bad", NPFPostRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad edit=%v", err)
	}
	if _, err := client.GetNPF(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad fetch=%v", err)
	}

	validText := json.RawMessage(`{"type":"text","text":"ok"}`)
	image := json.RawMessage(`{"type":"image"}`)
	video := json.RawMessage(`{"type":"video"}`)
	future, past := testNow.Add(time.Hour), testNow.Add(-time.Hour)
	tooManyBlocks := make([]json.RawMessage, 1001)
	tooManyImages := make([]json.RawMessage, 31)
	for index := range tooManyImages {
		tooManyImages[index] = image
	}
	largeBlock, _ := json.Marshal(map[string]string{"type": "custom", "data": strings.Repeat("x", maxNPFJSONBytes)})
	cases := []NPFPostRequest{
		{}, {Content: tooManyBlocks}, {Content: []json.RawMessage{validText}, State: "bad"},
		{Content: []json.RawMessage{validText}, State: NPFPublished, PublishOn: &future},
		{Content: []json.RawMessage{validText}, State: NPFQueue, PublishOn: &past},
		{Content: []json.RawMessage{validText}, Date: &future},
		{Content: []json.RawMessage{validText}, SourceURL: "ftp://source.test"},
		{Content: []json.RawMessage{validText}, InteractabilityReblog: "friends"},
		{Content: []json.RawMessage{json.RawMessage(`[]`)}},
		{Content: []json.RawMessage{json.RawMessage(`{"type":"text","text":"` + strings.Repeat("x", 4097) + `"}`)}},
		{Content: tooManyImages}, {Content: []json.RawMessage{validText}, Layout: []json.RawMessage{json.RawMessage(`[]`)}},
		{Content: []json.RawMessage{image}, Uploads: []NPFMediaUpload{{BlockIndex: 0}}},
		{Content: []json.RawMessage{image}, Uploads: []NPFMediaUpload{{BlockIndex: 0, Filename: "a", MIME: "image/png", Size: 1, Reader: strings.NewReader("a")}, {BlockIndex: 0, Filename: "b", MIME: "image/png", Size: 1, Reader: strings.NewReader("b")}}},
		{Content: []json.RawMessage{validText}, Uploads: []NPFMediaUpload{{BlockIndex: 0, Filename: "a", MIME: "text/plain", Size: 1, Reader: strings.NewReader("a")}}},
		{Content: []json.RawMessage{video, video}, Uploads: []NPFMediaUpload{{BlockIndex: 0, Filename: "a", MIME: "video/mp4", Size: 1, Reader: strings.NewReader("a")}, {BlockIndex: 1, Filename: "b", MIME: "video/mp4", Size: 1, Reader: strings.NewReader("b")}}},
		{Content: []json.RawMessage{validText}, Reblog: &NPFReblogTarget{PostID: "1", ReblogKey: "key"}},
		{Content: []json.RawMessage{validText}, Reblog: &NPFReblogTarget{BlogUUID: "t:id", PostID: "1", ReblogKey: "key", HideTrail: true, ExcludeTrailItems: []int{0}}},
		{Content: []json.RawMessage{largeBlock}},
	}
	for index, input := range cases {
		if _, _, err := client.prepareNPF(input, false); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("NPF validation %d=%v", index, err)
		}
	}
	if _, _, err := client.prepareNPF(NPFPostRequest{}, true); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty edit=%v", err)
	}

	_, public := newTestAdapter(t, server, false, []string{"basic", "write"})
	if _, err := public.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public publish=%v", err)
	}
	if _, err := public.GetNPF(context.Background(), "", "1"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public NPF fetch=%v", err)
	}
	_, limited := newTestAdapter(t, server, true, []string{"basic"})
	if _, err := limited.CreateNPF(context.Background(), "", NPFPostRequest{Content: []json.RawMessage{validText}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("NPF scope=%v", err)
	}
	if err := limited.DeletePost(context.Background(), "1"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("delete scope=%v", err)
	}
}

func TestNPFHelpers(t *testing.T) {
	if blockType, text, err := validateContentBlock(json.RawMessage(`{"type":"text","text":"hello"}`)); err != nil || blockType != "text" || text != "hello" {
		t.Fatalf("block=%q %q error=%v", blockType, text, err)
	}
	patched, err := withMediaIdentifier(json.RawMessage(`{"type":"image"}`), "0", "image/png")
	if err != nil || !strings.Contains(string(patched), `"identifier":"0"`) {
		t.Fatalf("patched=%s error=%v", patched, err)
	}
	if jsonObject(json.RawMessage(`[]`)) || !jsonObject(json.RawMessage(`{"ok":true}`)) || validToken("bad token") || !validToken("good_token-1") {
		t.Fatal("JSON/token helper mismatch")
	}
	if validReblogTarget(NPFReblogTarget{BlogUUID: "t:id", PostID: "1", ReblogKey: "key", ExcludeTrailItems: []int{-1}}) || validReblogTarget(NPFReblogTarget{BlogUUID: "t:id", PostID: "1", ReblogKey: "key", ExcludeTrailItems: []int{1, 1}}) {
		t.Fatal("invalid reblog target accepted")
	}
	if !validReblogTarget(NPFReblogTarget{BlogUUID: "t:id", PostID: "1", ReblogKey: "key"}) || !validNPFState(NPFPrivate) || validNPFState("bad") {
		t.Fatal("valid NPF helper rejected")
	}
	if got := escapeQuotes("a\\\"\r\nb"); strings.ContainsAny(got, "\r\n") || !strings.Contains(got, `\\`) {
		t.Fatalf("escaped filename=%q", got)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
