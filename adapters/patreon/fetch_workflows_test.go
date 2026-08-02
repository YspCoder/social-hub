package patreon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchAndTypedWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/oauth2/v2/identity":
			if request.URL.Query().Get("fields[user]") != userFields || request.URL.Query().Get("include") != "null" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"type":"user","id":"10","attributes":{"full_name":"Alice Creator","first_name":"Alice","vanity":"alice","image_url":"https://cdn.example/alice.png","url":"https://patreon.com/alice"}}}`)
		case "/api/oauth2/v2/posts/200":
			writeJSON(writer, http.StatusOK, postFixture("200", "published", "100"))
		case "/api/oauth2/v2/campaigns/100/posts":
			if request.URL.Query().Get("page[count]") != "1000" || request.URL.Query().Get("page[cursor]") != "post-cursor" || request.URL.Query().Get("fields[post]") != postFields {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[`+postFixtureResource("201", "pending", "100")+`,{"type":"other","id":"skip"}],"meta":{"pagination":{"total":2,"cursors":{"next":"post-next"}}}}`)
		case "/api/oauth2/v2/campaigns/100":
			writeJSON(writer, http.StatusOK, `{"data":`+campaignFixture("100")+`}`)
		case "/api/oauth2/v2/campaigns":
			if request.URL.Query().Get("page[count]") != "20" || request.URL.Query().Get("page[cursor]") != "campaign-cursor" || request.URL.Query().Get("fields[campaign]") != campaignFields {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[`+campaignFixture("100")+`],"meta":{"pagination":{"cursors":{"next":"campaign-next"}}}}`)
		case "/api/oauth2/v2/members/member-1":
			writeJSON(writer, http.StatusOK, `{"data":`+memberFixture("member-1", "100")+`}`)
		case "/api/oauth2/v2/campaigns/100/members":
			if request.URL.Query().Get("page[count]") != "2" || request.URL.Query().Get("page[cursor]") != "member-cursor" || request.URL.Query().Get("fields[member]") != memberFields {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[`+memberFixture("member-2", "100")+`],"meta":{"pagination":{"cursors":{"next":"member-next"}}}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	scopes := []string{"identity", "campaigns", "campaigns.posts", "campaigns.members"}
	_, client := newTestClient(t, server, true, false, scopes)

	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != "10" || user.Username == nil || *user.Username != "alice" || user.DisplayName == nil || *user.DisplayName != "Alice Creator" || user.AvatarURL == nil || len(user.Extensions["patreon.user"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "200")
	if err != nil || post.ID != "200" || post.AuthorID == nil || *post.AuthorID != "10" || post.Text == nil || *post.Text != "<p>Creator post</p>" || post.Visibility == nil || *post.Visibility != "public" || post.Status.State != socialhub.PublishStatePublished || len(post.Media) != 1 || post.Media[0].URL != "https://video.example/embed" {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "10", Cursor: "post-cursor", MaxResults: 1000})
	if err != nil || len(posts.Items) != 1 || posts.Items[0].ID != "201" || posts.Items[0].Status.State != socialhub.PublishStatePending || posts.NextCursor == nil || *posts.NextCursor != "post-next" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "200"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments=%v", err)
	}
	campaign, err := client.GetCampaign(context.Background())
	if err != nil || campaign.ID != "100" || campaign.CreatorID != "10" || campaign.Name == nil || *campaign.Name != "Alice Studio" || campaign.PatronCount == nil || *campaign.PatronCount != 42 || campaign.ImageURL == nil || len(campaign.Raw) == 0 {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	campaigns, err := client.ListCampaigns(context.Background(), 0, "campaign-cursor")
	if err != nil || len(campaigns.Items) != 1 || campaigns.NextCursor == nil || *campaigns.NextCursor != "campaign-next" || !campaigns.HasMore {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	member, err := client.GetMember(context.Background(), "member-1")
	if err != nil || member.ID != "member-1" || member.UserID != "11" || member.CampaignID != "100" || member.FullName != nil || member.PatronStatus == nil || *member.PatronStatus != "active_patron" || member.EntitledAmountCents == nil || *member.EntitledAmountCents != 500 || len(member.EntitledTierIDs) != 2 || len(member.Raw) == 0 {
		t.Fatalf("member=%#v err=%v", member, err)
	}
	members, err := client.ListMembers(context.Background(), 2, "member-cursor")
	if err != nil || len(members.Items) != 1 || members.Items[0].ID != "member-2" || members.NextCursor == nil || *members.NextCursor != "member-next" || !members.HasMore {
		t.Fatalf("members=%#v err=%v", members, err)
	}
	client.scopes = append(client.scopes, "campaigns.members[email]")
	if fields := client.memberQuery(nil).Get("fields[member]"); fields != memberFields+",email" {
		t.Fatalf("email fields=%s", fields)
	}
}

func TestFetchAndWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false, []string{"identity"})
	now := time.Now()
	invalidCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "other"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad/path"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "other"})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 1001})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad/path"})
			return err
		},
		func() error { _, err := client.GetMember(context.Background(), "bad/path"); return err },
		func() error { _, err := client.ListMembers(context.Background(), -1, ""); return err },
		func() error { _, err := client.ListCampaigns(context.Background(), 1, "bad\n"); return err },
	}
	for index, call := range invalidCalls {
		err := call()
		if !errors.Is(err, socialhub.ErrInvalidArgument) && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	if _, err := client.GetPost(context.Background(), "200"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("post scope=%v", err)
	}
	if _, err := client.GetCampaign(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("campaign scope=%v", err)
	}
	if _, err := client.GetMember(context.Background(), "member-1"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("member scope=%v", err)
	}
}

func TestBadJSONAPIResponsesAndMappingHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/oauth2/v2/identity":
			writeJSON(writer, http.StatusOK, `{"data":{"type":"other","id":"10"}}`)
		case "/api/oauth2/v2/posts/200":
			writeJSON(writer, http.StatusOK, `{"data":{"type":"post","id":"201","relationships":{"campaign":{"data":{"type":"campaign","id":"100"}}}}}`)
		case "/api/oauth2/v2/campaigns/100/posts", "/api/oauth2/v2/campaigns", "/api/oauth2/v2/campaigns/100/members":
			writeJSON(writer, http.StatusOK, `{"data":[],"meta":{"pagination":{"cursors":{"next":"bad\nvalue"}}}}`)
		case "/api/oauth2/v2/campaigns/100":
			writeJSON(writer, http.StatusOK, `{"data":{"type":"campaign","id":"101"}}`)
		case "/api/oauth2/v2/members/member-1":
			writeJSON(writer, http.StatusOK, `{"data":{"type":"member","id":"member-1","relationships":{"campaign":{"data":{"type":"campaign","id":"999"}}}}}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false, []string{"identity", "campaigns", "campaigns.posts", "campaigns.members"})
	calls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), "me"); return err },
		func() error { _, err := client.GetPost(context.Background(), "200"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
			return err
		},
		func() error { _, err := client.GetCampaign(context.Background()); return err },
		func() error { _, err := client.ListCampaigns(context.Background(), 0, ""); return err },
		func() error { _, err := client.GetMember(context.Background(), "member-1"); return err },
		func() error { _, err := client.ListMembers(context.Background(), 0, ""); return err },
	}
	for index, call := range calls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}

	failed := "failed"
	post := postResource{ID: "1", Type: "post"}
	post.Attributes.AppStatus = &failed
	mapped := mapPost("creator", post, time.Now())
	if mapped.Status.State != socialhub.PublishStateFailed || mapped.Text != nil || mapped.Visibility != nil {
		t.Fatalf("mapped=%#v", mapped)
	}
	blank := " "
	if cleanStringPointer(&blank) != nil || firstStringPointer(nil, &blank) != nil || stringPointer("") != nil || stringValue(nil) != "" {
		t.Fatal("string helper contract failed")
	}
}

func postFixture(id, status, campaignID string) string {
	return `{"data":` + postFixtureResource(id, status, campaignID) + `}`
}

func postFixtureResource(id, status, campaignID string) string {
	published := `"2026-08-01T10:00:00Z"`
	public := "true"
	if status == "pending" {
		published = "null"
		public = "false"
	}
	return `{"type":"post","id":"` + id + `","attributes":{"app_status":"` + status + `","content":"<p>Creator post</p>","embed_url":"https://video.example/embed","is_public":` + public + `,"published_at":` + published + `,"title":"Post","url":"https://patreon.com/posts/` + id + `"},"relationships":{"campaign":{"data":{"type":"campaign","id":"` + campaignID + `"}},"user":{"data":{"type":"user","id":"10"}}}}`
}

func campaignFixture(id string) string {
	return `{"type":"campaign","id":"` + id + `","attributes":{"name":"Alice Studio","creation_name":"videos","summary":"A campaign","url":"https://patreon.com/alice","image_small_url":"https://cdn.example/campaign.png","currency":"USD","patron_count":42,"is_monthly":true,"is_nsfw":false,"created_at":"2020-01-01T00:00:00Z","published_at":"2020-02-01T00:00:00Z"},"relationships":{"creator":{"data":{"type":"user","id":"10"}}}}`
}

func memberFixture(id, campaignID string) string {
	return `{"type":"member","id":"` + id + `","attributes":{"full_name":null,"patron_status":"active_patron","last_charge_status":"Paid","last_charge_date":"2026-08-01T00:00:00Z","pledge_relationship_start":"2025-01-01T00:00:00Z","campaign_lifetime_support_cents":5000,"currently_entitled_amount_cents":500,"will_pay_amount_cents":500},"relationships":{"campaign":{"data":{"type":"campaign","id":"` + campaignID + `"}},"user":{"data":{"type":"user","id":"11"}},"currently_entitled_tiers":{"data":[{"type":"tier","id":"tier-1"},{"type":"tier","id":"tier-2"}]}}}`
}
