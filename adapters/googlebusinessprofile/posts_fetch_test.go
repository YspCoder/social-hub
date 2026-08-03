package googlebusinessprofile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCommonFetchAndPublish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v4/accounts/1001/locations/2002":
			writeJSON(writer, http.StatusOK, `{"name":"accounts/1001/locations/2002","languageCode":"en-US","storeCode":"store-7","locationName":"Example Store","primaryPhone":"+1-555-0100","websiteUrl":"https://store.example","profile":{"description":"Local store"},"metadata":{"mapsUrl":"https://maps.google.example/store","newReviewUrl":"https://search.google.example/review"},"locationState":{"isVerified":true,"isPublished":true}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts/"+testPostID:
			writeJSON(writer, http.StatusOK, localPostJSON(testPostID, "STANDARD", "LIVE"))
		case request.Method == http.MethodGet && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts":
			if request.URL.Query().Get("pageSize") != "2" || request.URL.Query().Get("pageToken") != "page-token" {
				t.Errorf("list query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"localPosts":[`+localPostJSON(testPostID, "STANDARD", "LIVE")+`],"nextPageToken":"next-token"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts":
			var body LocalPostCreateRequest
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.LanguageCode != "en-US" || body.Summary != "New opening hours" || body.TopicType != LocalPostStandard {
				t.Errorf("publish body=%#v", body)
			}
			writeJSON(writer, http.StatusCreated, localPostJSON("post-2", "STANDARD", "PROCESSING"))
		case request.Method == http.MethodDelete && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts/"+testPostID:
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{businessScope})

	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != testLocationID || user.Username == nil || *user.Username != "store-7" ||
		user.DisplayName == nil || *user.DisplayName != "Example Store" || user.ProfileURL == nil ||
		*user.ProfileURL != "https://maps.google.example/store" || len(user.Extensions["google_business_profile.location"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), testPostID)
	if err != nil || post.ID != testPostID || post.AuthorID == nil || *post.AuthorID != testLocationID ||
		post.Text == nil || *post.Text != "Hello customers" || post.Status == nil || post.Status.State != socialhub.PublishStatePublished ||
		len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage || post.Media[0].URL != "https://google.example/photo.jpg" ||
		len(post.Extensions["google_business_profile.local_post"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testLocationID, Cursor: "page-token", MaxResults: 2})
	if err != nil || len(page.Items) != 1 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "next-token" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	text := "New opening hours"
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil || published.ID != "post-2" || published.Status == nil || published.Status.State != socialhub.PublishStatePending {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	status, err := client.PublishStatus(context.Background(), testPostID)
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), testPostID); err != nil {
		t.Fatal(err)
	}
}

func TestTypedEventOfferAndPatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts":
			var body LocalPostCreateRequest
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.TopicType != LocalPostOffer || body.Event == nil || body.Offer == nil ||
				body.Event.Title != "Summer sale" || body.Offer.CouponCode != "SAVE20" || len(body.Media) != 1 {
				t.Errorf("offer body=%#v", body)
			}
			writeJSON(writer, http.StatusCreated, localPostJSON("offer-1", "OFFER", "SCHEDULED"))
		case request.Method == http.MethodPatch && request.URL.Path == "/v4/accounts/1001/locations/2002/localPosts/"+testPostID:
			if request.URL.Query().Get("updateMask") != "summary,callToAction,scheduledTime,media" {
				t.Errorf("update mask=%q", request.URL.Query().Get("updateMask"))
			}
			var body LocalPostPatchRequest
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Summary == nil || *body.Summary != "Updated" ||
				body.CallToAction == nil || body.CallToAction.ActionType != ActionLearnMore || body.Media == nil || len(*body.Media) != 1 {
				t.Errorf("patch body=%#v", body)
			}
			writeJSON(writer, http.StatusOK, localPostJSON(testPostID, "STANDARD", "LIVE"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{businessScope})
	event := LocalPostEventDetails{
		Title: "Summer sale",
		Schedule: TimeInterval{
			StartDate: Date{Year: 2026, Month: 8, Day: 10}, StartTime: TimeOfDay{Hours: 9},
			EndDate: Date{Year: 2026, Month: 8, Day: 12}, EndTime: TimeOfDay{Hours: 18},
		},
	}
	post, err := client.CreateLocalPost(context.Background(), LocalPostCreateRequest{
		Summary: "Save this week", TopicType: LocalPostOffer, Event: &event,
		Offer: &LocalPostOfferDetails{CouponCode: "SAVE20", RedeemOnlineURL: "https://store.example/sale", TermsConditions: "While supplies last"},
		Media: []LocalPostMedia{{MediaFormat: MediaFormatPhoto, SourceURL: "https://cdn.example/sale.jpg"}},
	})
	if err != nil || post.ID != "offer-1" || post.State != LocalPostScheduled {
		t.Fatalf("offer=%#v err=%v", post, err)
	}
	summary := "Updated"
	scheduled := time.Date(2026, time.August, 20, 1, 0, 0, 0, time.UTC)
	media := []LocalPostMedia{{MediaFormat: MediaFormatPhoto, SourceURL: "https://cdn.example/new.jpg"}}
	post, err = client.UpdateLocalPost(context.Background(), testPostID, LocalPostPatchRequest{
		Summary: &summary, CallToAction: &CallToAction{ActionType: ActionLearnMore, URL: "https://store.example/news"},
		ScheduledTime: &scheduled, Media: &media,
	})
	if err != nil || post.ID != testPostID {
		t.Fatalf("updated=%#v err=%v", post, err)
	}
}

func TestPostAndFetchValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v4/accounts/1001/locations/2002":
			writeJSON(writer, http.StatusOK, `{"name":"accounts/1001/locations/9999","locationName":"Wrong"}`)
		case "/v4/accounts/1001/locations/2002/localPosts/" + testPostID:
			writeJSON(writer, http.StatusOK, localPostJSON("wrong-post", "STANDARD", "LIVE"))
		case "/v4/accounts/1001/locations/2002/localPosts":
			writeJSON(writer, http.StatusOK, `{"localPosts":[`+localPostJSON(testPostID, "STANDARD", "LIVE")+`],"nextPageToken":"\n"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, nil)
	if _, err := client.GetUser(context.Background(), "other"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user filter error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), "me"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("user ownership error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("post ID error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), testPostID); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("post ownership error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "other"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("list user error=%v", err)
	}
	now := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 101}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("page size error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "\n"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("response cursor error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testPostID}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments error=%v", err)
	}

	text := "hello"
	invalidCommon := []socialhub.CreatePostRequest{
		{}, {MediaIDs: []string{"media"}}, {Text: &text, MediaIDs: []string{"media"}},
		{Text: &text, ReplyToID: &text}, {Text: &text, QuotePostID: &text}, {Text: &text, Visibility: &text},
	}
	for index, input := range invalidCommon {
		if _, err := client.Publish(context.Background(), input); err == nil {
			t.Fatalf("common publish %d unexpectedly succeeded", index)
		}
	}
	if err := client.DeletePost(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete error=%v", err)
	}
	if _, err := client.UpdateLocalPost(context.Background(), "bad/id", LocalPostPatchRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("update ID error=%v", err)
	}
}

func TestLocalPostInputValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil)
	badDate := LocalPostEventDetails{Title: "Event", Schedule: TimeInterval{
		StartDate: Date{Year: 2026, Month: 2, Day: 30}, StartTime: TimeOfDay{},
		EndDate: Date{Year: 2026, Month: 3, Day: 1}, EndTime: TimeOfDay{},
	}}
	invalidCreates := []LocalPostCreateRequest{
		{},
		{Summary: "x", TopicType: LocalPostAlert},
		{Summary: "x", TopicType: LocalPostEvent},
		{Summary: "x", TopicType: LocalPostStandard, Event: &badDate},
		{Summary: "x", TopicType: LocalPostOffer, Event: &badDate, Offer: &LocalPostOfferDetails{}},
		{Summary: "x", TopicType: LocalPostStandard, Media: []LocalPostMedia{{MediaFormat: MediaFormatPhoto, SourceURL: "file:///tmp/x"}}},
		{Summary: "x", TopicType: LocalPostStandard, CallToAction: &CallToAction{ActionType: ActionCall, URL: "https://example.com"}},
	}
	for index, input := range invalidCreates {
		if _, err := client.CreateLocalPost(context.Background(), input); err == nil {
			t.Fatalf("create %d unexpectedly succeeded", index)
		}
	}
	invalidPatches := []LocalPostPatchRequest{{}, {LanguageCode: stringPointer("en US")}, {Summary: stringPointer(string([]byte{0xff}))}}
	badCTA := CallToAction{ActionType: "UNKNOWN"}
	badTopic := LocalPostAlert
	badMedia := []LocalPostMedia{{MediaFormat: "AUDIO", SourceURL: "https://example.com/a"}}
	invalidPatches = append(invalidPatches, LocalPostPatchRequest{CallToAction: &badCTA}, LocalPostPatchRequest{TopicType: &badTopic}, LocalPostPatchRequest{Media: &badMedia})
	for index, input := range invalidPatches {
		if _, err := client.UpdateLocalPost(context.Background(), testPostID, input); err == nil {
			t.Fatalf("patch %d unexpectedly succeeded", index)
		}
	}
}
