package baiduads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCreativeCreationRequiresPausedParent(t *testing.T) {
	addCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = decodeRequest(t, request)
		switch request.URL.Path {
		case "/json/sms/service/AdgroupService/getAdgroup":
			writeSuccess(t, writer, []any{adGroupWire(testAdGroupID, testCampaignID, "Active", false)})
		case "/json/sms/service/CreativeService/addCreative":
			addCalled = true
			writeSuccess(t, writer, []any{creativeWire(302, testCampaignID, testAdGroupID, false)})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server)
	defer adapter.Close()
	_, err := client.CreateCreative(context.Background(), validCreativeInput())
	if requireHubError(t, err).Code != socialhub.CodeInvalidArgument || addCalled {
		t.Fatalf("err=%v addCalled=%v", err, addCalled)
	}
}

func TestCreativeSecondPhaseFailureReturnsCreatedResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = decodeRequest(t, request)
		switch request.URL.Path {
		case "/json/sms/service/AdgroupService/getAdgroup":
			writeSuccess(t, writer, []any{adGroupWire(testAdGroupID, testCampaignID, "Paused", true)})
		case "/json/sms/service/CreativeService/addCreative":
			writeSuccess(t, writer, []any{creativeWire(302, testCampaignID, testAdGroupID, false)})
		case "/json/sms/service/CreativeService/updateCreative":
			status := 1
			if err := writeJSONValue(writer, map[string]any{
				"header": map[string]any{
					"status": status, "desc": "rate limited",
					"failures": []any{map[string]any{"code": 8501, "message": "slow down"}}, "traceid": "phase-two",
				},
				"body": map[string]any{},
			}); err != nil {
				t.Fatal(err)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server)
	defer adapter.Close()
	creative, err := client.CreateCreative(context.Background(), validCreativeInput())
	if creative == nil || creative.ID != 302 || !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("creative=%+v err=%v", creative, err)
	}
	hub := requireHubError(t, err)
	if hub.Op != "creative_update" || hub.RequestID != "phase-two" || hub.RetryAfter == 0 {
		t.Fatalf("err=%+v", hub)
	}
}

func TestCreateResourcesRejectUnexpectedActiveState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = decodeRequest(t, request)
		switch request.URL.Path {
		case "/json/sms/service/CampaignService/addCampaign":
			writeSuccess(t, writer, []any{campaignWire(103, "Unsafe", false)})
		case "/json/sms/service/AdgroupService/addAdgroup":
			writeSuccess(t, writer, []any{adGroupWire(203, testCampaignID, "Unsafe", false)})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server)
	defer adapter.Close()
	if _, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{Name: "Unsafe", Budget: 100, MarketingTargetID: 0}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("campaign err=%v", err)
	}
	if _, err := client.CreateAdGroup(context.Background(), CreateAdGroupRequest{CampaignID: testCampaignID, Name: "Unsafe", MaxPrice: 1}); requireHubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("ad group err=%v", err)
	}
}

func writeJSONValue(writer http.ResponseWriter, value any) error {
	writer.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(writer).Encode(value)
}
