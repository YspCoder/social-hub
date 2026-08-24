package microsoftads

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMutationsValidateAccountAndParentsBeforeWrite(t *testing.T) {
	validCampaign := Campaign{ID: testCampaignID, CampaignType: "Search", Languages: []string{"English"}}
	validAdGroup := AdGroup{ID: testAdGroupID}
	tests := []struct {
		name     string
		mutation string
		handler  func(http.ResponseWriter, *http.Request)
		call     func(*Client) error
	}{
		{
			name: "configured account", mutation: "/Campaigns",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/Account/Query" {
					t.Fatalf("unexpected path=%s", request.URL.Path)
				}
				writeValue(t, writer, http.StatusOK, map[string]any{"Account": Account{ID: "9999"}})
			},
			call: func(client *Client) error {
				_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{Name: "Safe", DailyBudget: 10, TimeZone: "UTC"})
				return err
			},
		},
		{
			name: "campaign parent", mutation: "/AdGroups",
			handler: accountThen(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/Campaigns/QueryByIds" {
					t.Fatalf("unexpected path=%s", request.URL.Path)
				}
				wrong := validCampaign
				wrong.ID = "9999"
				writeValue(t, writer, http.StatusOK, map[string]any{"Campaigns": []Campaign{wrong}})
			}),
			call: func(client *Client) error {
				_, err := client.CreateAdGroup(context.Background(), testCampaignID, CreateAdGroupRequest{Name: "Safe", Language: "English"})
				return err
			},
		},
		{
			name: "ad group parent for RSA", mutation: "/Ads",
			handler: accountThen(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/AdGroups/QueryByIds" {
					t.Fatalf("unexpected path=%s", request.URL.Path)
				}
				wrong := validAdGroup
				wrong.ID = "9999"
				writeValue(t, writer, http.StatusOK, map[string]any{"AdGroups": []AdGroup{wrong}})
			}),
			call: func(client *Client) error {
				_, err := client.CreateResponsiveSearchAd(context.Background(), testCampaignID, testAdGroupID, CreateResponsiveSearchAdRequest{
					FinalURLs: []string{"https://example.com"}, Headlines: testAdTextAssets(3), Descriptions: testAdTextAssets(2),
				})
				return err
			},
		},
		{
			name: "ad group parent for Keyword", mutation: "/Keywords",
			handler: accountThen(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/AdGroups/QueryByIds" {
					t.Fatalf("unexpected path=%s", request.URL.Path)
				}
				writeValue(t, writer, http.StatusOK, map[string]any{"AdGroups": []AdGroup{}})
			}),
			call: func(client *Client) error {
				_, err := client.CreateKeyword(context.Background(), testCampaignID, testAdGroupID, CreateKeywordRequest{Text: "safe", MatchType: MatchTypeExact})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mutationWrites atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assertAPIRequest(t, request)
				if request.URL.Path == test.mutation {
					mutationWrites.Add(1)
				}
				test.handler(writer, request)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			err := test.call(client)
			if err == nil || mutationWrites.Load() != 0 {
				t.Fatalf("error=%v mutation writes=%d", err, mutationWrites.Load())
			}
		})
	}
}

func TestPartialErrorsAreTypedAndHTTPFaultsAreClassified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/Account/Query":
			writeValue(t, writer, http.StatusOK, map[string]any{"Account": Account{ID: testAccountID}})
		case "/Campaigns":
			writer.Header().Set("TrackingId", "partial-tracking")
			writeValue(t, writer, http.StatusOK, map[string]any{
				"CampaignIds":   []any{nil},
				"PartialErrors": []map[string]any{{"Code": 117, "ErrorCode": "CallRateExceeded", "Message": "developer_token: secret-value", "Index": 0}},
			})
		default:
			t.Fatalf("unexpected path=%s", request.URL.Path)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{Name: "Rate", DailyBudget: 10, TimeZone: "UTC"})
	var apiError *APIError
	if !errors.As(err, &apiError) || !errors.Is(err, socialhub.ErrRateLimited) || !apiError.Retryable() ||
		len(apiError.Failures) != 1 || apiError.Failures[0].Message != "developer_token: [REDACTED]" || apiError.TrackingID != "partial-tracking" {
		t.Fatalf("typed partial error=%#v err=%v", apiError, err)
	}

	tests := []struct {
		body string
		want socialhub.ErrorCode
	}{
		{`{"TrackingId":"rate","Type":"ApiFaultDetail","OperationErrors":[{"Code":117,"ErrorCode":"CallRateExceeded","Message":"slow"}]}`, socialhub.CodeRateLimited},
		{`{"ApiFaultDetail":{"TrackingId":"concurrent","OperationErrors":[{"Code":207,"ErrorCode":"ConcurrentRequestOverLimit","Message":"wait"}]}}`, socialhub.CodeRateLimited},
		{`{"AdApiFaultDetail":{"OperationErrors":[{"Code":105,"ErrorCode":"InvalidCredentials","Message":"Authorization: Bearer top-secret"}]}}`, socialhub.CodeUnauthenticated},
		{`{"EditorialApiFaultDetail":{"BatchErrors":[{"Code":1,"ErrorCode":"InvalidField","Message":"download https://example.blob.core.windows.net/report.zip?sig=top-secret"}]}}`, socialhub.CodeInvalidArgument},
	}
	for _, test := range tests {
		err := decodeHTTPError(http.StatusBadRequest, http.Header{"Retry-After": {"2.5"}}, []byte(test.body))
		hub := hubError(t, err)
		if hub.Code != test.want {
			t.Errorf("body=%s error=%#v", test.body, hub)
		}
		if strings.Contains(hub.PlatformMessage, "top-secret") || (test.want == socialhub.CodeRateLimited && hub.RetryAfter != 2500*time.Millisecond) {
			t.Errorf("unsafe or incomplete error=%#v", hub)
		}
	}
}

func TestRedirectsSecureDownloadAndSizeBound(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetHits.Add(1)
		_, _ = writer.Write([]byte("target"))
	}))
	defer target.Close()
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/Account/Query", "/redirect-report":
			http.Redirect(writer, request, target.URL+"/leaked?sig=target", http.StatusFound)
		case "/large-report":
			writer.WriteHeader(http.StatusOK)
			writer.(http.Flusher).Flush()
			_, _ = writer.Write([]byte("0123456789abcdefghijklmnop"))
		case "/failed-report":
			writeValue(t, writer, http.StatusBadRequest, map[string]any{
				"OperationErrors": []map[string]any{{"Code": 1, "ErrorCode": "InvalidRequest", "Message": "failed"}},
			})
		default:
			t.Fatalf("unexpected path=%s", request.URL.Path)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	if _, err := client.GetAccount(context.Background()); err == nil || targetHits.Load() != 0 {
		t.Fatalf("API redirect error=%v target hits=%d", err, targetHits.Load())
	}
	var destination bytes.Buffer
	redirectURL := server.URL + "/redirect-report?sig=super-secret"
	if _, err := client.DownloadReport(context.Background(), redirectURL, &destination); err == nil || strings.Contains(err.Error(), "super-secret") || targetHits.Load() != 0 {
		t.Fatalf("download redirect error=%v target hits=%d", err, targetHits.Load())
	}
	destination.Reset()
	written, err := client.DownloadReport(context.Background(), server.URL+"/large-report?sig=large-secret", &destination)
	if err == nil || written != 16 || destination.Len() != 16 || strings.Contains(err.Error(), "large-secret") {
		t.Fatalf("large download written=%d len=%d err=%v", written, destination.Len(), err)
	}
	if _, err := client.DownloadReport(context.Background(), server.URL+"/failed-report?sig=failed-secret", &destination); err == nil || strings.Contains(err.Error(), "failed-secret") {
		t.Fatalf("failed download error=%v", err)
	}
	for _, rawURL := range []string{
		"http://reporting.api.bingads.microsoft.com/report.zip",
		"https://evil.example/report.zip",
		"https://user:pass@reporting.api.bingads.microsoft.com/report.zip",
	} {
		if _, err := client.DownloadReport(context.Background(), rawURL, &destination); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("unsafe URL=%q err=%v", rawURL, err)
		}
	}
}

func TestValidationRejectsBeforeNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("validation reached network")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	calls := []func() error{
		func() error { _, err := client.GetCampaign(ctx, "bad"); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			_, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { _, err := client.SetCampaignStatus(ctx, testCampaignID, "Deleted"); return err },
		func() error { _, err := client.GetAdGroup(ctx, "bad", testAdGroupID); return err },
		func() error { _, err := client.CreateAdGroup(ctx, testCampaignID, CreateAdGroupRequest{}); return err },
		func() error {
			_, err := client.UpdateAdGroup(ctx, testCampaignID, testAdGroupID, UpdateAdGroupRequest{})
			return err
		},
		func() error {
			_, err := client.SetAdGroupStatus(ctx, testCampaignID, testAdGroupID, "Deleted")
			return err
		},
		func() error { _, err := client.ListResponsiveSearchAds(ctx, "bad", testAdGroupID); return err },
		func() error {
			_, err := client.CreateResponsiveSearchAd(ctx, testCampaignID, testAdGroupID, CreateResponsiveSearchAdRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateResponsiveSearchAd(ctx, testCampaignID, testAdGroupID, testAdID, UpdateResponsiveSearchAdRequest{})
			return err
		},
		func() error {
			_, err := client.SetResponsiveSearchAdStatus(ctx, testCampaignID, testAdGroupID, testAdID, "Deleted")
			return err
		},
		func() error { _, err := client.ListKeywords(ctx, "bad", testAdGroupID); return err },
		func() error {
			_, err := client.CreateKeyword(ctx, testCampaignID, testAdGroupID, CreateKeywordRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateKeyword(ctx, testCampaignID, testAdGroupID, testKeywordID, UpdateKeywordRequest{})
			return err
		},
		func() error {
			_, err := client.SetKeywordStatus(ctx, testCampaignID, testAdGroupID, testKeywordID, "Deleted")
			return err
		},
		func() error {
			_, err := client.SubmitCampaignPerformanceReport(ctx, CampaignPerformanceReportRequest{})
			return err
		},
		func() error { _, err := client.PollReport(ctx, ""); return err },
		func() error { _, err := client.DownloadReport(ctx, "", nil); return err },
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
}

func accountThen(t *testing.T, next http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/Account/Query" {
			writeValue(t, writer, http.StatusOK, map[string]any{"Account": Account{ID: testAccountID}})
			return
		}
		next(writer, request)
	}
}
