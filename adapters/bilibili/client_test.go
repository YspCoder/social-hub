package bilibili

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

const archiveFixture = `{"resource_id":"BV1TEST","title":"fixture title","cover":"https://img/cover.jpg","tid":229,"no_reprint":1,"desc":"fixture description","tag":"social-hub,go","copyright":1,"video_info":{"cid":123,"filename":"video-file","duration":5,"share_url":"https://www.bilibili.com/video/BV1TEST","iframe_url":"player.bilibili.com/player.html?bvid=BV1TEST"},"addit_info":{"state":0,"state_desc":"开放浏览","reject_reason":""},"ctime":1785542400,"ptime":1785542460}`

func TestGetUserAndArchiveReads(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, ok := verifySignedRequest(request, nil)
		if !ok || len(body) != 0 {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/arcopen/fn/user/account/info":
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"name":"UP","face":"https://img/face.jpg","openid":"open-id-1"}}`))
		case "/arcopen/fn/archive/view":
			if request.URL.Query().Get("resource_id") != "BV1TEST" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":` + archiveFixture + `}`))
		case "/arcopen/fn/archive/viewlist":
			if request.URL.Query().Get("pn") != "1" || request.URL.Query().Get("ps") != "1" || request.URL.Query().Get("status") != "all" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"list":[` + archiveFixture + `],"page":{"pn":1,"ps":1,"total":2}}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	user, err := client.GetUser(context.Background(), "open-id-1")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "UP" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "BV1TEST")
	if err != nil || post.Status.State != socialhub.PublishStatePublished || post.URL == nil || *post.URL != "https://www.bilibili.com/video/BV1TEST" || len(post.Media) != 2 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "open-id-1", MaxResults: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "2" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
}

func TestDeleteArchiveUsesSignedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, ok := verifySignedRequest(request, nil)
		var payload map[string]string
		if !ok || json.Unmarshal(body, &payload) != nil || payload["resource_id"] != "BV1TEST" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"code":0,"message":"0"}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if err := client.DeletePost(context.Background(), "BV1TEST"); err != nil {
		t.Fatal(err)
	}
}

func TestMissingScopeReturnsApprovalRequired(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	client.scopes = []string{"USER_INFO"}
	_, err := client.GetPost(context.Background(), "BV1TEST")
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("error=%v", err)
	}
}

func verifySignedRequest(request *http.Request, contentMD5Override *string) ([]byte, bool) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, false
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	wantMD5 := ""
	if contentMD5Override != nil {
		wantMD5 = *contentMD5Override
	} else {
		sum := md5.Sum(body)
		wantMD5 = hex.EncodeToString(sum[:])
	}
	if request.Header.Get("Access-Token") != "access-token" || request.Header.Get("X-Bili-Content-Md5") != wantMD5 || request.Header.Get("X-Bili-Timestamp") != "1785542400" || request.Header.Get("Content-Type") == "" || request.Header.Get("Accept") != "application/json" {
		return body, false
	}
	if request.Header.Get("Authorization") != signatureFor(signedHeaders(request), "app-secret") {
		return body, false
	}
	return body, true
}
