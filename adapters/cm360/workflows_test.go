package cm360

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestProfileTraffickingAndReportingWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /userprofiles/" + testProfileID:
			writeJSON(t, writer, http.StatusOK, profileResource())
		case http.MethodGet + " /userprofiles/" + testProfileID + "/advertisers/" + testAdvertiserID:
			writeJSON(t, writer, http.StatusOK, advertiserResource())
		case http.MethodGet + " /userprofiles/" + testProfileID + "/campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, campaignResource(testCampaignID, "Existing campaign", false))
		case http.MethodGet + " /userprofiles/" + testProfileID + "/campaigns":
			query := request.URL.Query()
			if query["advertiserIds"][0] != testAdvertiserID || query.Get("maxResults") != "20" ||
				query.Get("archived") != "false" || query.Get("sortField") != "NAME" || query.Get("sortOrder") != "DESCENDING" {
				t.Errorf("campaign query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, listCampaignsResponse{
				Campaigns: []Campaign{campaignResource(testCampaignID, "Existing campaign", false)}, NextPageToken: "campaign-next",
			})
		case http.MethodPost + " /userprofiles/" + testProfileID + "/campaigns":
			var payload campaignCreatePayload
			decodeJSONBody(t, request, &payload)
			if !payload.Archived || payload.AdvertiserID != testAdvertiserID || payload.Name != "New campaign" {
				t.Errorf("campaign create=%#v", payload)
			}
			created := campaignResource("334", payload.Name, payload.Archived)
			created.StartDate, created.EndDate = payload.StartDate, payload.EndDate
			writeJSON(t, writer, http.StatusOK, created)
		case http.MethodPatch + " /userprofiles/" + testProfileID + "/campaigns":
			if request.URL.Query().Get("id") != testCampaignID {
				t.Errorf("campaign patch query=%v", request.URL.Query())
			}
			var payload campaignPatchPayload
			decodeJSONBody(t, request, &payload)
			updated := campaignResource(testCampaignID, *payload.Name, *payload.Archived)
			writeJSON(t, writer, http.StatusOK, updated)
		case http.MethodGet + " /userprofiles/" + testProfileID + "/placements/" + testPlacementID:
			writeJSON(t, writer, http.StatusOK, placementResource())
		case http.MethodGet + " /userprofiles/" + testProfileID + "/placements":
			query := request.URL.Query()
			if query["advertiserIds"][0] != testAdvertiserID || query.Get("activeStatus") != string(PlacementActive) ||
				query["campaignIds"][0] != testCampaignID {
				t.Errorf("placement query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, listPlacementsResponse{Placements: []Placement{placementResource()}, NextPageToken: "placement-next"})
		case http.MethodGet + " /userprofiles/" + testProfileID + "/ads/" + testAdID:
			writeJSON(t, writer, http.StatusOK, adResource())
		case http.MethodGet + " /userprofiles/" + testProfileID + "/ads":
			query := request.URL.Query()
			if query.Get("advertiserId") != testAdvertiserID || query.Get("active") != "true" || query.Get("type") != string(AdStandard) {
				t.Errorf("ad query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, listAdsResponse{Ads: []Ad{adResource()}, NextPageToken: "ad-next"})
		case http.MethodPost + " /userprofiles/" + testProfileID + "/reportdata/query":
			var query ReportDataQueryRequest
			decodeJSONBody(t, request, &query)
			if len(query.DimensionFilters) != 1 || query.DimensionFilters[0].DimensionName != "advertiser" ||
				query.DimensionFilters[0].ID != testAdvertiserID || query.DimensionFilters[0].MatchType != "EXACT" {
				t.Errorf("report query=%#v", query)
			}
			writeJSON(t, writer, http.StatusOK, ReportDataResponse{
				ColumnHeaders: []ColumnHeader{{Name: "campaign", Type: "DIMENSION"}, {Name: "impressions", Type: "METRIC"}},
				Rows:          []ReportDataRow{{Values: []string{"Campaign", "100"}}}, TotalRow: &ReportDataRow{Values: []string{"", "100"}},
				NextPageToken: "report-data-next",
			})
		case http.MethodGet + " /userprofiles/" + testProfileID + "/reports/" + testReportID:
			writeJSON(t, writer, http.StatusOK, reportResource())
		case http.MethodGet + " /userprofiles/" + testProfileID + "/reports":
			if request.URL.Query().Get("scope") != "MINE" || request.URL.Query().Get("maxResults") != "10" {
				t.Errorf("report list query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, listReportsResponse{Items: []Report{reportResource()}, NextPageToken: "report-next"})
		case http.MethodPost + " /userprofiles/" + testProfileID + "/reports/" + testReportID + "/run":
			if request.URL.Query().Get("synchronous") != "false" {
				t.Errorf("run query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, reportFileResource(ReportFileQueued))
		case http.MethodGet + " /userprofiles/" + testProfileID + "/reports/" + testReportID + "/files/" + testFileID:
			writeJSON(t, writer, http.StatusOK, reportFileResource(ReportFileAvailable))
		case http.MethodGet + " /userprofiles/" + testProfileID + "/reports/" + testReportID + "/files":
			writeJSON(t, writer, http.StatusOK, listReportFilesResponse{Items: []ReportFile{reportFileResource(ReportFileAvailable)}, NextPageToken: "file-next"})
		case http.MethodGet + " /reports/" + testReportID + "/files/" + testFileID:
			if request.URL.Query().Get("alt") != "media" || request.Header.Get("Range") != "bytes=0-3" ||
				request.Header.Get("Accept") != "application/octet-stream" {
				t.Errorf("download=%v headers=%v", request.URL, request.Header)
			}
			writer.Header().Set("Content-Range", "bytes 0-3/4")
			writer.Header().Set("Content-Length", "4")
			writer.WriteHeader(http.StatusPartialContent)
			_, _ = writer.Write([]byte("data"))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()

	if profile, err := client.GetProfile(ctx); err != nil || profile.ProfileID != testProfileID {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	if advertiser, err := client.GetAdvertiser(ctx); err != nil || advertiser.ID != testAdvertiserID {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	if campaign, err := client.GetCampaign(ctx, testCampaignID); err != nil || campaign.ID != testCampaignID {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	archived := false
	campaigns, err := client.ListCampaigns(ctx, CampaignListRequest{
		MaxResults: 20, Archived: &archived, SortField: CampaignSortName, SortOrder: SortDescending,
	})
	if err != nil || len(campaigns.Items) != 1 || campaigns.NextPageToken != "campaign-next" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	created, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "New campaign", StartDate: "2026-08-10", EndDate: "2026-12-31"})
	if err != nil || !created.Archived || created.ID != "334" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	name, archive := "Renamed campaign", true
	updated, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: &name, Archived: &archive})
	if err != nil || updated.Name != name || !updated.Archived {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}

	if placement, err := client.GetPlacement(ctx, testPlacementID); err != nil || placement.ID != testPlacementID {
		t.Fatalf("placement=%#v err=%v", placement, err)
	}
	placements, err := client.ListPlacements(ctx, PlacementListRequest{CampaignIDs: []string{testCampaignID}, ActiveStatus: PlacementActive})
	if err != nil || len(placements.Items) != 1 || placements.NextPageToken != "placement-next" {
		t.Fatalf("placements=%#v err=%v", placements, err)
	}
	if ad, err := client.GetAd(ctx, testAdID); err != nil || ad.ID != testAdID {
		t.Fatalf("ad=%#v err=%v", ad, err)
	}
	active := true
	ads, err := client.ListAds(ctx, AdListRequest{Active: &active, Type: AdStandard})
	if err != nil || len(ads.Items) != 1 || ads.NextPageToken != "ad-next" {
		t.Fatalf("ads=%#v err=%v", ads, err)
	}

	data, err := client.QueryReportData(ctx, validReportQuery(), socialhub.WithRequestID("caller-request"))
	if err != nil || len(data.Rows) != 1 || data.NextPageToken != "report-data-next" {
		t.Fatalf("data=%#v err=%v", data, err)
	}
	if report, err := client.GetReport(ctx, testReportID); err != nil || report.ID != testReportID {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	reports, err := client.ListReports(ctx, ReportListRequest{MaxResults: 10})
	if err != nil || len(reports.Items) != 1 || reports.NextPageToken != "report-next" {
		t.Fatalf("reports=%#v err=%v", reports, err)
	}
	if file, err := client.RunReport(ctx, testReportID, false); err != nil || file.Status != ReportFileQueued {
		t.Fatalf("run file=%#v err=%v", file, err)
	}
	if file, err := client.GetReportFile(ctx, testReportID, testFileID); err != nil || file.Status != ReportFileAvailable {
		t.Fatalf("file=%#v err=%v", file, err)
	}
	files, err := client.ListReportFiles(ctx, testReportID, ReportFileListRequest{})
	if err != nil || len(files.Items) != 1 || files.NextPageToken != "file-next" {
		t.Fatalf("files=%#v err=%v", files, err)
	}
	var output bytes.Buffer
	result, err := client.DownloadReportFileRange(ctx, testReportID, testFileID, ByteRange{Start: 0, End: 3}, &output)
	if err != nil || output.String() != "data" || result.BytesWritten != 4 || !result.Complete || result.ContentRange != "bytes 0-3/4" {
		t.Fatalf("download=%#v body=%q err=%v", result, output.String(), err)
	}
}

func validReportQuery() ReportDataQueryRequest {
	return ReportDataQueryRequest{
		DateRange:      DateRange{StartDate: "2026-08-01", EndDate: "2026-08-08"},
		DimensionNames: []string{"campaign"}, MetricNames: []string{"impressions"}, MaxResults: 100,
	}
}
