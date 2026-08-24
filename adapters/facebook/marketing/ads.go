package marketing

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const adFields = "id,account_id,campaign_id,adset_id,name,status,configured_status,effective_status,creative{id,name},created_time,updated_time"

func (client *Client) GetAd(ctx context.Context, adID string, options ...socialhub.CallOption) (*Ad, error) {
	if !validNumericID(adID) {
		return nil, invalidArgument("ad_get", "ad ID must be numeric")
	}
	if err := client.requireRead("ad_get"); err != nil {
		return nil, err
	}
	var response Ad
	query := url.Values{"fields": {adFields}}
	if err := client.api.JSON(ctx, http.MethodGet, "/"+url.PathEscape(adID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("ad_get", adID, response.ID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListAds(ctx context.Context, input ListAdsRequest, options ...socialhub.CallOption) (socialhub.Page[Ad], error) {
	limit, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Ad]{}, err
	}
	if input.AdSetID != "" && !validNumericID(input.AdSetID) || !validateStatuses(input.EffectiveStatuses) {
		return socialhub.Page[Ad]{}, invalidArgument("ad_list", "ad set ID or effective status is invalid")
	}
	if err := client.requireRead("ad_list"); err != nil {
		return socialhub.Page[Ad]{}, err
	}
	query := url.Values{"fields": {adFields}}
	addPaging(query, input.Cursor, limit)
	if len(input.EffectiveStatuses) > 0 {
		if err := setJSONForm(query, "effective_status", input.EffectiveStatuses, "ad_list"); err != nil {
			return socialhub.Page[Ad]{}, err
		}
	}
	parent := client.adAccountResource()
	if input.AdSetID != "" {
		parent = url.PathEscape(input.AdSetID)
	}
	var response graphPage[Ad]
	if err := client.api.JSON(ctx, http.MethodGet, "/"+parent+"/ads", query, nil, &response, options...); err != nil {
		return socialhub.Page[Ad]{}, err
	}
	for _, item := range response.Data {
		if err := requireResponseID("ad_list", "", item.ID); err != nil {
			return socialhub.Page[Ad]{}, err
		}
	}
	return toPage(response), nil
}

func (client *Client) CreateAd(ctx context.Context, input CreateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	if !validRequiredText(input.Name, 512) || !validNumericID(input.AdSetID) || !validNumericID(input.CreativeID) {
		return nil, invalidArgument("ad_create", "name, ad set ID, and creative ID are required")
	}
	if err := client.requireManagement("ad_create"); err != nil {
		return nil, err
	}
	form := url.Values{"name": {input.Name}, "adset_id": {input.AdSetID}, "status": {string(StatusPaused)}}
	if err := setJSONForm(form, "creative", map[string]string{"creative_id": input.CreativeID}, "ad_create"); err != nil {
		return nil, err
	}
	var response idResponse
	if err := client.postForm(ctx, "/"+client.adAccountResource()+"/ads", form, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("ad_create", "", response.ID); err != nil {
		return nil, err
	}
	return &Ad{
		ID: response.ID, AccountID: client.adAccountID, AdSetID: input.AdSetID, Name: input.Name,
		Status: StatusPaused, Creative: &CreativeRef{ID: input.CreativeID},
	}, nil
}

func (client *Client) UpdateAd(ctx context.Context, adID string, input UpdateAdRequest, options ...socialhub.CallOption) error {
	if !validNumericID(adID) {
		return invalidArgument("ad_update", "ad ID must be numeric")
	}
	if input.Name == nil && input.Status == nil && input.CreativeID == nil {
		return invalidArgument("ad_update", "at least one patch field is required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.Status != nil && !validMutationStatus(*input.Status) ||
		input.CreativeID != nil && !validNumericID(*input.CreativeID) {
		return invalidArgument("ad_update", "one or more patch fields are invalid")
	}
	if err := client.requireManagement("ad_update"); err != nil {
		return err
	}
	form := url.Values{}
	if input.Name != nil {
		form.Set("name", *input.Name)
	}
	if input.Status != nil {
		form.Set("status", string(*input.Status))
	}
	if input.CreativeID != nil {
		if err := setJSONForm(form, "creative", map[string]string{"creative_id": *input.CreativeID}, "ad_update"); err != nil {
			return err
		}
	}
	var response successResponse
	if err := client.postForm(ctx, "/"+url.PathEscape(adID), form, &response, options...); err != nil {
		return err
	}
	return requireMutationSuccess("ad_update", response)
}
