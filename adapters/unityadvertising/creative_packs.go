package unityadvertising

import (
	"context"
	"encoding/json"
	"net/url"

	"social-hub/pkg/socialhub"
)

type CreativePack struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	CreativeIDs         []string         `json:"creativeIds"`
	Type                CreativePackType `json:"type"`
	CampaignIDs         []string         `json:"campaignIds"`
	AndroidStoreListing *string          `json:"androidStoreListing"`
	AppleProductPageID  *string          `json:"appleProductPageId"`
	Raw                 json.RawMessage  `json:"-"`
}

type ListCreativePacksRequest struct {
	Offset int64
	Limit  int
	Name   string
}

type CreateCreativePackRequest struct {
	Name                string           `json:"name"`
	CreativeIDs         []string         `json:"creativeIds"`
	Type                CreativePackType `json:"type"`
	AndroidStoreListing *NullableString  `json:"androidStoreListing,omitempty"`
	AppleProductPageID  *NullableString  `json:"appleProductPageId,omitempty"`
}

type UpdateCreativePackRequest struct {
	Name                *string         `json:"name,omitempty"`
	AndroidStoreListing *NullableString `json:"androidStoreListing,omitempty"`
	AppleProductPageID  *NullableString `json:"appleProductPageId,omitempty"`
}

type CreativePacksWorkflow interface {
	ListCreativePacks(context.Context, string, ListCreativePacksRequest, ...socialhub.CallOption) (Page[CreativePack], error)
	CreateCreativePack(context.Context, string, CreateCreativePackRequest, ...socialhub.CallOption) (*CreativePack, error)
	GetCreativePack(context.Context, string, string, ...socialhub.CallOption) (*CreativePack, error)
	UpdateCreativePack(context.Context, string, string, UpdateCreativePackRequest, ...socialhub.CallOption) (*CreativePack, error)
	DeleteCreativePack(context.Context, string, string, ...socialhub.CallOption) error
}

func (client *Client) ListCreativePacks(ctx context.Context, campaignSetID string, input ListCreativePacksRequest, options ...socialhub.CallOption) (Page[CreativePack], error) {
	appPath, err := client.appPath("creative_pack_list", campaignSetID)
	if err != nil {
		return Page[CreativePack]{}, err
	}
	if input.Offset < 0 || input.Limit < 0 || input.Limit > 700 || input.Name != "" && !validText(input.Name, 255) {
		return Page[CreativePack]{}, invalidArgument("creative_pack_list", "offset, limit, or name filter is invalid")
	}
	query := make(url.Values)
	if input.Offset > 0 {
		query.Set("offset", formatInt64(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", formatInt(input.Limit))
	}
	if input.Name != "" {
		query.Set("filter[name]", input.Name)
	}
	var page Page[CreativePack]
	if err := client.getJSON(ctx, "creative_pack_list", appPath+"/creative-packs", query, &page, options...); err != nil {
		return Page[CreativePack]{}, err
	}
	if !validPage(page, 700, validCreativePack) {
		return Page[CreativePack]{}, platformContractError("creative_pack_list", "Unity returned an invalid creative pack page")
	}
	return page, nil
}

func (client *Client) CreateCreativePack(ctx context.Context, campaignSetID string, input CreateCreativePackRequest, options ...socialhub.CallOption) (*CreativePack, error) {
	appPath, err := client.appPath("creative_pack_create", campaignSetID)
	if err != nil {
		return nil, err
	}
	if err := validateCreateCreativePack(input); err != nil {
		return nil, err
	}
	var pack CreativePack
	if err := client.postJSON(ctx, "creative_pack_create", appPath+"/creative-packs", input, &pack, options...); err != nil {
		return nil, err
	}
	if !validCreativePack(pack) {
		return nil, platformContractError("creative_pack_create", "Unity returned an invalid creative pack")
	}
	return &pack, nil
}

func (client *Client) GetCreativePack(ctx context.Context, campaignSetID, creativePackID string, options ...socialhub.CallOption) (*CreativePack, error) {
	path, err := client.creativePackPath("creative_pack_get", campaignSetID, creativePackID)
	if err != nil {
		return nil, err
	}
	var pack CreativePack
	if err := client.getJSON(ctx, "creative_pack_get", path, nil, &pack, options...); err != nil {
		return nil, err
	}
	if !validCreativePack(pack) || pack.ID != creativePackID {
		return nil, platformContractError("creative_pack_get", "Unity returned a creative pack that does not match the requested ID")
	}
	return &pack, nil
}

func (client *Client) UpdateCreativePack(ctx context.Context, campaignSetID, creativePackID string, input UpdateCreativePackRequest, options ...socialhub.CallOption) (*CreativePack, error) {
	path, err := client.creativePackPath("creative_pack_update", campaignSetID, creativePackID)
	if err != nil {
		return nil, err
	}
	if err := validateUpdateCreativePack(input); err != nil {
		return nil, err
	}
	var pack CreativePack
	if err := client.patchJSON(ctx, "creative_pack_update", path, input, &pack, options...); err != nil {
		return nil, err
	}
	if !validCreativePack(pack) || pack.ID != creativePackID {
		return nil, platformContractError("creative_pack_update", "Unity returned a creative pack that does not match the requested ID")
	}
	return &pack, nil
}

func (client *Client) DeleteCreativePack(ctx context.Context, campaignSetID, creativePackID string, options ...socialhub.CallOption) error {
	path, err := client.creativePackPath("creative_pack_delete", campaignSetID, creativePackID)
	if err != nil {
		return err
	}
	return client.deleteJSON(ctx, "creative_pack_delete", path, options...)
}

func validateCreateCreativePack(input CreateCreativePackRequest) error {
	if !validText(input.Name, 255) || len(input.CreativeIDs) < 1 || len(input.CreativeIDs) > 3 || !validCreativePackType(input.Type) {
		return invalidArgument("creative_pack_create", "name, one to three creative IDs, and a documented type are required")
	}
	if !validUniqueMongoIDs(input.CreativeIDs) || !validStoreListing(input.AndroidStoreListing) || !validProductPageID(input.AppleProductPageID) {
		return invalidArgument("creative_pack_create", "creative ID or store listing is invalid")
	}
	return nil
}

func validateUpdateCreativePack(input UpdateCreativePackRequest) error {
	if input.Name == nil && input.AndroidStoreListing == nil && input.AppleProductPageID == nil {
		return invalidArgument("creative_pack_update", "at least one creative pack field is required")
	}
	if input.Name != nil && !validText(*input.Name, 255) || !validStoreListing(input.AndroidStoreListing) || !validProductPageID(input.AppleProductPageID) {
		return invalidArgument("creative_pack_update", "creative pack name or store listing is invalid")
	}
	return nil
}

func validCreativePack(pack CreativePack) bool {
	return validMongoID(pack.ID) && validText(pack.Name, 255) && len(pack.CreativeIDs) >= 1 && len(pack.CreativeIDs) <= 3 &&
		validUniqueMongoIDs(pack.CreativeIDs) && validCreativePackType(pack.Type)
}

func validCreativePackType(value CreativePackType) bool {
	return value == CreativePackVideo || value == CreativePackPlayable || value == CreativePackVideoPlayable
}

func validUniqueMongoIDs(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validMongoID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validStoreListing(value *NullableString) bool {
	return value == nil || value.Value == nil || androidStoreListingRegexp.MatchString(*value.Value)
}

func validProductPageID(value *NullableString) bool {
	return value == nil || value.Value == nil || uuidPattern.MatchString(*value.Value)
}

func (pack *CreativePack) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*creativePackAlias)(pack), &pack.Raw)
}

type creativePackAlias CreativePack
