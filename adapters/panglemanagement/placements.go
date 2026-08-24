package panglemanagement

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	createPlacementPath = "/union/media/open_api/code/create"
	updatePlacementPath = "/union/media/open_api/code/update"
	queryPlacementPath  = "/union/media/open_api/code/query"
	expectedCPMPath     = "/union_pangle/open/api/code/cpm"
)

type createPlacementWire struct {
	authWire
	AppID              ID                  `json:"app_id"`
	Type               AdSlotType          `json:"ad_slot_type"`
	Name               string              `json:"ad_slot_name,omitempty"`
	MaskRuleID         *int64              `json:"mask_rule_id,omitempty"`
	MaskRuleIDs        *[]ID               `json:"mask_rule_ids,omitempty"`
	Bidding            *BiddingType        `json:"bidding_type,omitempty"`
	CPM                Money               `json:"cpm,omitempty"`
	Currency           Currency            `json:"currency,omitempty"`
	CPMByRegion        map[Region]Money    `json:"cpm_for_region,omitempty"`
	RenderType         int                 `json:"render_type"`
	Categories         []AdCategory        `json:"ad_categories,omitempty"`
	SlideBanner        *SlideBanner        `json:"slide_banner,omitempty"`
	Width              int                 `json:"width,omitempty"`
	Height             int                 `json:"height,omitempty"`
	Orientation        *Orientation        `json:"orientation,omitempty"`
	RewardName         string              `json:"reward_name,omitempty"`
	RewardCount        *int64              `json:"reward_count,omitempty"`
	RewardIsCallback   *int                `json:"reward_is_callback,omitempty"`
	RewardCallbackURL  string              `json:"reward_callback_url,omitempty"`
	AcceptMaterialType *AcceptMaterialType `json:"accept_material_type,omitempty"`
}

type updatePlacementWire struct {
	authWire
	AdSlotID           ID                  `json:"ad_slot_id"`
	Name               *string             `json:"ad_slot_name,omitempty"`
	Status             *PlacementStatus    `json:"status,omitempty"`
	MaskRuleIDs        *[]ID               `json:"mask_rule_ids,omitempty"`
	CPM                *Money              `json:"cpm,omitempty"`
	Currency           *Currency           `json:"currency,omitempty"`
	CPMByRegion        map[Region]Money    `json:"cpm_for_region,omitempty"`
	Categories         *[]AdCategory       `json:"ad_categories,omitempty"`
	SlideBanner        *SlideBanner        `json:"slide_banner,omitempty"`
	Orientation        *Orientation        `json:"orientation,omitempty"`
	RewardName         *string             `json:"reward_name,omitempty"`
	RewardCount        *int64              `json:"reward_count,omitempty"`
	RewardIsCallback   *int                `json:"reward_is_callback,omitempty"`
	RewardCallbackURL  *string             `json:"reward_callback_url,omitempty"`
	UpdateSecurityKey  *int                `json:"update_security_key,omitempty"`
	AcceptMaterialType *AcceptMaterialType `json:"accept_material_type,omitempty"`
}

type listPlacementsWire struct {
	authWire
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	IDs      []ID              `json:"ad_slot_id,omitempty"`
	Names    []string          `json:"ad_slot_name,omitempty"`
	AppIDs   []ID              `json:"app_id,omitempty"`
	AppNames []string          `json:"app_name,omitempty"`
	Types    []AdSlotType      `json:"ad_slot_type,omitempty"`
	Statuses []PlacementStatus `json:"status,omitempty"`
}

type updateExpectedCPMWire struct {
	authWire
	AdSlotID ID       `json:"ad_slot_id"`
	AppID    ID       `json:"site_id"`
	CPM      Money    `json:"cpm"`
	Delete   bool     `json:"delete_cpm"`
	Currency Currency `json:"currency"`
}

type placementMutationData struct {
	AdSlotID          ID              `json:"ad_slot_id"`
	Status            PlacementStatus `json:"status"`
	RewardSecurityKey string          `json:"reward_security_key"`
}

type listPlacementsData struct {
	PageInfo   PageInfo    `json:"page_info"`
	Placements []Placement `json:"ad_slot_list"`
}

func (client *Client) CreatePlacement(ctx context.Context, input CreatePlacementRequest, options ...socialhub.CallOption) (PlacementMutationResult, error) {
	const operation = "placement_create"
	if err := validateCreatePlacement(input); err != nil {
		return PlacementMutationResult{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return PlacementMutationResult{}, err
	}
	wire := createPlacementWire{
		authWire: auth, AppID: input.AppID, Name: input.Name,
		MaskRuleID: input.MaskRuleID, MaskRuleIDs: input.MaskRuleIDs, Bidding: input.Bidding,
		CPM: input.CPM, Currency: input.Currency, CPMByRegion: input.CPMByRegion,
	}
	applyCreatePlacementSpec(&wire, input.Spec)
	envelope, status, header, err := client.doJSON(ctx, operation, createPlacementPath, wire, auth.Sign, true, options...)
	if err != nil {
		return PlacementMutationResult{}, err
	}
	if err := requireZeroCode(operation, envelope, status, header, client, true); err != nil {
		return PlacementMutationResult{}, err
	}
	var data placementMutationData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return PlacementMutationResult{}, outcomeUnknownError(operation, err)
	}
	if !validNumericID(string(data.AdSlotID)) || !validPlacementStatus(data.Status) ||
		!validResponseText(data.RewardSecurityKey, 4_096) {
		failure := platformContractError(operation, "Pangle returned invalid placement creation data", status)
		return PlacementMutationResult{}, outcomeUnknownError(operation, failure)
	}
	return PlacementMutationResult{
		AdSlotID: data.AdSlotID, Status: data.Status,
		RewardSecurityKey: data.RewardSecurityKey, RequestID: envelope.RequestID,
	}, nil
}

func applyCreatePlacementSpec(wire *createPlacementWire, spec PlacementSpec) {
	switch typed := spec.(type) {
	case NativeSpec:
		applyNativeSpec(wire, typed)
	case *NativeSpec:
		applyNativeSpec(wire, *typed)
	case BannerSpec:
		applyBannerSpec(wire, typed)
	case *BannerSpec:
		applyBannerSpec(wire, *typed)
	case AppOpenSpec:
		applyAppOpenSpec(wire, typed)
	case *AppOpenSpec:
		applyAppOpenSpec(wire, *typed)
	case RewardedVideoSpec:
		applyRewardedSpec(wire, typed)
	case *RewardedVideoSpec:
		applyRewardedSpec(wire, *typed)
	case InterstitialSpec:
		applyInterstitialSpec(wire, typed)
	case *InterstitialSpec:
		applyInterstitialSpec(wire, *typed)
	}
}

func applyNativeSpec(wire *createPlacementWire, spec NativeSpec) {
	wire.Type, wire.RenderType, wire.Categories = AdSlotNative, 2, spec.Categories
}

func applyBannerSpec(wire *createPlacementWire, spec BannerSpec) {
	wire.Type, wire.RenderType = AdSlotBanner, 1
	wire.SlideBanner, wire.Width, wire.Height = &spec.Slide, spec.Width, spec.Height
}

func applyAppOpenSpec(wire *createPlacementWire, spec AppOpenSpec) {
	wire.Type, wire.RenderType = AdSlotAppOpen, 1
	wire.Orientation, wire.AcceptMaterialType = &spec.Orientation, spec.AcceptMaterial
}

func applyRewardedSpec(wire *createPlacementWire, spec RewardedVideoSpec) {
	wire.Type, wire.RenderType = AdSlotRewardedVideo, 1
	wire.Orientation, wire.RewardName, wire.RewardCount = &spec.Orientation, spec.RewardName, &spec.RewardCount
	callback := 0
	if spec.VerifyServer {
		callback = 1
	}
	wire.RewardIsCallback, wire.RewardCallbackURL = &callback, spec.CallbackURL
}

func applyInterstitialSpec(wire *createPlacementWire, spec InterstitialSpec) {
	wire.Type, wire.RenderType = AdSlotInterstitial, 1
	wire.Orientation, wire.AcceptMaterialType = &spec.Orientation, spec.AcceptMaterial
}

func (client *Client) UpdatePlacement(ctx context.Context, input UpdatePlacementRequest, options ...socialhub.CallOption) (PlacementMutationResult, error) {
	const operation = "placement_update"
	if err := validateUpdatePlacement(input); err != nil {
		return PlacementMutationResult{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return PlacementMutationResult{}, err
	}
	wire := updatePlacementWire{
		authWire: auth, AdSlotID: input.AdSlotID, Name: input.Name, Status: input.Status,
		MaskRuleIDs: input.MaskRuleIDs, CPM: input.CPM, Currency: input.Currency, CPMByRegion: input.CPMByRegion,
		Categories: input.Categories, SlideBanner: input.SlideBanner, Orientation: input.Orientation,
		RewardName: input.RewardName, RewardCount: input.RewardCount,
		RewardCallbackURL: input.RewardCallbackURL, AcceptMaterialType: input.AcceptMaterial,
	}
	wire.RewardIsCallback = boolInt(input.RewardIsCallback)
	wire.UpdateSecurityKey = boolInt(input.UpdateSecurityKey)
	envelope, status, header, err := client.doJSON(ctx, operation, updatePlacementPath, wire, auth.Sign, true, options...)
	if err != nil {
		return PlacementMutationResult{}, err
	}
	if err := requireZeroCode(operation, envelope, status, header, client, true); err != nil {
		return PlacementMutationResult{}, err
	}
	var data placementMutationData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return PlacementMutationResult{}, outcomeUnknownError(operation, err)
	}
	if data.AdSlotID != input.AdSlotID || !validPlacementStatus(data.Status) ||
		!validResponseText(data.RewardSecurityKey, 4_096) {
		failure := platformContractError(operation, "Pangle returned invalid placement update data", status)
		return PlacementMutationResult{}, outcomeUnknownError(operation, failure)
	}
	return PlacementMutationResult{
		AdSlotID: data.AdSlotID, Status: data.Status,
		RewardSecurityKey: data.RewardSecurityKey, RequestID: envelope.RequestID,
	}, nil
}

func boolInt(value *bool) *int {
	if value == nil {
		return nil
	}
	encoded := 0
	if *value {
		encoded = 1
	}
	return &encoded
}

func (client *Client) ListPlacements(ctx context.Context, input ListPlacementsRequest, options ...socialhub.CallOption) (PlacementPage, error) {
	const operation = "placements_list"
	if err := validateListPlacements(input); err != nil {
		return PlacementPage{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return PlacementPage{}, err
	}
	wire := listPlacementsWire{
		authWire: auth, Page: input.Page, PageSize: input.PageSize,
		IDs: input.IDs, Names: input.Names, AppIDs: input.AppIDs, AppNames: input.AppNames,
		Types: input.Types, Statuses: input.Statuses,
	}
	envelope, status, header, err := client.doJSON(ctx, operation, queryPlacementPath, wire, auth.Sign, false, options...)
	if err != nil {
		return PlacementPage{}, err
	}
	if err := requireZeroCode(operation, envelope, status, header, client, false); err != nil {
		return PlacementPage{}, err
	}
	var data listPlacementsData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return PlacementPage{}, err
	}
	if !validPageInfo(data.PageInfo, input.Page, input.PageSize, len(data.Placements)) {
		return PlacementPage{}, platformContractError(operation, "Pangle returned invalid placement pagination data", status)
	}
	for _, placement := range data.Placements {
		if !validPlacementResponse(placement) {
			return PlacementPage{}, platformContractError(operation, "Pangle returned invalid placement data", status)
		}
	}
	return PlacementPage{
		Placements: data.Placements, PageInfo: data.PageInfo,
		HasMore: data.PageInfo.Page < data.PageInfo.TotalPages,
	}, nil
}

func (client *Client) UpdateExpectedCPM(ctx context.Context, input UpdateExpectedCPMRequest, options ...socialhub.CallOption) (MutationReceipt, error) {
	const operation = "expected_cpm_update"
	if err := validateUpdateExpectedCPM(input); err != nil {
		return MutationReceipt{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return MutationReceipt{}, err
	}
	wire := updateExpectedCPMWire{
		authWire: auth, AdSlotID: input.AdSlotID, AppID: input.AppID,
		CPM: input.CPM, Delete: input.Delete, Currency: input.Currency,
	}
	envelope, status, header, err := client.doJSON(ctx, operation, expectedCPMPath, wire, auth.Sign, true, options...)
	if err != nil {
		return MutationReceipt{}, err
	}
	code := scalarCode(envelope.Code)
	if code != "PG0000" && code != "0" {
		if code == "" {
			failure := platformContractError(operation, "Pangle response omitted a valid business code", status)
			return MutationReceipt{}, outcomeUnknownError(operation, failure)
		}
		return MutationReceipt{}, businessError(operation, status, header, code, envelope.RequestID, client.clock.Now())
	}
	return MutationReceipt{RequestID: envelope.RequestID}, nil
}

func requireZeroCode(operation string, envelope apiEnvelope, status int, header http.Header, client *Client, mutation bool) error {
	code := scalarCode(envelope.Code)
	if code == "0" {
		return nil
	}
	if code == "" {
		failure := platformContractError(operation, "Pangle response omitted a valid business code", status)
		if mutation {
			return outcomeUnknownError(operation, failure)
		}
		return failure
	}
	return businessError(operation, status, header, code, envelope.RequestID, client.clock.Now())
}
