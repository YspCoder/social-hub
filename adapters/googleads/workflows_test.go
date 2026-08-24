package googleads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPaidMediaWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/v25/customers:listAccessibleCustomers":
			if request.Method != http.MethodGet {
				t.Errorf("method=%s", request.Method)
			}
			writeJSON(writer, http.StatusOK, `{"resourceNames":["customers/1234567890","customers/2222222222"]}`)
		case "/v25/customers/1234567890/googleAds:search":
			handleSearch(t, writer, request)
		case "/v25/customers/1234567890/campaignBudgets:mutate":
			handleBudgetMutate(t, writer, request)
		case "/v25/customers/1234567890/campaigns:mutate":
			handleCampaignMutate(t, writer, request)
		case "/v25/customers/1234567890/adGroups:mutate":
			handleAdGroupMutate(t, writer, request)
		case "/v25/customers/1234567890/adGroupAds:mutate":
			handleAdGroupAdMutate(t, writer, request)
		case "/v25/customers/1234567890/ads:mutate":
			handleAdMutate(t, writer, request)
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	customer, err := client.GetCustomer(ctx)
	if err != nil || customer.ID != testCustomerID || len(customer.Raw) == 0 {
		t.Fatalf("customer=%#v err=%v", customer, err)
	}
	accessible, err := client.ListAccessibleCustomers(ctx)
	if err != nil || len(accessible) != 2 {
		t.Fatalf("accessible=%v err=%v", accessible, err)
	}

	budgets, err := client.ListCampaignBudgets(ctx, ListRequest{PageToken: "page-1"})
	if err != nil || len(budgets.Items) != 1 || budgets.NextPageToken != "page-2" || len(budgets.Items[0].Raw) == 0 {
		t.Fatalf("budgets=%#v err=%v", budgets, err)
	}
	shared := false
	budget, err := client.CreateCampaignBudget(ctx, CreateCampaignBudgetRequest{
		Name: "Search budget", AmountMicros: 5_000_000, ExplicitlyShared: &shared,
		Fields: map[string]any{"alignedBiddingStrategyId": "99"},
	})
	if err != nil || budget.ResourceName != testBudget {
		t.Fatalf("budget=%#v err=%v", budget, err)
	}
	budget, err = client.UpdateCampaignBudget(ctx, testBudget, UpdateCampaignBudgetRequest{
		Name: stringPointer("Search budget 2"), AmountMicros: int64Pointer(6_000_000),
	})
	if err != nil || budget.Name != "Search budget 2" {
		t.Fatalf("budget=%#v err=%v", budget, err)
	}
	if err := client.RemoveCampaignBudget(ctx, testBudget); err != nil {
		t.Fatal(err)
	}

	campaigns, err := client.ListCampaigns(ctx, ListRequest{})
	if err != nil || len(campaigns.Items) != 1 || campaigns.Items[0].CampaignBudget != testBudget {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Brand search", BudgetResourceName: testBudget,
		ContainsEUPoliticalAdvertising: DoesNotContainEUPoliticalAdvertising,
		NetworkSettings:                &NetworkSettings{TargetGoogleSearch: true, TargetSearchNetwork: true},
		Fields:                         map[string]any{"trackingUrlTemplate": "https://tracking.example/{lpurl}"},
	})
	if err != nil || campaign.Status != StatusPaused {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	campaign, err = client.UpdateCampaign(ctx, testCampaign, UpdateCampaignRequest{
		Name: stringPointer("Brand search 2"), BudgetResourceName: stringPointer(testBudget),
		NetworkSettings: &NetworkSettings{TargetGoogleSearch: true},
	})
	if err != nil || campaign.Name != "Brand search 2" {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	if _, err := client.SetCampaignStatus(ctx, testCampaign, StatusEnabled); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveCampaign(ctx, testCampaign); err != nil {
		t.Fatal(err)
	}

	adGroups, err := client.ListAdGroups(ctx, ListAdGroupsRequest{CampaignResourceName: testCampaign})
	if err != nil || len(adGroups.Items) != 1 || adGroups.Items[0].Campaign != testCampaign {
		t.Fatalf("adGroups=%#v err=%v", adGroups, err)
	}
	adGroup, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{
		CampaignResourceName: testCampaign, Name: "Exact", CPCBidMicros: 2_000_000,
		Fields: map[string]any{"targetCpaMicros": "3000000"},
	})
	if err != nil || adGroup.Status != StatusPaused {
		t.Fatalf("adGroup=%#v err=%v", adGroup, err)
	}
	adGroup, err = client.UpdateAdGroup(ctx, testAdGroup, UpdateAdGroupRequest{
		Name: stringPointer("Exact 2"), CPCBidMicros: int64Pointer(2_500_000),
	})
	if err != nil || adGroup.Name != "Exact 2" {
		t.Fatalf("adGroup=%#v err=%v", adGroup, err)
	}
	if _, err := client.SetAdGroupStatus(ctx, testAdGroup, StatusEnabled); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveAdGroup(ctx, testAdGroup); err != nil {
		t.Fatal(err)
	}

	ads, err := client.ListResponsiveSearchAds(ctx, ListAdsRequest{AdGroupResourceName: testAdGroup})
	if err != nil || len(ads.Items) != 1 || ads.Items[0].Ad.ResourceName != testAd {
		t.Fatalf("ads=%#v err=%v", ads, err)
	}
	headlines := validHeadlines()
	descriptions := validDescriptions()
	ad, err := client.CreateResponsiveSearchAd(ctx, CreateResponsiveSearchAdRequest{
		AdGroupResourceName: testAdGroup, Name: "RSA", FinalURLs: []string{"https://example.com/landing"},
		Headlines: headlines, Descriptions: descriptions, Path1: "products", Path2: "search",
		Fields: map[string]any{"trackingUrlTemplate": "https://tracking.example/{lpurl}"},
	})
	if err != nil || ad.Status != StatusPaused || ad.Ad.ResourceName != testAd {
		t.Fatalf("ad=%#v err=%v", ad, err)
	}
	updated, err := client.UpdateResponsiveSearchAd(ctx, testAd, UpdateResponsiveSearchAdRequest{
		Name: stringPointer("RSA 2"), FinalURLs: &[]string{"https://example.com/new"},
		Headlines: &headlines, Descriptions: &descriptions, Path1: stringPointer("new"), Path2: stringPointer("offer"),
	})
	if err != nil || updated.Name != "RSA 2" {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if _, err := client.SetAdStatus(ctx, testAdGroupAd, StatusEnabled); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveAd(ctx, testAdGroupAd); err != nil {
		t.Fatal(err)
	}

	report, err := client.Search(ctx, SearchRequest{
		Query: "SELECT metrics.clicks FROM campaign", PageToken: "report-page", ValidateOnly: true,
	}, socialhub.WithRequestID("caller-request"))
	if err != nil || len(report.Rows) != 1 || report.TotalResultsCount != "1" || report.QueryResourceConsumption != "7" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func handleSearch(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	body := decodeBody(t, request)
	if _, found := body["pageSize"]; found {
		t.Fatal("deprecated pageSize must not be sent")
	}
	query, _ := body["query"].(string)
	switch {
	case query == customerQuery:
		writeJSON(writer, http.StatusOK, `{"results":[{"customer":{"resourceName":"customers/1234567890","id":"1234567890","descriptiveName":"Brand","currencyCode":"USD","timeZone":"Asia/Shanghai"}}]}`)
	case query == campaignBudgetQuery:
		if body["pageToken"] != "page-1" {
			t.Errorf("budget search=%v", body)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"campaignBudget":{"resourceName":"customers/1234567890/campaignBudgets/101","id":"101","name":"Search budget","amountMicros":"5000000"}}],"nextPageToken":"page-2"}`)
	case query == campaignQuery:
		writeJSON(writer, http.StatusOK, `{"results":[{"campaign":{"resourceName":"customers/1234567890/campaigns/201","id":"201","name":"Brand search","status":"PAUSED","campaignBudget":"customers/1234567890/campaignBudgets/101"}}]}`)
	case strings.Contains(query, "FROM ad_group_ad"):
		if !strings.Contains(query, testAdGroup) {
			t.Errorf("ad query=%s", query)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"adGroupAd":{"resourceName":"customers/1234567890/adGroupAds/301~401","adGroup":"customers/1234567890/adGroups/301","status":"PAUSED","ad":{"resourceName":"customers/1234567890/ads/401","id":"401","type":"RESPONSIVE_SEARCH_AD","finalUrls":["https://example.com"]}}}]}`)
	case strings.Contains(query, "FROM ad_group"):
		if !strings.Contains(query, testCampaign) {
			t.Errorf("ad group query=%s", query)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"adGroup":{"resourceName":"customers/1234567890/adGroups/301","id":"301","campaign":"customers/1234567890/campaigns/201","name":"Exact","status":"PAUSED"}}]}`)
	case query == "SELECT metrics.clicks FROM campaign":
		if body["pageToken"] != "report-page" || body["validateOnly"] != true || request.Header.Get("X-Request-ID") != "caller-request" {
			t.Errorf("report search=%v headers=%v", body, request.Header)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"metrics":{"clicks":"4"}}],"fieldMask":"metrics.clicks","totalResultsCount":"1","queryResourceConsumption":"7"}`)
	default:
		t.Fatalf("unexpected query=%q", query)
	}
}

func handleBudgetMutate(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	operation := firstOperation(t, decodeBody(t, request))
	switch {
	case operation["create"] != nil:
		resource := operation["create"].(map[string]any)
		if resource["amountMicros"] != "5000000" || resource["alignedBiddingStrategyId"] != "99" || resource["explicitlyShared"] != false {
			t.Errorf("budget create=%v", resource)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"customers/1234567890/campaignBudgets/101","campaignBudget":{"resourceName":"customers/1234567890/campaignBudgets/101","name":"Search budget"}}]}`)
	case operation["update"] != nil:
		resource := operation["update"].(map[string]any)
		if operation["updateMask"] != "name,amount_micros" || resource["amountMicros"] != "6000000" {
			t.Errorf("budget update=%v", operation)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"customers/1234567890/campaignBudgets/101","campaignBudget":{"resourceName":"customers/1234567890/campaignBudgets/101","name":"Search budget 2"}}]}`)
	default:
		if operation["remove"] != testBudget {
			t.Errorf("budget remove=%v", operation)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"customers/1234567890/campaignBudgets/101"}]}`)
	}
}

func handleCampaignMutate(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	operation := firstOperation(t, decodeBody(t, request))
	name, status := "Brand search", StatusPaused
	switch {
	case operation["create"] != nil:
		resource := operation["create"].(map[string]any)
		if resource["status"] != "PAUSED" || resource["advertisingChannelType"] != "SEARCH" || resource["containsEuPoliticalAdvertising"] != string(DoesNotContainEUPoliticalAdvertising) || resource["manualCpc"] == nil {
			t.Errorf("campaign create=%v", resource)
		}
	case operation["update"] != nil:
		if operation["updateMask"] == "status" {
			status = StatusEnabled
		} else {
			name = "Brand search 2"
			if operation["updateMask"] != "name,campaign_budget,network_settings.target_google_search,network_settings.target_search_network,network_settings.target_content_network" {
				t.Errorf("campaign update mask=%v", operation["updateMask"])
			}
		}
	default:
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"customers/1234567890/campaigns/201"}]}`)
		return
	}
	writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testCampaign+`","campaign":{"resourceName":"`+testCampaign+`","name":"`+name+`","status":"`+string(status)+`","campaignBudget":"`+testBudget+`"}}]}`)
}

func handleAdGroupMutate(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	operation := firstOperation(t, decodeBody(t, request))
	name, status := "Exact", StatusPaused
	switch {
	case operation["create"] != nil:
		resource := operation["create"].(map[string]any)
		if resource["status"] != "PAUSED" || resource["type"] != "SEARCH_STANDARD" || resource["cpcBidMicros"] != "2000000" {
			t.Errorf("ad group create=%v", resource)
		}
	case operation["update"] != nil:
		resource := operation["update"].(map[string]any)
		if operation["updateMask"] == "status" {
			status = StatusEnabled
		} else {
			name = "Exact 2"
			if operation["updateMask"] != "name,cpc_bid_micros" || resource["cpcBidMicros"] != "2500000" {
				t.Errorf("ad group update=%v", operation)
			}
		}
	default:
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAdGroup+`"}]}`)
		return
	}
	writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAdGroup+`","adGroup":{"resourceName":"`+testAdGroup+`","campaign":"`+testCampaign+`","name":"`+name+`","status":"`+string(status)+`"}}]}`)
}

func handleAdGroupAdMutate(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	operation := firstOperation(t, decodeBody(t, request))
	if operation["create"] != nil {
		resource := operation["create"].(map[string]any)
		ad := resource["ad"].(map[string]any)
		if resource["status"] != "PAUSED" || resource["adGroup"] != testAdGroup || ad["responsiveSearchAd"] == nil {
			t.Errorf("RSA create=%v", resource)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAdGroupAd+`","adGroupAd":{"resourceName":"`+testAdGroupAd+`","adGroup":"`+testAdGroup+`","status":"PAUSED","ad":{"resourceName":"`+testAd+`","name":"RSA"}}}]}`)
		return
	}
	if operation["update"] != nil {
		if operation["updateMask"] != "status" {
			t.Errorf("ad status=%v", operation)
		}
		writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAdGroupAd+`","adGroupAd":{"resourceName":"`+testAdGroupAd+`","status":"ENABLED"}}]}`)
		return
	}
	writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAdGroupAd+`"}]}`)
}

func handleAdMutate(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	operation := firstOperation(t, decodeBody(t, request))
	resource := operation["update"].(map[string]any)
	if operation["updateMask"] != "name,final_urls,responsive_search_ad.headlines,responsive_search_ad.descriptions,responsive_search_ad.path1,responsive_search_ad.path2" || resource["responsiveSearchAd"] == nil {
		t.Errorf("ad update=%v", operation)
	}
	writeJSON(writer, http.StatusOK, `{"results":[{"resourceName":"`+testAd+`","ad":{"resourceName":"`+testAd+`","name":"RSA 2","type":"RESPONSIVE_SEARCH_AD"}}]}`)
}

func validHeadlines() []AdTextAsset {
	return []AdTextAsset{{Text: "Buy now"}, {Text: "Free shipping"}, {Text: "Official store", PinnedField: "HEADLINE_3"}}
}

func validDescriptions() []AdTextAsset {
	return []AdTextAsset{{Text: "Shop the latest products."}, {Text: "Fast delivery and easy returns."}}
}
