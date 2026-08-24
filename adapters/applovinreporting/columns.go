package applovinreporting

const (
	CampaignColumnAd               CampaignColumn = "ad"
	CampaignColumnAdCreativeType   CampaignColumn = "ad_creative_type"
	CampaignColumnApplication      CampaignColumn = "application"
	CampaignColumnCampaign         CampaignColumn = "campaign"
	CampaignColumnCampaignID       CampaignColumn = "campaign_id_external"
	CampaignColumnClicks           CampaignColumn = "clicks"
	CampaignColumnConversions      CampaignColumn = "conversions"
	CampaignColumnCost             CampaignColumn = "cost"
	CampaignColumnCountry          CampaignColumn = "country"
	CampaignColumnCreativeSet      CampaignColumn = "creative_set"
	CampaignColumnCreativeSetID    CampaignColumn = "creative_set_id"
	CampaignColumnCTR              CampaignColumn = "ctr"
	CampaignColumnDay              CampaignColumn = "day"
	CampaignColumnHour             CampaignColumn = "hour"
	CampaignColumnImpressions      CampaignColumn = "impressions"
	CampaignColumnPlatform         CampaignColumn = "platform"
	CampaignColumnPublisherRevenue CampaignColumn = "revenue"
	CampaignColumnPublisherECPM    CampaignColumn = "ecpm"
	CampaignColumnPublisherPackage CampaignColumn = "package_name"
	CampaignColumnPublisherStoreID CampaignColumn = "store_id"
	CampaignColumnPublisherZone    CampaignColumn = "zone"
	CampaignColumnPublisherZoneID  CampaignColumn = "zone_id"
)

const (
	AssetColumnID                  AssetColumn = "asset_id"
	AssetColumnName                AssetColumn = "asset_name"
	AssetColumnURL                 AssetColumn = "asset_url"
	AssetColumnCampaign            AssetColumn = "campaign"
	AssetColumnCampaignID          AssetColumn = "campaign_id"
	AssetColumnCampaignPackageName AssetColumn = "campaign_package_name"
	AssetColumnClicks              AssetColumn = "clicks"
	AssetColumnCost                AssetColumn = "cost"
	AssetColumnCreativeSet         AssetColumn = "creative_set"
	AssetColumnCreativeSetID       AssetColumn = "creative_set_id"
	AssetColumnCTR                 AssetColumn = "ctr"
	AssetColumnImpressions         AssetColumn = "impressions"
)

const (
	PlayableColumnAssetID               PlayableColumn = "asset_id"
	PlayableColumnAverageDuration       PlayableColumn = "average_duration"
	PlayableColumnCountry               PlayableColumn = "country"
	PlayableColumnDay                   PlayableColumn = "day"
	PlayableColumnDeviceModel           PlayableColumn = "device_model"
	PlayableColumnHTMLName              PlayableColumn = "html_name"
	PlayableColumnImpressions           PlayableColumn = "impressions"
	PlayableColumnOriginalURL           PlayableColumn = "original_url"
	PlayableColumnOSVersion             PlayableColumn = "os_version"
	PlayableColumnPlatform              PlayableColumn = "platform"
	PlayableColumnSpend                 PlayableColumn = "spend"
	PlayableColumnTotalInteractions     PlayableColumn = "total_interactions"
	PlayableColumnUniqueInteractions    PlayableColumn = "unique_interactions"
	PlayableColumnUniqueInteractionRate PlayableColumn = "unique_interactions_rate"
	PlayableColumnUniqueRedirects       PlayableColumn = "unique_redirects"
	PlayableColumnUniqueRedirectRate    PlayableColumn = "unique_redirects_rate"
	PlayableColumnHTMLLoading           PlayableColumn = "html_loading"
	PlayableColumnHTMLLoaded            PlayableColumn = "html_loaded"
	PlayableColumnHTMLDisplayed         PlayableColumn = "html_displayed"
	PlayableColumnHTMLCompleted         PlayableColumn = "html_completed"
	PlayableColumnHTMLCompletionRate    PlayableColumn = "html_completion_rate"
	PlayableColumnChallengeStarted      PlayableColumn = "challenge_started"
	PlayableColumnChallengeFailed       PlayableColumn = "challenge_failed"
	PlayableColumnChallengeFailedRate   PlayableColumn = "challenge_failed_rate"
	PlayableColumnChallengeRetry        PlayableColumn = "challenge_retry"
	PlayableColumnChallengeRetryRate    PlayableColumn = "challenge_retry_rate"
	PlayableColumnChallengeSolved       PlayableColumn = "challenge_solved"
	PlayableColumnChallengeSolvedRate   PlayableColumn = "challenge_solved_rate"
	PlayableColumnChallengePass25       PlayableColumn = "challenge_pass_25"
	PlayableColumnChallengePass25Rate   PlayableColumn = "challenge_pass_25_rate"
	PlayableColumnChallengePass50       PlayableColumn = "challenge_pass_50"
	PlayableColumnChallengePass50Rate   PlayableColumn = "challenge_pass_50_rate"
	PlayableColumnChallengePass75       PlayableColumn = "challenge_pass_75"
	PlayableColumnChallengePass75Rate   PlayableColumn = "challenge_pass_75_rate"
	PlayableColumnCTAClicked            PlayableColumn = "cta_clicked"
	PlayableColumnCTAClickRate          PlayableColumn = "cta_click_rate"
	PlayableColumnEndcardShown          PlayableColumn = "endcard_shown"
	PlayableColumnRedirectCount         PlayableColumn = "redirect_count"
	PlayableColumnRedirectRate          PlayableColumn = "redirect_rate"
	PlayableColumnBlackViewError        PlayableColumn = "black_view_error"
	PlayableColumnBlackViewErrorRate    PlayableColumn = "black_view_error_rate"
	PlayableColumnRenderingError        PlayableColumn = "rendering_error"
	PlayableColumnRenderingErrorRate    PlayableColumn = "rendering_error_rate"
	PlayableColumnRuntimeError          PlayableColumn = "runtime_error"
	PlayableColumnRuntimeErrorRate      PlayableColumn = "runtime_error_rate"
)

var (
	standardWindows  = []string{"0d", "1d", "2d", "3d", "7d", "14d", "28d", "30d", "90d", "1y"}
	no28Windows      = []string{"0d", "1d", "2d", "3d", "7d", "14d", "30d", "90d", "1y"}
	retentionWindows = []string{"1d", "3d", "7d", "14d", "28d"}
	webWindows       = []string{"0d", "1d", "2d", "3d", "7d", "14d", "28d"}

	appPublisherColumns = campaignColumns(
		"ad_type", "application", "application_is_hidden", "bidding_integration", "clicks", "country", "ctr", "day",
		"device_type", "ecpm", "hour", "impressions", "package_name", "placement_type", "platform", "revenue", "size",
		"store_id", "zone", "zone_id",
	)
	appAdvertiserColumns = expandedCampaignColumns(
		campaignColumns(
			"ad", "ad_creative_type", "ad_type", "app_id_external", "application", "average_cpa", "average_cpc",
			"bidding_and_billing_method", "campaign", "campaign_ad_type", "campaign_bid_goal", "campaign_id_external",
			"campaign_package_name", "campaign_roas_goal", "campaign_store_id", "campaign_type", "clicks", "conversion_rate",
			"conversions", "cost", "country", "creative_set", "creative_set_id", "ctr", "custom_page_id", "day", "device_type",
			"external_placement_id", "first_purchase", "hour", "impressions", "optimization_day_target", "placement_type", "platform",
			"sales", "size", "target_event", "target_event_count", "traffic_source",
		),
		map[string][]string{
			"ad_rev": standardWindows, "ad_roas": standardWindows, "cost_per_target_event": no28Windows,
			"cpp": no28Windows, "iap_rev": standardWindows, "iap_roas": standardWindows, "ret": retentionWindows,
			"roas": standardWindows, "sales": no28Windows, "target_event_count": no28Windows,
			"total_rev": standardWindows, "unique_purchasers": standardWindows,
		},
		[]string{"ad_rev", "ad_roas", "cost_per_target_event", "cpp", "iap_rev", "iap_roas", "ret", "roas", "sales", "target_event_count", "total_rev", "unique_purchasers"},
	)
	webAdvertiserColumns = expandedCampaignColumns(
		campaignColumns(
			"audience_strategy", "average_cpc", "campaign", "campaign_bid_goal", "campaign_id_external", "campaign_type", "clicks",
			"cost", "country", "creative_set", "creative_set_id", "ctr", "custom_page_id", "day", "hour", "impressions",
			"nc_d0_checkout_rev", "nc_d0_checkouts", "nc_d0_cpp", "nc_d0_roas", "nc_percent_d0_checkout_rev",
			"nc_percent_d0_checkouts", "nc_d7_checkout_rev", "nc_d7_checkouts", "nc_d7_cpp", "nc_d7_roas",
			"nc_percent_d7_checkout_rev", "nc_percent_d7_checkouts", "new_visitor_rate", "optimization_day_target", "placement_type",
			"platform", "sales", "target_event", "cost_per_target_event_0d", "target_event_count_0d",
		),
		map[string][]string{
			"chka": webWindows, "chka_usd": webWindows, "cost_per_chka": webWindows,
			"cpp": webWindows, "roas": webWindows, "sales": webWindows,
		},
		[]string{"chka", "chka_usd", "cost_per_chka", "cpp", "roas", "sales"},
	)
	assetWebColumns = assetColumns(
		"asset_id", "asset_name", "asset_url", "campaign", "campaign_id", "clicks", "cost", "creative_set", "creative_set_id", "ctr", "impressions",
	)
	assetAppColumns = appendAssetColumn(assetWebColumns, AssetColumnCampaignPackageName)
	playableColumns = playableColumnValues(
		"asset_id", "average_duration", "country", "day", "device_model", "html_name", "impressions", "original_url", "os_version", "platform", "spend",
		"total_interactions", "unique_interactions", "unique_interactions_rate", "unique_redirects", "unique_redirects_rate",
		"html_loading", "html_loaded", "html_displayed", "html_completed", "html_completion_rate",
		"challenge_started", "challenge_failed", "challenge_failed_rate", "challenge_retry", "challenge_retry_rate", "challenge_solved", "challenge_solved_rate",
		"challenge_pass_25", "challenge_pass_25_rate", "challenge_pass_50", "challenge_pass_50_rate", "challenge_pass_75", "challenge_pass_75_rate",
		"cta_clicked", "cta_click_rate", "endcard_shown", "redirect_count", "redirect_rate", "black_view_error", "black_view_error_rate",
		"rendering_error", "rendering_error_rate", "runtime_error", "runtime_error_rate",
	)

	appAdvertiserColumnSet = toCampaignColumnSet(appAdvertiserColumns)
	appPublisherColumnSet  = toCampaignColumnSet(appPublisherColumns)
	webAdvertiserColumnSet = toCampaignColumnSet(webAdvertiserColumns)
	assetAppColumnSet      = toAssetColumnSet(assetAppColumns)
	assetWebColumnSet      = toAssetColumnSet(assetWebColumns)
	playableColumnSet      = toPlayableColumnSet(playableColumns)
	assetFilterColumnSet   = toAssetColumnSet(assetColumns("asset_id", "asset_name", "campaign", "campaign_id", "campaign_package_name", "creative_set", "creative_set_id"))
	assetMetricColumnSet   = toAssetColumnSet(assetColumns("clicks", "cost", "ctr", "impressions"))
)

func campaignColumns(values ...string) []CampaignColumn {
	result := make([]CampaignColumn, len(values))
	for index, value := range values {
		result[index] = CampaignColumn(value)
	}
	return result
}

func assetColumns(values ...string) []AssetColumn {
	result := make([]AssetColumn, len(values))
	for index, value := range values {
		result[index] = AssetColumn(value)
	}
	return result
}

func playableColumnValues(values ...string) []PlayableColumn {
	result := make([]PlayableColumn, len(values))
	for index, value := range values {
		result[index] = PlayableColumn(value)
	}
	return result
}

func expandedCampaignColumns(base []CampaignColumn, groups map[string][]string, order []string) []CampaignColumn {
	result := append([]CampaignColumn(nil), base...)
	for _, stem := range order {
		for _, suffix := range groups[stem] {
			result = append(result, CampaignColumn(stem+"_"+suffix))
		}
	}
	return result
}

func appendAssetColumn(values []AssetColumn, value AssetColumn) []AssetColumn {
	result := append([]AssetColumn(nil), values...)
	return append(result, value)
}

func toCampaignColumnSet(values []CampaignColumn) map[CampaignColumn]struct{} {
	result := make(map[CampaignColumn]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func toAssetColumnSet(values []AssetColumn) map[AssetColumn]struct{} {
	result := make(map[AssetColumn]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func toPlayableColumnSet(values []PlayableColumn) map[PlayableColumn]struct{} {
	result := make(map[PlayableColumn]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// CampaignColumns returns the current typed column contract for an account and
// report type. WEB accounts have no publisher report surface.
func CampaignColumns(accountType AccountType, reportType ReportType) []CampaignColumn {
	var source []CampaignColumn
	switch {
	case accountType == AccountTypeApp && reportType == ReportAdvertiser:
		source = appAdvertiserColumns
	case accountType == AccountTypeApp && reportType == ReportPublisher:
		source = appPublisherColumns
	case accountType == AccountTypeWeb && reportType == ReportAdvertiser:
		source = webAdvertiserColumns
	}
	return append([]CampaignColumn(nil), source...)
}

func AssetColumns(accountType AccountType) []AssetColumn {
	if accountType == AccountTypeApp {
		return append([]AssetColumn(nil), assetAppColumns...)
	}
	if accountType == AccountTypeWeb {
		return append([]AssetColumn(nil), assetWebColumns...)
	}
	return nil
}

func PlayableColumns() []PlayableColumn {
	return append([]PlayableColumn(nil), playableColumns...)
}

func campaignColumnSet(accountType AccountType, reportType ReportType) map[CampaignColumn]struct{} {
	switch {
	case accountType == AccountTypeApp && reportType == ReportAdvertiser:
		return appAdvertiserColumnSet
	case accountType == AccountTypeApp && reportType == ReportPublisher:
		return appPublisherColumnSet
	case accountType == AccountTypeWeb && reportType == ReportAdvertiser:
		return webAdvertiserColumnSet
	default:
		return nil
	}
}

func assetColumnSet(accountType AccountType) map[AssetColumn]struct{} {
	if accountType == AccountTypeApp {
		return assetAppColumnSet
	}
	if accountType == AccountTypeWeb {
		return assetWebColumnSet
	}
	return nil
}
