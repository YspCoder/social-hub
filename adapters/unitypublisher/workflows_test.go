package unitypublisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAllPublisherManageWorkflows(t *testing.T) {
	organizationPath := "/public/v1/organizations/" + testOrganizationID
	applicationPath := organizationPath + "/applications/" + testApplicationID
	placementPath := applicationPath + "/placements/" + testPlacementID
	devicePath := organizationPath + "/test-devices/" + testDeviceID
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertBearerRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " " + organizationPath + "/applications":
			writeJSON(t, writer, http.StatusOK, []Application{applicationFixture()})
		case http.MethodPost + " " + organizationPath + "/applications":
			assertDryRun(t, request)
			var payload CreateApplicationRequest
			decodeJSONBody(t, request, &payload)
			if payload.Name != "My Game" || payload.Platform != PlatformAndroid || payload.ProjectName == nil || *payload.ProjectName != "Shared Game" || payload.Privacy == nil {
				t.Errorf("application create payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, applicationFixture())
		case http.MethodGet + " " + applicationPath:
			if request.Header.Get("X-Request-ID") != "caller-request" {
				t.Errorf("X-Request-ID=%q", request.Header.Get("X-Request-ID"))
			}
			writeJSON(t, writer, http.StatusOK, applicationFixture())
		case http.MethodPatch + " " + applicationPath:
			assertDryRun(t, request)
			var payload UpdateApplicationRequest
			decodeJSONBody(t, request, &payload)
			if payload.Name == nil || *payload.Name != "My Renamed Game" || payload.KidsSettings == nil || !*payload.KidsSettings {
				t.Errorf("application update payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, applicationFixture())
		case http.MethodGet + " " + applicationPath + "/test-mode":
			writeJSON(t, writer, http.StatusOK, applicationTestModeFixture())
		case http.MethodPatch + " " + applicationPath + "/test-mode":
			assertDryRun(t, request)
			var payload UpdateTestModeRequest
			decodeJSONBody(t, request, &payload)
			if payload.TestMode != TestModeForceOff {
				t.Errorf("test mode payload=%#v", payload)
			}
			response := applicationTestModeFixture()
			response.TestMode = testModePointer(TestModeForceOff)
			writeJSON(t, writer, http.StatusOK, response)
		case http.MethodGet + " " + applicationPath + "/placements":
			query := request.URL.Query()
			if query.Get("isArchived") != "false" || !slices.Equal(query["adFormat"], []string{"rewarded", "banner"}) {
				t.Errorf("placement list query=%v", query)
			}
			writeJSON(t, writer, http.StatusOK, []Placement{placementFixture()})
		case http.MethodPost + " " + applicationPath + "/placements":
			assertDryRun(t, request)
			var payload struct {
				Name          string                 `json:"name"`
				AdFormat      AdFormat               `json:"adFormat"`
				Configuration RewardedConfigurations `json:"adFormatConfigurations"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Name != "Rewarded Placement" || payload.AdFormat != AdFormatRewarded || payload.Configuration.Name != "coins" || payload.Configuration.Value != 100 {
				t.Errorf("placement create payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, placementFixture())
		case http.MethodGet + " " + placementPath:
			writeJSON(t, writer, http.StatusOK, placementFixture())
		case http.MethodPut + " " + placementPath:
			assertDryRun(t, request)
			var payload map[string]json.RawMessage
			decodeJSONBody(t, request, &payload)
			if string(payload["adFormat"]) != `"banner"` || len(payload["adFormatConfigurations"]) == 0 {
				t.Errorf("placement update payload=%s", payload)
			}
			response := placementFixture()
			response.AdFormat = AdFormatBanner
			writeJSON(t, writer, http.StatusOK, response)
		case http.MethodDelete + " " + placementPath:
			assertDryRun(t, request)
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodPatch + " " + placementPath + "/restore":
			assertDryRun(t, request)
			writer.WriteHeader(http.StatusOK)
		case http.MethodGet + " " + organizationPath + "/placements":
			writeJSON(t, writer, http.StatusOK, []OrganizationPlacement{organizationPlacementFixture()})
		case http.MethodGet + " " + organizationPath + "/test-devices":
			if request.URL.Query().Get("platform") != "iOS" {
				t.Errorf("platform query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, []TestDevice{testDeviceFixture()})
		case http.MethodPost + " " + organizationPath + "/test-devices":
			assertDryRun(t, request)
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if payload["platform"] != "iOS" || payload["name"] != "QA iPhone 15" {
				t.Errorf("test device create payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusCreated, testDeviceFixture())
		case http.MethodGet + " " + devicePath:
			writeJSON(t, writer, http.StatusOK, testDeviceFixture())
		case http.MethodPatch + " " + devicePath:
			assertDryRun(t, request)
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			value, exists := payload["platform"]
			if !exists || value != nil || payload["name"] != "QA device" {
				t.Errorf("test device update payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, testDeviceFixture())
		case http.MethodDelete + " " + devicePath:
			assertDryRun(t, request)
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	dryRun := MutationOptions{DryRun: true}

	if values, err := client.ListApplications(ctx); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("applications=%#v err=%v", values, err)
	}
	createApplication := CreateApplicationRequest{
		Name: "My Game", Platform: PlatformAndroid, Privacy: &Privacy{}, ProjectName: stringPointer("Shared Game"),
	}
	if value, err := client.CreateApplication(ctx, createApplication, dryRun); err != nil || value.ID != testApplicationID {
		t.Fatalf("created application=%#v err=%v", value, err)
	}
	if value, err := client.GetApplication(ctx, testApplicationID, socialhub.WithRequestID("caller-request")); err != nil || value.ID != testApplicationID {
		t.Fatalf("application=%#v err=%v", value, err)
	}
	if value, err := client.UpdateApplication(ctx, testApplicationID, UpdateApplicationRequest{Name: stringPointer("My Renamed Game"), KidsSettings: boolPointer(true)}, dryRun); err != nil || value.ID != testApplicationID {
		t.Fatalf("updated application=%#v err=%v", value, err)
	}
	if value, err := client.GetApplicationTestMode(ctx, testApplicationID); err != nil || value.TestMode == nil || *value.TestMode != TestModeForceAll || len(value.Raw) == 0 {
		t.Fatalf("test mode=%#v err=%v", value, err)
	}
	if value, err := client.UpdateApplicationTestMode(ctx, testApplicationID, UpdateTestModeRequest{TestMode: TestModeForceOff}, dryRun); err != nil || value.TestMode == nil || *value.TestMode != TestModeForceOff {
		t.Fatalf("updated test mode=%#v err=%v", value, err)
	}
	archived := false
	if values, err := client.ListApplicationPlacements(ctx, testApplicationID, ListApplicationPlacementsRequest{IsArchived: &archived, AdFormats: []AdFormat{AdFormatRewarded, AdFormatBanner}}); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("placements=%#v err=%v", values, err)
	}
	rewarded := PlacementRequest{Name: "Rewarded Placement", AdFormat: AdFormatRewarded, AdFormatConfigurations: RewardedConfigurations{Name: "coins", Value: 100}}
	if value, err := client.CreatePlacement(ctx, testApplicationID, rewarded, dryRun); err != nil || value.ID != testPlacementID {
		t.Fatalf("created placement=%#v err=%v", value, err)
	}
	if value, err := client.GetPlacement(ctx, testApplicationID, testPlacementID); err != nil || value.ID != testPlacementID {
		t.Fatalf("placement=%#v err=%v", value, err)
	}
	banner := PlacementRequest{Name: "Banner Placement", AdFormat: AdFormatBanner, AdFormatConfigurations: BannerConfigurations{BannerRefreshRate: 30}}
	if value, err := client.UpdatePlacement(ctx, testApplicationID, testPlacementID, banner, dryRun); err != nil || value.AdFormat != AdFormatBanner {
		t.Fatalf("updated placement=%#v err=%v", value, err)
	}
	if err := client.ArchivePlacement(ctx, testApplicationID, testPlacementID, dryRun); err != nil {
		t.Fatal(err)
	}
	if err := client.RestorePlacement(ctx, testApplicationID, testPlacementID, dryRun); err != nil {
		t.Fatal(err)
	}
	if values, err := client.ListOrganizationPlacements(ctx); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("organization placements=%#v err=%v", values, err)
	}
	if values, err := client.ListTestDevices(ctx, PlatformIOS); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("test devices=%#v err=%v", values, err)
	}
	platform := PlatformIOS
	createDevice := CreateTestDeviceRequest{Platform: &NullablePlatform{Value: &platform}, Name: "QA iPhone 15", AdvertisingID: "AEBE52E7-03EE-455A-B3C4-E57283966239"}
	if value, err := client.CreateTestDevice(ctx, createDevice, dryRun); err != nil || value.ID != testDeviceID {
		t.Fatalf("created device=%#v err=%v", value, err)
	}
	if value, err := client.GetTestDevice(ctx, testDeviceID); err != nil || value.ID != testDeviceID {
		t.Fatalf("device=%#v err=%v", value, err)
	}
	if value, err := client.UpdateTestDevice(ctx, testDeviceID, UpdateTestDeviceRequest{Platform: &NullablePlatform{}, Name: stringPointer("QA device")}, dryRun); err != nil || value.ID != testDeviceID {
		t.Fatalf("updated device=%#v err=%v", value, err)
	}
	if err := client.DeleteTestDevice(ctx, testDeviceID, dryRun); err != nil {
		t.Fatal(err)
	}
}

func assertDryRun(t *testing.T, request *http.Request) {
	t.Helper()
	if request.URL.Query().Get("dryrun") != "true" {
		t.Errorf("dryrun query=%v", request.URL.Query())
	}
}

func testModePointer(value TestMode) *TestMode { return &value }
