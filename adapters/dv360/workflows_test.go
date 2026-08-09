package dv360

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdvertiserCampaignInsertionOrderAndLineItemWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID:
			writeJSON(t, writer, http.StatusOK, advertiserResource(EntityStatusActive))
		case http.MethodGet + " /v4/advertisers":
			if request.URL.Query().Get("partnerId") != testPartnerID || request.URL.Query().Get("pageSize") != "20" ||
				request.URL.Query().Get("pageToken") != "advertisers-page" || request.URL.Query().Get("orderBy") != "displayName desc" {
				t.Errorf("advertiser query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, listAdvertisersResponse{
				Advertisers: []Advertiser{advertiserResource(EntityStatusActive)}, NextPageToken: "advertisers-next",
			})
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/campaigns":
			if request.URL.Query().Get("filter") == "" || request.Header.Get("X-Request-ID") != "caller-request" {
				t.Errorf("campaign query=%v headers=%v", request.URL.Query(), request.Header)
			}
			writeJSON(t, writer, http.StatusOK, listCampaignsResponse{
				Campaigns:     []Campaign{campaignResource(testCampaignID, "Brand campaign", EntityStatusActive)},
				NextPageToken: "campaigns-next",
			})
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, campaignResource(testCampaignID, "Brand campaign", EntityStatusActive))
		case http.MethodPost + " /v4/advertisers/" + testAdvertiserID + "/campaigns":
			var payload campaignCreatePayload
			decodeJSONBody(t, request, &payload)
			if payload.EntityStatus != EntityStatusPaused || payload.DisplayName != "New campaign" || payload.CampaignGoal.Type != CampaignGoalBrandAwareness {
				t.Errorf("campaign create=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, campaignResource("1002", payload.DisplayName, payload.EntityStatus))
		case http.MethodPatch + " /v4/advertisers/" + testAdvertiserID + "/campaigns/" + testCampaignID:
			if request.URL.Query().Get("updateMask") != "displayName,entityStatus" {
				t.Errorf("campaign mask=%q", request.URL.Query().Get("updateMask"))
			}
			var payload campaignPatchPayload
			decodeJSONBody(t, request, &payload)
			writeJSON(t, writer, http.StatusOK, campaignResource(testCampaignID, *payload.DisplayName, *payload.EntityStatus))
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/insertionOrders":
			writeJSON(t, writer, http.StatusOK, listInsertionOrdersResponse{
				InsertionOrders: []InsertionOrder{insertionOrderResource(testInsertionOrderID, testCampaignID, "Main IO", EntityStatusActive)},
				NextPageToken:   "orders-next",
			})
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/insertionOrders/" + testInsertionOrderID:
			writeJSON(t, writer, http.StatusOK, insertionOrderResource(testInsertionOrderID, testCampaignID, "Main IO", EntityStatusActive))
		case http.MethodPost + " /v4/advertisers/" + testAdvertiserID + "/insertionOrders":
			var payload insertionOrderCreatePayload
			decodeJSONBody(t, request, &payload)
			if payload.EntityStatus != EntityStatusDraft || payload.CampaignID != testCampaignID || payload.DisplayName != "New IO" {
				t.Errorf("insertion order create=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, insertionOrderResource("2002", payload.CampaignID, payload.DisplayName, payload.EntityStatus))
		case http.MethodPatch + " /v4/advertisers/" + testAdvertiserID + "/insertionOrders/" + testInsertionOrderID:
			if request.URL.Query().Get("updateMask") != "displayName,entityStatus" {
				t.Errorf("insertion order mask=%q", request.URL.Query().Get("updateMask"))
			}
			var payload insertionOrderPatchPayload
			decodeJSONBody(t, request, &payload)
			writeJSON(t, writer, http.StatusOK, insertionOrderResource(testInsertionOrderID, testCampaignID, *payload.DisplayName, *payload.EntityStatus))
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/lineItems":
			writeJSON(t, writer, http.StatusOK, listLineItemsResponse{
				LineItems:     []LineItem{lineItemResource(testLineItemID, "Main line", EntityStatusActive)},
				NextPageToken: "lines-next",
			})
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/lineItems/" + testLineItemID:
			writeJSON(t, writer, http.StatusOK, lineItemResource(testLineItemID, "Main line", EntityStatusActive))
		case http.MethodGet + " /v4/advertisers/" + testAdvertiserID + "/lineItems/" + testDuplicateLineItemID:
			writeJSON(t, writer, http.StatusOK, lineItemResource(testDuplicateLineItemID, "Copied line", EntityStatusDraft))
		case http.MethodPost + " /v4/advertisers/" + testAdvertiserID + "/lineItems":
			var payload lineItemCreatePayload
			decodeJSONBody(t, request, &payload)
			if payload.EntityStatus != EntityStatusDraft || payload.InsertionOrderID != testInsertionOrderID ||
				payload.ContainsEUPoliticalAds != DoesNotContainEUPoliticalAdvertising {
				t.Errorf("line item create=%#v", payload)
			}
			created := lineItemResource("3003", payload.DisplayName, payload.EntityStatus)
			writeJSON(t, writer, http.StatusOK, created)
		case http.MethodPatch + " /v4/advertisers/" + testAdvertiserID + "/lineItems/" + testLineItemID:
			if request.URL.Query().Get("updateMask") != "containsEuPoliticalAds,displayName,entityStatus" {
				t.Errorf("line item mask=%q", request.URL.Query().Get("updateMask"))
			}
			var payload lineItemPatchPayload
			decodeJSONBody(t, request, &payload)
			updated := lineItemResource(testLineItemID, *payload.DisplayName, *payload.EntityStatus)
			updated.ContainsEUPoliticalAds = *payload.ContainsEUPoliticalAds
			writeJSON(t, writer, http.StatusOK, updated)
		case http.MethodPost + " /v4/advertisers/" + testAdvertiserID + "/lineItems/" + testLineItemID + ":duplicate":
			var payload duplicateLineItemPayload
			decodeJSONBody(t, request, &payload)
			if payload.TargetDisplayName != "Copied line" || payload.ContainsEUPoliticalAds != DoesNotContainEUPoliticalAdvertising {
				t.Errorf("duplicate=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, duplicateLineItemResponse{DuplicateLineItemID: testDuplicateLineItemID})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()

	if advertiser, err := client.GetAdvertiser(ctx); err != nil || advertiser.AdvertiserID != testAdvertiserID {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	advertisers, err := client.ListAdvertisers(ctx, ListRequest{
		PageSize: 20, PageToken: "advertisers-page", OrderBy: "displayName desc",
	})
	if err != nil || len(advertisers.Items) != 1 || advertisers.NextPageToken != "advertisers-next" {
		t.Fatalf("advertisers=%#v err=%v", advertisers, err)
	}
	if campaign, err := client.GetCampaign(ctx, testCampaignID); err != nil || campaign.CampaignID != testCampaignID {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListRequest{Filter: `entityStatus="ENTITY_STATUS_ACTIVE"`}, socialhub.WithRequestID("caller-request"))
	if err != nil || len(campaigns.Items) != 1 || campaigns.NextPageToken != "campaigns-next" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, validCreateCampaignRequest("New campaign"))
	if err != nil || createdCampaign.EntityStatus != EntityStatusPaused || createdCampaign.CampaignID != "1002" {
		t.Fatalf("created campaign=%#v err=%v", createdCampaign, err)
	}
	campaignName, active := "Updated campaign", EntityStatusActive
	updatedCampaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{
		DisplayName: &campaignName, EntityStatus: &active,
	})
	if err != nil || updatedCampaign.DisplayName != campaignName || updatedCampaign.EntityStatus != active {
		t.Fatalf("updated campaign=%#v err=%v", updatedCampaign, err)
	}

	if order, err := client.GetInsertionOrder(ctx, testInsertionOrderID); err != nil || order.InsertionOrderID != testInsertionOrderID {
		t.Fatalf("order=%#v err=%v", order, err)
	}
	orders, err := client.ListInsertionOrders(ctx, ListRequest{})
	if err != nil || len(orders.Items) != 1 || orders.NextPageToken != "orders-next" {
		t.Fatalf("orders=%#v err=%v", orders, err)
	}
	createdOrder, err := client.CreateInsertionOrder(ctx, validCreateInsertionOrderRequest("New IO"))
	if err != nil || createdOrder.EntityStatus != EntityStatusDraft || createdOrder.InsertionOrderID != "2002" {
		t.Fatalf("created order=%#v err=%v", createdOrder, err)
	}
	orderName := "Updated IO"
	updatedOrder, err := client.UpdateInsertionOrder(ctx, testInsertionOrderID, UpdateInsertionOrderRequest{
		DisplayName: &orderName, EntityStatus: &active,
	})
	if err != nil || updatedOrder.DisplayName != orderName || updatedOrder.EntityStatus != active {
		t.Fatalf("updated order=%#v err=%v", updatedOrder, err)
	}

	if line, err := client.GetLineItem(ctx, testLineItemID); err != nil || line.LineItemID != testLineItemID {
		t.Fatalf("line=%#v err=%v", line, err)
	}
	lines, err := client.ListLineItems(ctx, ListRequest{})
	if err != nil || len(lines.Items) != 1 || lines.NextPageToken != "lines-next" {
		t.Fatalf("lines=%#v err=%v", lines, err)
	}
	createdLine, err := client.CreateLineItem(ctx, validCreateLineItemRequest("New line"))
	if err != nil || createdLine.EntityStatus != EntityStatusDraft || createdLine.LineItemID != "3003" {
		t.Fatalf("created line=%#v err=%v", createdLine, err)
	}
	lineName, political := "Updated line", DoesNotContainEUPoliticalAdvertising
	updatedLine, err := client.UpdateLineItem(ctx, testLineItemID, UpdateLineItemRequest{
		DisplayName: &lineName, EntityStatus: &active, ContainsEUPoliticalAds: &political,
	})
	if err != nil || updatedLine.DisplayName != lineName || updatedLine.EntityStatus != active {
		t.Fatalf("updated line=%#v err=%v", updatedLine, err)
	}
	duplicate, err := client.DuplicateLineItem(ctx, testLineItemID, DuplicateLineItemRequest{
		TargetDisplayName: "Copied line", ContainsEUPoliticalAds: DoesNotContainEUPoliticalAdvertising,
	})
	if err != nil || duplicate.LineItemID != testDuplicateLineItemID || duplicate.DisplayName != "Copied line" {
		t.Fatalf("duplicate=%#v err=%v", duplicate, err)
	}
}

func validCreateCampaignRequest(name string) CreateCampaignRequest {
	end := Date{Year: 2026, Month: 12, Day: 31}
	return CreateCampaignRequest{
		DisplayName: name, CampaignGoal: validTestCampaignGoal(),
		CampaignFlight: CampaignFlight{
			PlannedSpendAmountMicros: "500000000",
			PlannedDates:             DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
		},
		FrequencyCap: FrequencyCap{Unlimited: true},
	}
}

func validCreateInsertionOrderRequest(name string) CreateInsertionOrderRequest {
	order := insertionOrderResource(testInsertionOrderID, testCampaignID, name, EntityStatusDraft)
	return CreateInsertionOrderRequest{
		CampaignID: testCampaignID, DisplayName: name, InsertionOrderType: InsertionOrderRTB,
		Pacing: order.Pacing, FrequencyCap: order.FrequencyCap, Budget: order.Budget,
		KPI: order.KPI, OptimizationObjective: order.OptimizationObjective,
		BidStrategy: &BiddingStrategy{FixedBid: &FixedBidStrategy{BidAmountMicros: "0"}},
	}
}

func validCreateLineItemRequest(name string) CreateLineItemRequest {
	line := lineItemResource(testLineItemID, name, EntityStatusDraft)
	line.Budget.BudgetUnit = ""
	return CreateLineItemRequest{
		InsertionOrderID: testInsertionOrderID, DisplayName: name, LineItemType: LineItemDisplayDefault,
		Flight: line.Flight, Budget: line.Budget, Pacing: line.Pacing,
		PartnerRevenueModel: line.PartnerRevenueModel, BidStrategy: line.BidStrategy,
		FrequencyCap: line.FrequencyCap, ContainsEUPoliticalAds: line.ContainsEUPoliticalAds,
	}
}

func TestWorkflowValidationOwnershipConflictsAndUnsupported(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		switch {
		case strings.HasSuffix(request.URL.Path, "/campaigns/"+testCampaignID):
			campaign := campaignResource(testCampaignID, "Parent", EntityStatusPaused)
			writeJSON(t, writer, http.StatusOK, campaign)
		case strings.HasSuffix(request.URL.Path, "/insertionOrders/"+testInsertionOrderID):
			writeJSON(t, writer, http.StatusOK, insertionOrderResource(testInsertionOrderID, testCampaignID, "Parent IO", EntityStatusDraft))
		case strings.HasSuffix(request.URL.Path, "/lineItems/"+testLineItemID):
			line := lineItemResource(testLineItemID, "YouTube", EntityStatusDraft)
			line.LineItemType = "LINE_ITEM_TYPE_YOUTUBE_AND_PARTNERS_ACTION"
			writeJSON(t, writer, http.StatusOK, line)
		default:
			t.Fatalf("unexpected request=%s", request.URL)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()

	before := requestCount
	for _, invoke := range []func() error{
		func() error { _, err := client.GetCampaign(ctx, "bad"); return err },
		func() error { _, err := client.ListCampaigns(ctx, ListRequest{PageSize: 201}); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			_, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { _, err := client.CreateInsertionOrder(ctx, CreateInsertionOrderRequest{}); return err },
		func() error { _, err := client.CreateLineItem(ctx, CreateLineItemRequest{}); return err },
		func() error { _, err := client.DuplicateLineItem(ctx, "bad", DuplicateLineItemRequest{}); return err },
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation error=%v", err)
		}
	}
	if requestCount != before {
		t.Fatalf("invalid calls made requests: before=%d after=%d", before, requestCount)
	}

	active := EntityStatusActive
	if _, err := client.UpdateInsertionOrder(ctx, testInsertionOrderID, UpdateInsertionOrderRequest{EntityStatus: &active}); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("insertion order activation error=%v", err)
	}
	name := "Renamed"
	if _, err := client.UpdateLineItem(ctx, testLineItemID, UpdateLineItemRequest{DisplayName: &name}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported line update error=%v", err)
	}
	if _, err := client.DuplicateLineItem(ctx, testLineItemID, DuplicateLineItemRequest{
		TargetDisplayName: "Copy", ContainsEUPoliticalAds: DoesNotContainEUPoliticalAdvertising,
	}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported duplicate error=%v", err)
	}
}
