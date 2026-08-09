package unityadvertising

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestUnityErrorClassificationRateLimitsAndRedaction(t *testing.T) {
	body := []byte(`{"title":"Too Many Requests","detail":"access_token=secret-value exhausted","details":[{"code":"BAD","path":"bid.value","message":"secret: hidden"}],"code":50,"type":"https://services.docs.unity.com/docs/errors/#50","status":429,"requestId":"request-body"}`)
	header := http.Header{
		"Retry-After": {"2.5"}, "RateLimit-Policy": {"10;w=1, 4000;w=1800"},
		"RateLimit": {"limit=10, remaining=0, reset=1"}, "Unity-RateLimit": {"limit=10, remaining=0, reset=1; limit=4000, remaining=0, reset=42"},
	}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RetryAfter != 2500*time.Millisecond ||
		api.Hub.RequestID != "request-body" || strings.Contains(api.Hub.PlatformMessage, "secret-value") || strings.Contains(api.Unity.Details[0].Message, "hidden") ||
		api.Hub.PlatformCode != "50" || api.RateLimitPolicy == "" || api.RateLimit == "" || api.UnityRateLimit == "" {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) || api.Error() == "" || api.Unwrap() == nil {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestErrorClassificationAndQuotaPolicy(t *testing.T) {
	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{403, "63", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "64", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "65", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{400, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{413, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{415, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{422, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{402, "", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{410, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{424, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{429, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{503, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.code)
		if code != test.want || class != test.class {
			t.Errorf("status=%d platformCode=%q got=%s/%s", test.status, test.code, code, class)
		}
	}
	policy := DefaultQuotaPolicy()
	if policy.Apps.RequestsPerSecond != 20 || policy.Bids.RequestsPerSecond != 10 || policy.Campaigns.RequestsPer30Minute != 4000 ||
		policy.Create.RequestsPerSecond != 1 || policy.Create.RequestsPer30Minute != 30 || policy.Mutation.RequestsPer30Minute != 100 {
		t.Fatalf("quota policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" ||
		redactSensitive("secret_key: topsecret") == "secret_key: topsecret" || boundedMessage(strings.Repeat("x", 20), 5) != "xxxxx" {
		t.Fatal("error helpers failed")
	}
	malformed := decodeHTTPError(http.StatusBadRequest, http.Header{"X-Request-Id": {"request-header"}}, []byte("bad request"))
	if fallback := malformed.(*APIError); fallback.Hub.RequestID != "request-header" || fallback.Hub.PlatformCode != "http_400" {
		t.Fatalf("fallback error=%#v", fallback)
	}
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	if parseRetryAfter(future) <= 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("Retry-After date parsing failed")
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.test?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil || hubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError ||
		hubError(t, platformError("test", socialhub.CodeConflict, socialhub.ClassPermanent, nil)).Code != socialhub.CodeConflict {
		t.Fatal("typed error helpers failed")
	}
	hub := &socialhub.Error{Code: socialhub.CodeRateLimited}
	if withOperation(hub, "mapped") != hub || hub.Op != "mapped" || withOperation(nil, "none") != nil {
		t.Fatal("operation mapping failed")
	}
}

func TestValidationPrimitivesAndNullableJSON(t *testing.T) {
	if !validEndpoint("https://example.test/path") || validEndpoint("https://user:pass@example.test") || validEndpoint("mailto:test@example.test") ||
		!validOrganizationID(testOrganizationID) || validOrganizationID("01") || validOrganizationID("9223372036854775808") ||
		!validMongoID(testCampaignID) || validMongoID("bad") || !validCountry("US") || validCountry("ZZ") ||
		!validBid("1.250") || validBid("1000") || !validResponseMaxBid("1000.000") || !validROASGoal("25.50") ||
		!validMoney("0.00") || !validPositiveMoney("0.10") || validPositiveMoney("0.00") || !validTotalMoney("0.00") ||
		!validSourceAppID(testSourceAppID) || validSourceAppID("short") || !validStore(StoreStandaloneAndroid) || validStore("other") ||
		!validCreativeLanguage(LanguageEnglish) || validCreativeLanguage("xx") || !validDate("2026-08-10") || validDate("2026-99-99") ||
		!validUploadFilename("image.PNG", ".png") || validUploadFilename("../image.png", ".png") ||
		!finiteNonNegative(0) || finiteNonNegative(math.NaN()) || finiteNonNegative(-1) {
		t.Fatal("validation primitive mismatch")
	}
	encoded, err := jsonMarshal(struct {
		Omitted *NullableString `json:"omitted,omitempty"`
		Null    *NullableString `json:"null"`
		Value   *NullableString `json:"value"`
	}{Null: NewNullString(), Value: NewNullableString("set")})
	if err != nil || string(encoded) != `{"null":null,"value":"set"}` {
		t.Fatalf("nullable JSON=%s err=%v", encoded, err)
	}
	if mediaTypeForFilename("image.unknown") != "application/octet-stream" {
		t.Fatal("unknown media type fallback failed")
	}
	patches := []struct {
		value any
		want  string
	}{
		{CPIBidPatch{Country: "US", Bid: nil}, `{"country":"US","bid":null}`},
		{SourceBidPatch{Country: "US", SourceAppID: testSourceAppID, Bid: nil}, `{"country":"US","sourceAppId":"` + testSourceAppID + `","bid":null}`},
		{ROASBidPatch{Country: "US", Goal: nil}, `{"country":"US","goal":null}`},
		{RetentionBidPatch{Country: "US", BaseBid: nil, MaxBid: "2.000"}, `{"country":"US","baseBid":null,"maxBid":"2.000"}`},
		{EventOptimizationBidPatch{Country: "US", Bid: nil}, `{"country":"US","bid":null}`},
	}
	for _, patch := range patches {
		encoded, err := json.Marshal(patch.value)
		if err != nil || string(encoded) != patch.want {
			t.Errorf("patch JSON=%s want=%s err=%v", encoded, patch.want, err)
		}
	}
}

func TestCampaignBudgetAndCreativeVariants(t *testing.T) {
	common := CampaignCreateBase{Name: "Campaign", BillingType: BillingCPI}
	validCampaigns := []CampaignCreateRequest{
		CreateInstallsCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalInstalls},
		&CreateRetentionCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalRetention},
		CreateROASCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalROAS, ROASTypes: []ROASType{ROASTypeIAP}, PostInstallWindow: PostInstallD7},
		&CreateCreativeTestingCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalCreativeTesting},
		CreateEventOptimizationCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalEventOptimization, EventOptimizationType: "purchase", SDKEventName: NewNullString()},
	}
	for _, input := range validCampaigns {
		if err := validateCreateCampaign(input); err != nil {
			t.Errorf("valid campaign rejected: %#v err=%v", input, err)
		}
	}
	badStrategy := common
	badStrategy.BiddingStrategy = BiddingManual
	invalidCampaigns := []CampaignCreateRequest{
		CreateInstallsCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalROAS},
		CreateRetentionCampaignRequest{CampaignCreateBase: badStrategy, Goal: CampaignGoalRetention},
		CreateROASCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalROAS, ROASTypes: []ROASType{"bad"}},
		CreateEventOptimizationCampaignRequest{CampaignCreateBase: common, Goal: CampaignGoalEventOptimization, SDKEventName: NewNullableString("")},
	}
	for _, input := range invalidCampaigns {
		if err := validateCreateCampaign(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("invalid campaign accepted: %#v", input)
		}
	}
	var nilROAS *CreateROASCampaignRequest
	if err := validateCreateCampaign(nilROAS); !errors.Is(err, socialhub.ErrInvalidArgument) ||
		validateUpdateCampaign(UpdateCampaignRequest{Name: stringPointer("Updated")}) != nil ||
		!errors.Is(validateUpdateCampaign(UpdateCampaignRequest{}), socialhub.ErrInvalidArgument) {
		t.Fatal("campaign pointer or update validation failed")
	}

	total, daily := Money("2500.00"), Money("500.00")
	countryMap := map[CountryCode]Money{"US": "100.00", "CA": "50.00"}
	groups := []CountryBudgetGroup{{Name: "North America", Countries: []CountryCode{"US", "CA"}, Limit: "500.00"}}
	validBudgets := []CampaignBudgetUpdate{
		DailyBudgetUpdate{Total: &total, Daily: &daily},
		&CountryBudgetUpdate{Total: &total, DailyPerCountry: countryMap},
		CountryGroupBudgetUpdate{Total: &total, DailyPerCountryGroup: groups},
	}
	for _, input := range validBudgets {
		if err := validateCampaignBudgetUpdate(input); err != nil {
			t.Errorf("valid budget rejected: %#v err=%v", input, err)
		}
	}
	if !validCampaignBudget(CampaignBudget{DailyPerCountry: countryMap}) || !validCampaignBudget(CampaignBudget{DailyPerCountryGroup: groups}) ||
		validCampaignBudget(CampaignBudget{DailyPerCountry: map[CountryCode]Money{}}) ||
		validCampaignBudget(CampaignBudget{Daily: &daily, DailyPerCountry: countryMap}) || validCountryBudgetMap(map[CountryCode]Money{"ZZ": "1.00"}) ||
		validCountryBudgetGroups([]CountryBudgetGroup{{Name: "bad!", Countries: []CountryCode{"US"}, Limit: "1.00"}}) {
		t.Fatal("budget union validation failed")
	}
	var nilDaily *DailyBudgetUpdate
	for _, input := range []CampaignBudgetUpdate{DailyBudgetUpdate{}, CountryBudgetUpdate{}, CountryGroupBudgetUpdate{}, nilDaily} {
		if err := validateCampaignBudgetUpdate(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("invalid budget accepted: %#v", input)
		}
	}

	image := []byte("image")
	uploads := []CreativeUpload{
		SquareEndCardUpload{Name: "Square", Language: LanguageEnglish, Filename: "square.png", Size: int64(len(image)), File: bytes.NewReader(image)},
		&EndCardPairUpload{Name: "Pair", Language: LanguageEnglish, PortraitFilename: "portrait.jpg", PortraitSize: int64(len(image)), PortraitFile: bytes.NewReader(image), LandscapeFilename: "landscape.gif", LandscapeSize: int64(len(image)), LandscapeFile: bytes.NewReader(image)},
		VideoUpload{Name: "Video", Language: LanguageEnglish, Filename: "video.mp4", Size: int64(len(image)), File: bytes.NewReader(image)},
		&PlayableUpload{Name: "Playable", Language: LanguageEnglish, Filename: "playable.html", Orientation: PlayableBoth, Size: int64(len(image)), File: bytes.NewReader(image)},
	}
	for _, upload := range uploads {
		metadata, parts, err := creativeUploadParts(upload)
		if err != nil || metadata == nil || len(parts) == 0 {
			t.Errorf("creative upload=%#v metadata=%v parts=%v err=%v", upload, metadata, parts, err)
		}
	}
	var nilPlayable *PlayableUpload
	if _, _, err := creativeUploadParts(nilPlayable); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil creative error=%v", err)
	}
	buffer := new(bytes.Buffer)
	writer := multipart.NewWriter(buffer)
	err := writeCreativeMultipart(writer, map[string]string{"name": "bad size"}, []creativeUploadPart{{field: "videoFile", filename: "video.mp4", mediaType: "video/mp4", size: 10, reader: bytes.NewReader(image)}})
	if !errors.Is(err, errCreativeSizeMismatch) {
		t.Fatalf("size mismatch error=%v", err)
	}
	buffer.Reset()
	writer = multipart.NewWriter(buffer)
	boundary := writer.Boundary()
	err = writeCreativeMultipart(writer, map[string]string{"name": "headers"}, []creativeUploadPart{{
		field: "squareEndCardFile", filename: "IMAGE.PNG", mediaType: mediaTypeForFilename("IMAGE.PNG"), size: int64(len(image)), reader: bytes.NewReader(image),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(buffer.Bytes()), boundary)
	metadataPart, err := reader.NextPart()
	if err != nil || metadataPart.FormName() != "creativeInfo" || metadataPart.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("metadata part=%#v err=%v", metadataPart, err)
	}
	filePart, err := reader.NextPart()
	if err != nil || filePart.FormName() != "squareEndCardFile" || filePart.FileName() != "IMAGE.PNG" || filePart.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("file part=%#v err=%v", filePart, err)
	}
	if validPlayableOrientation("bad") || !validPlayableOrientation(PlayableBoth) {
		t.Fatal("playable orientation validation failed")
	}
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

type unsupportedUpload struct{}

func (unsupportedUpload) isCreativeUpload() {}

func TestInvalidCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	badID := "bad"
	for _, invoke := range []func() error{
		func() error { _, err := client.ListApps(ctx, ListAppsRequest{Limit: 1001}); return err },
		func() error { _, err := client.GetApp(ctx, badID); return err },
		func() error { _, err := client.CreateApp(ctx, (*CreateAppleAppRequest)(nil)); return err },
		func() error { _, err := client.UpdateApp(ctx, testCampaignSetID, UpdateAppRequest{}); return err },
		func() error { return client.DeleteApp(ctx, badID) },
		func() error { _, err := client.ListCreatives(ctx, badID, ListCreativesRequest{}); return err },
		func() error { _, err := client.CreateCreative(ctx, testCampaignSetID, unsupportedUpload{}); return err },
		func() error { _, err := client.GetCreative(ctx, testCampaignSetID, badID); return err },
		func() error {
			_, err := client.ListCreativePacks(ctx, testCampaignSetID, ListCreativePacksRequest{Limit: 701})
			return err
		},
		func() error {
			_, err := client.CreateCreativePack(ctx, testCampaignSetID, CreateCreativePackRequest{})
			return err
		},
		func() error { _, err := client.GetCreativePack(ctx, testCampaignSetID, badID); return err },
		func() error {
			_, err := client.UpdateCreativePack(ctx, testCampaignSetID, testCreativePackID, UpdateCreativePackRequest{})
			return err
		},
		func() error { return client.DeleteCreativePack(ctx, testCampaignSetID, badID) },
		func() error {
			_, err := client.CreateCampaign(ctx, testCampaignSetID, (*CreateInstallsCampaignRequest)(nil))
			return err
		},
		func() error {
			_, err := client.GetCampaign(ctx, testCampaignSetID, testCampaignID, GetCampaignRequest{IncludeFields: []CampaignIncludeField{"bad"}})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(ctx, testCampaignSetID, testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { return client.DeleteCampaign(ctx, testCampaignSetID, badID) },
		func() error {
			_, err := client.AssignCreativePack(ctx, testCampaignSetID, testCampaignID, badID)
			return err
		},
		func() error { return client.UnassignCreativePack(ctx, testCampaignSetID, testCampaignID, badID) },
		func() error {
			_, err := client.UpdateTargeting(ctx, testCampaignSetID, testCampaignID, Targeting{AppTargeting: &AppTargetingOptions{AllowList: stringSlicePointer([]string{testSourceAppID}), BlockList: stringSlicePointer([]string{testSourceAppID})}})
			return err
		},
		func() error {
			_, err := client.UpdateCampaignBudget(ctx, testCampaignSetID, testCampaignID, DailyBudgetUpdate{})
			return err
		},
		func() error {
			_, err := client.ReplaceCPIBids(ctx, testCampaignSetID, testCampaignID, []CPIBid{{Country: "ZZ", Bid: "1.00"}})
			return err
		},
		func() error {
			_, err := client.PatchSourceBids(ctx, testCampaignSetID, testCampaignID, []SourceBidPatch{{Country: "US", SourceAppID: "bad"}})
			return err
		},
		func() error {
			_, err := client.ReplaceROASBids(ctx, testCampaignSetID, testCampaignID, []ROASBidReplace{{Country: "US", Goal: "bad"}})
			return err
		},
		func() error {
			_, err := client.PatchRetentionBids(ctx, testCampaignSetID, testCampaignID, []RetentionBidPatch{{Country: "US", MaxBid: "bad"}})
			return err
		},
		func() error {
			_, err := client.ReplaceEventOptimizationBids(ctx, testCampaignSetID, testCampaignID, []EventOptimizationBid{{Country: "ZZ", Bid: "1.00"}})
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("validation error=%v", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("invalid calls made %d requests", requestCount)
	}
}
