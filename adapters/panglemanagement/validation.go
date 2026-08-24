package panglemanagement

import (
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumFilterValues = 500
	maximumRewardCount  = int64(9_007_199_254_740_991)
)

var supportedCPMRegions = map[Region]struct{}{
	"jp": {}, "kr": {}, "tw": {}, "my": {}, "th": {}, "vn": {}, "sa": {}, "ae": {},
	"eg": {}, "tr": {}, "id": {}, "ru": {}, "ph": {}, "sg": {}, "kh": {}, "ua": {},
	"il": {}, "by": {}, "kz": {}, "br": {}, "mx": {}, "ar": {}, "co": {}, "cl": {},
	"pe": {}, "ca": {}, "au": {}, "nz": {}, "za": {}, "pk": {}, "kw": {}, "iq": {},
	"ma": {}, "qa": {}, "jo": {}, "om": {}, "bh": {}, "lb": {},
}

func validURL(value string) bool {
	if len(value) > 2_048 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validResponseText(value string, maximum int) bool {
	return utf8.ValidString(value) && len(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl)
}

func validNumericID(value string) bool {
	if value == "" || len(value) > 19 || len(value) > 1 && value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0
}

func validName(value string) bool {
	if !validOpaque(value, 180) {
		return false
	}
	weight := 0
	for _, character := range value {
		if character <= unicode.MaxASCII {
			weight++
		} else {
			weight += 2
		}
		if weight > 60 {
			return false
		}
	}
	return weight >= 1
}

func validSHA1(value string) bool {
	compact := strings.ReplaceAll(value, ":", "")
	if len(compact) != 40 || strings.Contains(value, ":") && len(value) != 59 {
		return false
	}
	if strings.Contains(value, ":") {
		for index := 2; index < len(value); index += 3 {
			if value[index] != ':' {
				return false
			}
		}
	}
	for index := range compact {
		character := compact[index]
		if character < '0' || character > '9' {
			lower := character | 0x20
			if lower < 'a' || lower > 'f' {
				return false
			}
		}
	}
	return true
}

func validOS(value OSType) bool { return value == OSIOS || value == OSAndroid }

func validCOPPA(value COPPA) bool {
	return value == COPPAClientConfigured || value == COPPAOver12 || value == COPPAUnder13
}

func validAppStatus(value AppStatus) bool {
	switch value {
	case AppStatusResume, AppStatusReview, AppStatusLive, AppStatusRejected,
		AppStatusSuspended, AppStatusAborted, AppStatusTest:
		return true
	default:
		return false
	}
}

func validPlacementStatus(value PlacementStatus) bool {
	switch value {
	case PlacementStatusResume, PlacementStatusLive, PlacementStatusPaused, PlacementStatusTest:
		return true
	default:
		return false
	}
}

func validAdSlotType(value AdSlotType) bool {
	switch value {
	case AdSlotNative, AdSlotBanner, AdSlotAppOpen, AdSlotRewardedVideo, AdSlotInterstitial:
		return true
	default:
		return false
	}
}

func validBiddingType(value BiddingType) bool {
	return value == BiddingFixedCPM || value == BiddingInApp || value == BiddingClientSide
}

func validOrientation(value Orientation) bool {
	return value == OrientationVertical || value == OrientationHorizontal
}

func validAcceptMaterial(value AcceptMaterialType) bool {
	return value == AcceptImageOnly || value == AcceptVideoOnly || value == AcceptVideoAndImage
}

func validInterstitialMaterial(value AcceptMaterialType) bool {
	return value == AcceptVideoOnly || value == AcceptVideoAndImage
}

func validCategory(value AdCategory) bool {
	switch value {
	case AdCategoryVideo, AdCategoryWideImage, AdCategorySquareImage, AdCategorySquareVideo:
		return true
	default:
		return false
	}
}

func validCategories(values []AdCategory) bool {
	if len(values) == 0 || len(values) > 4 {
		return false
	}
	seen := make(map[AdCategory]struct{}, len(values))
	for _, value := range values {
		if !validCategory(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validMaskRuleID(value int64) bool { return value == -1 || value > 0 }

func validIDs(values []ID, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[ID]struct{}, len(values))
	for _, value := range values {
		if !validNumericID(string(value)) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validNames(values []string) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validName(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validOSValues(values []OSType) bool {
	if len(values) > 2 {
		return false
	}
	seen := make(map[OSType]struct{}, len(values))
	for _, value := range values {
		if !validOS(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validAppStatuses(values []AppStatus) bool {
	if len(values) > 7 {
		return false
	}
	seen := make(map[AppStatus]struct{}, len(values))
	for _, value := range values {
		if !validAppStatus(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPlacementTypes(values []AdSlotType) bool {
	if len(values) > 5 {
		return false
	}
	seen := make(map[AdSlotType]struct{}, len(values))
	for _, value := range values {
		if !validAdSlotType(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPlacementStatuses(values []PlacementStatus) bool {
	if len(values) > 4 {
		return false
	}
	seen := make(map[PlacementStatus]struct{}, len(values))
	for _, value := range values {
		if !validPlacementStatus(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validMoney(value Money, currency Currency) bool {
	text := string(value)
	if text == "" || len(text) > 64 || strings.TrimSpace(text) != text {
		return false
	}
	dot := false
	for index := range text {
		if text[index] == '.' {
			if dot || index == 0 || index == len(text)-1 {
				return false
			}
			dot = true
			continue
		}
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	valueRat, ok := new(big.Rat).SetString(text)
	if !ok || valueRat.Sign() < 0 {
		return false
	}
	limit := int64(500)
	if currency == CurrencyCNY {
		limit = 3_000
	} else if currency != CurrencyUSD {
		return false
	}
	return valueRat.Cmp(new(big.Rat).SetInt64(limit)) <= 0
}

func validCPMByRegion(values map[Region]Money, currency Currency) bool {
	if len(values) == 0 || len(values) > len(supportedCPMRegions) {
		return false
	}
	for region, value := range values {
		if _, supported := supportedCPMRegions[region]; !supported || !validMoney(value, currency) {
			return false
		}
	}
	return true
}

func validateCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Pangle Management API does not define caller request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Pangle Management API does not define an idempotency-key header")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "Pangle Management API does not support response field selection")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "call timeout must not be negative")
	}
	return resolved, nil
}

func validateCreateApp(input CreateAppRequest, sandbox bool) error {
	if sandbox && input.Status != AppStatusTest || !sandbox && input.Status != AppStatusLive {
		return invalidArgument("app_create", "status must be test in sandbox and live in production")
	}
	if input.CategoryCode <= 0 || !validName(input.Name) {
		return invalidArgument("app_create", "app category and a 1..60 weighted-character name are required")
	}
	if !sandbox && !validURL(input.DownloadURL) {
		return invalidArgument("app_create", "a valid HTTP(S) download URL is required in production")
	}
	if sandbox && (input.DownloadURL != "" || input.COPPA != nil) {
		return invalidArgument("app_create", "sandbox app creation does not accept download_url or coppa_value")
	}
	if err := validateBlockingRules("app_create", input.MaskRuleID, input.MaskRuleIDs); err != nil {
		return err
	}
	if input.COPPA != nil && !validCOPPA(*input.COPPA) {
		return invalidArgument("app_create", "COPPA value must be -1, 0, or 1")
	}
	return nil
}

func validateUpdateApp(input UpdateAppRequest, sandbox bool) error {
	if !validNumericID(string(input.AppID)) {
		return invalidArgument("app_update", "app ID is invalid")
	}
	if input.Status == nil && input.CategoryCode == nil && input.Name == nil && input.DownloadURL == nil &&
		input.MaskRuleIDs == nil && input.COPPA == nil {
		return invalidArgument("app_update", "at least one editable field is required")
	}
	if input.Status != nil {
		allowed := *input.Status == AppStatusResume || sandbox && *input.Status == AppStatusTest || !sandbox && *input.Status == AppStatusLive
		if !allowed {
			return invalidArgument("app_update", "status must be resume or match the configured environment")
		}
	}
	if input.CategoryCode != nil && *input.CategoryCode <= 0 || input.Name != nil && !validName(*input.Name) ||
		input.DownloadURL != nil && !validURL(*input.DownloadURL) {
		return invalidArgument("app_update", "one or more app fields are invalid")
	}
	if sandbox && (input.DownloadURL != nil || input.COPPA != nil) {
		return invalidArgument("app_update", "sandbox app updates do not accept download_url or coppa_value")
	}
	if input.MaskRuleIDs != nil && !validIDs(*input.MaskRuleIDs, maximumFilterValues) {
		return invalidArgument("app_update", "blocking rule IDs are invalid")
	}
	if input.COPPA != nil && !validCOPPA(*input.COPPA) {
		return invalidArgument("app_update", "COPPA value must be -1, 0, or 1")
	}
	return nil
}

func validateListApps(input ListAppsRequest) error {
	if input.Page < 1 || int64(input.Page) > 2_147_483_647 || input.PageSize < 1 || input.PageSize > 500 {
		return invalidArgument("apps_list", "page must be 1..2147483647 and page size must be 1..500")
	}
	if !validIDs(input.IDs, maximumFilterValues) || !validNames(input.Names) ||
		!validOSValues(input.OS) || !validAppStatuses(input.Statuses) {
		return invalidArgument("apps_list", "one or more app filters are invalid")
	}
	return nil
}

func validateBlockingRules(operation string, legacy *int64, values *[]ID) error {
	if legacy != nil && values != nil {
		return invalidArgument(operation, "mask_rule_id and mask_rule_ids are mutually exclusive")
	}
	if legacy != nil && !validMaskRuleID(*legacy) || values != nil && !validIDs(*values, maximumFilterValues) {
		return invalidArgument(operation, "blocking rule IDs are invalid")
	}
	return nil
}

func validateCreatePlacement(input CreatePlacementRequest) error {
	if !validNumericID(string(input.AppID)) {
		return invalidArgument("placement_create", "app ID is invalid")
	}
	if input.Name != "" && !validName(input.Name) {
		return invalidArgument("placement_create", "placement name must use 1..60 weighted characters")
	}
	if err := validateBlockingRules("placement_create", input.MaskRuleID, input.MaskRuleIDs); err != nil {
		return err
	}
	if input.Bidding != nil && !validBiddingType(*input.Bidding) {
		return invalidArgument("placement_create", "bidding type is invalid")
	}
	if err := validateCPMSettings("placement_create", input.CPM, input.Currency, input.CPMByRegion, input.Bidding); err != nil {
		return err
	}
	return validatePlacementSpec(input.Spec)
}

func validateCPMSettings(operation string, cpm Money, currency Currency, byRegion map[Region]Money, bidding *BiddingType) error {
	if cpm == "" && currency != "" && len(byRegion) == 0 || cpm != "" && currency == "" || len(byRegion) > 0 && currency == "" {
		return invalidArgument(operation, "CPM values and currency must be configured together")
	}
	if cpm != "" && !validMoney(cpm, currency) {
		return invalidArgument(operation, "CPM is outside the supported currency range")
	}
	if len(byRegion) > 0 {
		if bidding == nil || *bidding != BiddingFixedCPM || !validCPMByRegion(byRegion, currency) {
			return invalidArgument(operation, "regional CPM requires fixed bidding, a valid currency, and supported regions")
		}
	}
	if bidding != nil && *bidding != BiddingFixedCPM && cpm != "" {
		return invalidArgument(operation, "CPM is only available for fixed bidding")
	}
	return nil
}

func validatePlacementSpec(spec PlacementSpec) error {
	switch typed := spec.(type) {
	case NativeSpec:
		return validateNativeSpec(typed)
	case *NativeSpec:
		if typed == nil {
			break
		}
		return validateNativeSpec(*typed)
	case BannerSpec:
		return validateBannerSpec(typed)
	case *BannerSpec:
		if typed == nil {
			break
		}
		return validateBannerSpec(*typed)
	case AppOpenSpec:
		return validateAppOpenSpec(typed)
	case *AppOpenSpec:
		if typed == nil {
			break
		}
		return validateAppOpenSpec(*typed)
	case RewardedVideoSpec:
		return validateRewardedSpec(typed)
	case *RewardedVideoSpec:
		if typed == nil {
			break
		}
		return validateRewardedSpec(*typed)
	case InterstitialSpec:
		return validateInterstitialSpec(typed)
	case *InterstitialSpec:
		if typed == nil {
			break
		}
		return validateInterstitialSpec(*typed)
	}
	return invalidArgument("placement_create", "a supported placement specification is required")
}

func validateNativeSpec(spec NativeSpec) error {
	if !validCategories(spec.Categories) {
		return invalidArgument("placement_create", "native placements require unique supported ad categories")
	}
	return nil
}

func validateBannerSpec(spec BannerSpec) error {
	validSize := spec.Width == 600 && spec.Height == 500 || spec.Width == 640 && spec.Height == 100
	if (spec.Slide != SlideBannerDisabled && spec.Slide != SlideBannerEnabled) || !validSize {
		return invalidArgument("placement_create", "banner placements require a supported slide mode and 600x500 or 640x100 size")
	}
	return nil
}

func validateAppOpenSpec(spec AppOpenSpec) error {
	if !validOrientation(spec.Orientation) || spec.AcceptMaterial != nil && !validAcceptMaterial(*spec.AcceptMaterial) {
		return invalidArgument("placement_create", "app-open orientation or material type is invalid")
	}
	return nil
}

func validateRewardedSpec(spec RewardedVideoSpec) error {
	if !validOrientation(spec.Orientation) || !validName(spec.RewardName) || spec.RewardCount < 0 || spec.RewardCount > maximumRewardCount {
		return invalidArgument("placement_create", "rewarded-video orientation, reward name, or count is invalid")
	}
	if spec.VerifyServer && !validURL(spec.CallbackURL) || !spec.VerifyServer && spec.CallbackURL != "" && !validURL(spec.CallbackURL) {
		return invalidArgument("placement_create", "a valid callback URL is required for server-verified rewards")
	}
	return nil
}

func validateInterstitialSpec(spec InterstitialSpec) error {
	if !validOrientation(spec.Orientation) || spec.AcceptMaterial != nil && !validInterstitialMaterial(*spec.AcceptMaterial) {
		return invalidArgument("placement_create", "interstitial orientation or material type is invalid")
	}
	return nil
}

func validateUpdatePlacement(input UpdatePlacementRequest) error {
	if !validNumericID(string(input.AdSlotID)) {
		return invalidArgument("placement_update", "ad placement ID is invalid")
	}
	if input.Name == nil && input.Status == nil && input.MaskRuleIDs == nil && input.CPM == nil && input.Currency == nil &&
		input.CPMByRegion == nil && input.Categories == nil && input.SlideBanner == nil && input.Orientation == nil &&
		input.RewardName == nil && input.RewardCount == nil && input.RewardIsCallback == nil && input.RewardCallbackURL == nil &&
		input.UpdateSecurityKey == nil && input.AcceptMaterial == nil {
		return invalidArgument("placement_update", "at least one editable field is required")
	}
	if input.Name != nil && !validName(*input.Name) || input.Status != nil && !validPlacementStatus(*input.Status) ||
		input.MaskRuleIDs != nil && !validIDs(*input.MaskRuleIDs, maximumFilterValues) {
		return invalidArgument("placement_update", "name, status, or blocking rule IDs are invalid")
	}
	if err := validateUpdateCPM(input); err != nil {
		return err
	}
	if input.Categories != nil && !validCategories(*input.Categories) ||
		input.SlideBanner != nil && *input.SlideBanner != SlideBannerDisabled && *input.SlideBanner != SlideBannerEnabled ||
		input.Orientation != nil && !validOrientation(*input.Orientation) ||
		input.RewardName != nil && !validName(*input.RewardName) ||
		input.RewardCount != nil && (*input.RewardCount < 0 || *input.RewardCount > maximumRewardCount) ||
		input.RewardCallbackURL != nil && !validURL(*input.RewardCallbackURL) ||
		input.AcceptMaterial != nil && !validInterstitialMaterial(*input.AcceptMaterial) {
		return invalidArgument("placement_update", "one or more type-specific fields are invalid")
	}
	if input.RewardIsCallback != nil && !*input.RewardIsCallback && input.RewardCallbackURL != nil ||
		input.RewardIsCallback != nil && !*input.RewardIsCallback && input.UpdateSecurityKey != nil && *input.UpdateSecurityKey {
		return invalidArgument("placement_update", "disabled server verification cannot update its callback URL or security key")
	}
	return nil
}

func validateUpdateCPM(input UpdatePlacementRequest) error {
	hasCPM := input.CPM != nil
	hasRegions := input.CPMByRegion != nil
	if (hasCPM || hasRegions) && input.Currency == nil || input.Currency != nil && !hasCPM && !hasRegions {
		return invalidArgument("placement_update", "CPM values and currency must be configured together")
	}
	if hasCPM && !validMoney(*input.CPM, *input.Currency) || hasRegions && !validCPMByRegion(input.CPMByRegion, *input.Currency) {
		return invalidArgument("placement_update", "CPM values, currency, or regions are invalid")
	}
	return nil
}

func validateListPlacements(input ListPlacementsRequest) error {
	if input.Page < 1 || int64(input.Page) > 2_147_483_647 || input.PageSize < 1 || input.PageSize > 500 {
		return invalidArgument("placements_list", "page must be 1..2147483647 and page size must be 1..500")
	}
	if !validIDs(input.IDs, maximumFilterValues) || !validNames(input.Names) ||
		!validIDs(input.AppIDs, maximumFilterValues) || !validNames(input.AppNames) ||
		!validPlacementTypes(input.Types) || !validPlacementStatuses(input.Statuses) {
		return invalidArgument("placements_list", "one or more placement filters are invalid")
	}
	return nil
}

func validateUpdateExpectedCPM(input UpdateExpectedCPMRequest) error {
	if !validNumericID(string(input.AdSlotID)) || !validNumericID(string(input.AppID)) {
		return invalidArgument("expected_cpm_update", "ad placement ID or app ID is invalid")
	}
	if !validMoney(input.CPM, input.Currency) {
		return invalidArgument("expected_cpm_update", "CPM and currency are invalid")
	}
	return nil
}

func validPageInfo(info PageInfo, requestedPage, requestedSize, itemCount int) bool {
	if info.Page != requestedPage || info.PageSize != requestedSize || info.TotalNumber < itemCount || itemCount > requestedSize ||
		info.TotalNumber < 0 || info.TotalPages < 0 {
		return false
	}
	expectedPages := 0
	if info.TotalNumber > 0 {
		expectedPages = (info.TotalNumber + info.PageSize - 1) / info.PageSize
	}
	return info.TotalPages == expectedPages
}

func validAppResponse(app App, sandbox bool) bool {
	if !validNumericID(string(app.ID)) || !validNumericID(string(app.UserID)) || !validAppStatus(app.Status) ||
		app.CategoryCode <= 0 || !validResponseText(app.Name, 1_024) || !validResponseText(app.PackageName, 1_024) ||
		!validOS(app.OS) || !sandbox && !validURL(app.DownloadURL) || sandbox && app.DownloadURL != "" && !validURL(app.DownloadURL) ||
		!validResponseText(app.DownloadAddress, 1_024) ||
		!validIDs(app.MaskRuleIDs, maximumFilterValues) {
		return false
	}
	if app.APKSign != "" && !validSHA1(app.APKSign) || app.DebugAPKSign != "" && !validSHA1(app.DebugAPKSign) ||
		app.COPPA != nil && !validCOPPA(*app.COPPA) {
		return false
	}
	return true
}

func validPlacementResponse(placement Placement) bool {
	if !validNumericID(string(placement.ID)) || !validNumericID(string(placement.AppID)) ||
		!validResponseText(placement.Name, 1_024) || !validResponseText(placement.AppName, 1_024) ||
		!validPlacementStatus(placement.Status) || !validAdSlotType(placement.Type) ||
		!validIDs(placement.MaskRuleIDs, maximumFilterValues) || placement.MaskRuleID != "" && !validNumericID(string(placement.MaskRuleID)) ||
		!validBiddingType(placement.BiddingType) || placement.UseMediation < 0 || placement.UseMediation > 1 {
		return false
	}
	if placement.Type == AdSlotNative && placement.RenderType != 2 || placement.Type != AdSlotNative && placement.RenderType != 1 {
		return false
	}
	if len(placement.Categories) > 0 && !validCategories(placement.Categories) || placement.Width < 0 || placement.Height < 0 ||
		placement.Orientation != 0 && !validOrientation(placement.Orientation) ||
		placement.RewardCount < 0 || placement.RewardCount > maximumRewardCount ||
		placement.RewardIsCallback < 0 || placement.RewardIsCallback > 1 ||
		placement.RewardCallbackURL != "" && !validURL(placement.RewardCallbackURL) ||
		!validResponseText(placement.RewardSecurityKey, 4_096) ||
		placement.CPM != "" && !validNonnegativeDecimal(placement.CPM) {
		return false
	}
	if placement.AcceptMaterial != 0 && !validAcceptMaterial(placement.AcceptMaterial) || len(placement.ExpectedCPM) > 256 {
		return false
	}
	for _, expected := range placement.ExpectedCPM {
		if !validResponseRegion(expected.Country) || !validNonnegativeDecimal(expected.CPM) {
			return false
		}
	}
	return true
}

func validResponseRegion(value Region) bool {
	text := string(value)
	return len(text) == 2 && text[0] >= 'a' && text[0] <= 'z' && text[1] >= 'a' && text[1] <= 'z'
}

func validNonnegativeDecimal(value Decimal) bool {
	return validMoney(Money(value), CurrencyCNY) || validMoney(Money(value), CurrencyUSD)
}
