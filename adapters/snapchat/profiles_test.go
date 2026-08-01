package snapchat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPublicProfileWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/v1/public_profiles/profile-1":
			writeJSON(writer, profileListFixture())
		case "/v1/public_profiles/my_profile":
			writeJSON(writer, `{"request_id":"request-my","request_status":"SUCCESS","public_profile":{"id":"profile-1","display_name":"Creator","snap_user_name":"creator"}}`)
		case "/public/v1/public_profiles/search":
			if request.URL.Query().Get("query") != "Creator" || request.URL.Query().Get("limit") != "20" ||
				request.URL.Query().Get("category") != "CATEGORY_PERSON" || request.URL.Query().Get("tier") != "TIER_PUBLIC" ||
				request.URL.Query().Get("includeStandard") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"request_id":"request-search","request_status":"SUCCESS","public_profiles":[{"sub_request_status":"SUCCESS","public_profile":{"id":"profile-2","display_name":"Other","subscriber_count":5}}]}`)
		case "/v1/public_profiles/profile-1/spotlights":
			if request.URL.Query().Get("limit") != "10" || request.URL.Query().Get("cursor") != "cursor-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, spotlightListFixture())
		case "/v1/public_profiles/profile-1/spotlights/spot-1":
			writeJSON(writer, spotlightListFixture())
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{requiredScope})
	workflow := client.PublicProfileWorkflow()

	profile, err := workflow.Profile(context.Background(), "profile-1")
	if err != nil || profile.ID != "profile-1" || profile.Username == nil || *profile.Username != "creator" || profile.ProfileURL == nil {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	var profileExtension struct {
		SubscriberCount int64 `json:"subscriber_count"`
	}
	if err := json.Unmarshal(profile.Extensions["snapchat.public_profile"], &profileExtension); err != nil || profileExtension.SubscriberCount != 409 {
		t.Fatalf("profile extension=%#v err=%v", profileExtension, err)
	}
	myProfile, err := workflow.MyProfile(context.Background())
	if err != nil || myProfile.ID != "profile-1" {
		t.Fatalf("my profile=%#v err=%v", myProfile, err)
	}
	search, err := workflow.SearchProfiles(context.Background(), ProfileSearchRequest{
		Query: "Creator", Limit: 20, Category: SearchCategoryPerson, Tier: SearchTierPublic, IncludeStandard: true,
	})
	if err != nil || len(search.Items) != 1 || search.Items[0].ID != "profile-2" || search.HasMore {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	spotlights, err := workflow.ListSpotlights(context.Background(), SpotlightListRequest{Limit: 10, Cursor: "cursor-1"})
	if err != nil || len(spotlights.Items) != 1 || spotlights.NextCursor == nil || *spotlights.NextCursor != "cursor-2" {
		t.Fatalf("spotlights=%#v err=%v", spotlights, err)
	}
	post := spotlights.Items[0]
	if post.Text == nil || *post.Text != "A new spotlight" || len(post.Media) != 1 || post.Media[0].Duration == nil ||
		*post.Media[0].Duration != 10*time.Second || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("mapped spotlight=%#v", post)
	}
	spotlight, err := workflow.Spotlight(context.Background(), "spot-1")
	if err != nil || spotlight.ID != "spot-1" {
		t.Fatalf("spotlight=%#v err=%v", spotlight, err)
	}
}

func TestWorkflowValidationAndBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, `{"request_id":"request-err","request_status":"ERROR","error_code":"AUTHORIZATION_PERMISSION_DENIED","display_message":"allowlist required"}`)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{requiredScope})
	workflow := client.PublicProfileWorkflow()
	if _, err := workflow.Profile(context.Background(), ""); err == nil {
		t.Fatal("expected profile validation error")
	}
	if _, err := workflow.SearchProfiles(context.Background(), ProfileSearchRequest{Query: "x", Limit: 101}); err == nil {
		t.Fatal("expected search limit validation error")
	}
	if _, err := workflow.ListSpotlights(context.Background(), SpotlightListRequest{Limit: -1}); err == nil {
		t.Fatal("expected Spotlight limit validation error")
	}
	if _, err := workflow.MyProfile(context.Background()); err == nil {
		t.Fatal("expected business-status error")
	}
}

func profileListFixture() string {
	return `{
  "request_id":"request-profile",
  "request_status":"SUCCESS",
  "public_profiles":[{
    "sub_request_status":"SUCCESS",
    "public_profile":{
      "id":"profile-1",
      "organization_id":"organization-1",
      "display_name":"Creator",
      "description":"Profile description",
      "snap_user_name":"creator",
      "profile_type":"PUBLIC_PROFILE",
      "profile_tier":"TIER_PUBLIC",
      "subscriber_count":"409",
      "logo_urls":{"original_logo_url":"https://cdn.example/avatar.jpg"}
    }
  }]
}`
}

func spotlightListFixture() string {
	return `{
  "request_id":"request-spotlight",
  "request_status":"SUCCESS",
  "spotlights":[{
    "sub_request_status":"SUCCESS",
    "spotlight":{
      "id":"spot-1",
      "profile_id":"profile-1",
      "thumbnail_url":"https://cdn.example/thumb.jpg",
      "media_url":"https://cdn.example/video.mp4",
      "created_at":"2026-08-01T00:00:00Z",
      "duration":10,
      "title":"A new spotlight",
      "status":"LIVE",
      "hashtags":["example"],
      "ml_tags":["demo"]
    }
  }],
  "paging":{"next_page_id":"cursor-2","next_link":"https://businessapi.snapchat.com/next"}
}`
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(writer, value)
}
