package flickr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type flickrFixture struct {
	t       *testing.T
	mu      sync.Mutex
	methods []string
}

func (fixture *flickrFixture) handler(writer http.ResponseWriter, request *http.Request) {
	fixture.t.Helper()
	if request.URL.Path != "/rest" {
		http.NotFound(writer, request)
		return
	}
	if request.Method == http.MethodPost {
		verifyOAuthSignature(fixture.t, request, "consumer-secret", "token-secret", false)
	} else if request.Header.Get("Authorization") != "" {
		verifyOAuthSignature(fixture.t, request, "consumer-secret", "token-secret", false)
	}
	if err := request.ParseForm(); err != nil {
		fixture.t.Errorf("ParseForm: %v", err)
		writeJSON(writer, http.StatusBadRequest, `{"stat":"fail","code":95,"message":"bad form"}`)
		return
	}
	method := request.Form.Get("method")
	fixture.mu.Lock()
	fixture.methods = append(fixture.methods, method)
	fixture.mu.Unlock()
	if request.Form.Get("api_key") != "api-key" || request.Form.Get("format") != "json" || request.Form.Get("nojsoncallback") != "1" {
		fixture.t.Errorf("common parameters=%v", request.Form)
	}

	switch method {
	case "flickr.people.getInfo":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","person":{"nsid":"owner@N01","ispro":1,"iconserver":"7","iconfarm":2,"username":{"_content":"alice"},"realname":{"_content":"Alice Example"},"location":{"_content":"Shanghai"},"photosurl":{"_content":"https://www.flickr.com/photos/owner@N01/"},"profileurl":{"_content":"https://www.flickr.com/people/owner@N01/"},"photos":{"firstdate":"1700000000","count":42}}}`)
	case "flickr.photos.getInfo":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photo":{"id":"photo-1","secret":"secret","server":"server-1","farm":2,"media":"video","isfavorite":true,"license":4,"safety_level":"1","rotation":0,"views":"17","owner":{"nsid":"owner@N01","username":"alice"},"title":{"_content":"Title"},"description":{"_content":"Description"},"visibility":{"ispublic":0,"isfriend":1,"isfamily":1},"dates":{"posted":"1700000000","lastupdate":1700000100},"comments":{"_content":"3"},"tags":{"tag":[{"id":"tag-1","raw":"raw tag","_content":"normalized"}]},"urls":{"url":[{"type":"photopage","_content":"https://www.flickr.com/photos/owner@N01/photo-1/"}]}}}`)
	case "flickr.people.getPhotos", "flickr.people.getPublicPhotos":
		if request.Form.Get("user_id") != "owner@N01" || request.Form.Get("page") != "2" || request.Form.Get("per_page") != "500" {
			fixture.t.Errorf("photo list parameters=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photos":{"page":2,"pages":3,"perpage":500,"total":"501","photo":[{"id":"photo-2","owner":"owner@N01","secret":"secret-2","server":"server-2","farm":1,"title":"Second","description":{"_content":"Summary"},"ispublic":1,"isfriend":0,"isfamily":0,"dateupload":"1700000200","lastupdate":"1700000300","ownername":"Alice","tags":"one two","views":9,"media":"photo","width_o":"640","height_o":480,"url_o":"https://example.test/original.jpg"}]}}`)
	case "flickr.photos.setMeta":
		if request.Method != http.MethodPost || request.Form.Get("photo_id") != "photo-1" || request.Form.Get("title") != "" || request.Form.Get("description") != "updated" {
			fixture.t.Errorf("setMeta request=%s %v", request.Method, request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok"}`)
	case "flickr.photos.delete":
		writeJSON(writer, http.StatusOK, `{"stat":"ok"}`)
	case "flickr.favorites.add", "flickr.favorites.remove":
		if request.Form.Get("photo_id") != "photo-1" {
			fixture.t.Errorf("favorite parameters=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok"}`)
	case "flickr.photos.comments.getList":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","comments":{"photo_id":"photo-1","comment":[{"id":"comment-1","author":"user-2","authorname":"Bob","datecreate":"1700000400","permalink":"https://example.test/c1","_content":"first"},{"id":"comment-2","author":"user-3","authorname":"Carol","datecreate":1700000500,"_content":"second"}]}}`)
	case "flickr.photos.comments.addComment":
		if request.Form.Get("photo_id") != "photo-1" || request.Form.Get("comment_text") != "new comment" {
			fixture.t.Errorf("comment parameters=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok","comment":{"id":"comment-new"}}`)
	case "flickr.photos.comments.deleteComment":
		writeJSON(writer, http.StatusOK, `{"stat":"ok"}`)
	case "flickr.photosets.getInfo":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photoset":{"id":"album-1","owner":"owner@N01","primary":"photo-1","photos":2,"videos":"1","count_views":"11","count_comments":3,"date_create":"1700000000","date_update":1700000100,"title":{"_content":"Album"},"description":{"_content":"Album description"}}}`)
	case "flickr.photosets.getList":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photosets":{"page":"1","pages":2,"perpage":1,"total":"2","photoset":[{"id":"album-1","owner":"owner@N01","primary":"photo-1","photos":"2","videos":1,"title":{"_content":"Album"}}]}}`)
	case "flickr.photosets.getPhotos":
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photoset":{"id":"album-1","owner":"owner@N01","page":2,"pages":"2","perpage":"1","total":2,"photo":[{"id":"photo-2","owner":"owner@N01","secret":"secret-2","server":"server-2","title":"Second","ispublic":1,"media":"photo"}]}}`)
	case "flickr.photosets.create":
		if request.Form.Get("title") != "New album" || request.Form.Get("primary_photo_id") != "photo-1" {
			fixture.t.Errorf("create album parameters=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photoset":{"id":"album-new","url":"https://www.flickr.com/photos/owner@N01/sets/album-new"}}`)
	case "flickr.photosets.addPhoto", "flickr.photosets.removePhoto":
		if request.Form.Get("photoset_id") != "album-1" || request.Form.Get("photo_id") != "photo-2" {
			fixture.t.Errorf("album membership parameters=%v", request.Form)
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok"}`)
	default:
		fixture.t.Errorf("unexpected Flickr method %q", method)
		writeJSON(writer, http.StatusBadRequest, `{"stat":"fail","code":95,"message":"unexpected method"}`)
	}
}

func TestPhotoFetchAndMutationWorkflows(t *testing.T) {
	fixture := &flickrFixture{t: t}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()

	user, err := client.GetUser(ctx, "owner@N01", socialhub.WithRequestID("req-1"))
	if err != nil || user.ID != "owner@N01" || user.DisplayName == nil || *user.DisplayName != "Alice Example" || user.AvatarURL == nil || *user.AvatarURL != "https://farm2.staticflickr.com/7/buddyicons/owner@N01.jpg" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(ctx, "photo-1")
	if err != nil || post.ID != "photo-1" || post.Text == nil || *post.Text != "Description" || post.Visibility == nil || *post.Visibility != "friends_and_family" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeVideo || len(post.Metrics) != 2 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	start, end := time.Unix(1600000000, 0), time.Unix(1800000000, 0)
	photos, err := client.ListPhotos(ctx, PhotoListRequest{Cursor: "2", MaxResults: 900, StartTime: &start, EndTime: &end, SafeSearch: 1, Privacy: 3})
	if err != nil || len(photos.Items) != 1 || photos.NextCursor == nil || *photos.NextCursor != "3" || photos.PrevCursor == nil || *photos.PrevCursor != "1" {
		t.Fatalf("photos=%#v err=%v", photos, err)
	}
	posts, err := client.ListPosts(ctx, socialhub.ListPostsRequest{UserID: "owner@N01", Cursor: "2", MaxResults: 500})
	if err != nil || len(posts.Items) != 1 || posts.Items[0].Media[0].Width == nil || *posts.Items[0].Media[0].Width != 640 || posts.Items[0].Media[0].URL != "https://example.test/original.jpg" {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	empty, description := "", "updated"
	if err := client.UpdatePhoto(ctx, "photo-1", UpdatePhotoRequest{Title: &empty, Description: &description}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePhoto(ctx, "photo-1"); err != nil {
		t.Fatal(err)
	}

	reaction := socialhub.ReactionRequest{ActorID: "owner@N01", TargetID: "photo-1", Kind: socialhub.ReactionLike}
	if err := client.React(ctx, reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(ctx, reaction); err != nil {
		t.Fatal(err)
	}
	comments, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: "photo-1", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].Text != "first" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	comment, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: "photo-1", Text: "new comment"})
	if err != nil || comment.ID != "comment-new" || comment.CreatedAt == nil || !comment.CreatedAt.Equal(testNow) {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(ctx, "comment-new"); err != nil {
		t.Fatal(err)
	}
}

func TestAlbumWorkflows(t *testing.T) {
	fixture := &flickrFixture{t: t}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()

	album, err := client.GetAlbum(ctx, "album-1", "")
	if err != nil || album.ID != "album-1" || album.Title.Text != "Album" {
		t.Fatalf("album=%#v err=%v", album, err)
	}
	albums, err := client.ListAlbums(ctx, AlbumListRequest{Cursor: "1", MaxResults: 1})
	if err != nil || len(albums.Items) != 1 || albums.NextCursor == nil || *albums.NextCursor != "2" {
		t.Fatalf("albums=%#v err=%v", albums, err)
	}
	photos, err := client.ListAlbumPhotos(ctx, AlbumPhotosRequest{AlbumID: "album-1", Cursor: "2", MaxResults: 1, Privacy: 1, Media: "photos"})
	if err != nil || len(photos.Items) != 1 || photos.PrevCursor == nil || *photos.PrevCursor != "1" || photos.HasMore {
		t.Fatalf("album photos=%#v err=%v", photos, err)
	}
	created, err := client.CreateAlbum(ctx, CreateAlbumRequest{Title: "New album", Description: "Description", PrimaryPhotoID: "photo-1"})
	if err != nil || created.ID != "album-new" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if err := client.AddAlbumPhoto(ctx, "album-1", "photo-2"); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveAlbumPhoto(ctx, "album-1", "photo-2"); err != nil {
		t.Fatal(err)
	}
}

func TestPublicPhotostreamUsesUnsignedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.URL.Query().Get("method") != "flickr.people.getPublicPhotos" {
			t.Errorf("public request=%s auth=%q", request.URL.String(), request.Header.Get("Authorization"))
		}
		writeJSON(writer, http.StatusOK, `{"stat":"ok","photos":{"page":1,"pages":1,"perpage":1,"total":1,"photo":[{"id":"photo-public","owner":"owner@N01","secret":"s","server":"srv","title":"Public","ispublic":1,"media":"photo"}]}}`)
	}))
	defer server.Close()
	config := testConfig(server.URL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Approval.Scopes = nil
	config.Accounts[0].Settings = map[string]any{"user_id": "owner@N01"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != "photo-public" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("public client must not expose Reactor")
	}
}
