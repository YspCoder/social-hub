package snapchat

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

// PublicProfileService implements the typed Snapchat read workflow.
type PublicProfileService struct {
	client *Client
}

func (s *PublicProfileService) Profile(ctx context.Context, profileID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if strings.TrimSpace(profileID) == "" {
		return nil, invalidArgument("profile", "profile ID is required")
	}
	if err := s.client.requireScope("profile"); err != nil {
		return nil, err
	}
	var response profileEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/v1/public_profiles/"+url.PathEscape(profileID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkResponse("profile", response.responseMeta, profileStates(response.PublicProfiles)); err != nil {
		return nil, err
	}
	for _, item := range response.PublicProfiles {
		if item.PublicProfile.ID == profileID {
			return mapProfile(s.client.accountID, item.PublicProfile), nil
		}
	}
	return nil, platformError("profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
}

func (s *PublicProfileService) MyProfile(ctx context.Context, options ...socialhub.CallOption) (*socialhub.User, error) {
	if err := s.client.requireScope("my_profile"); err != nil {
		return nil, err
	}
	var response profileEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/v1/public_profiles/my_profile", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkResponse("my_profile", response.responseMeta, nil); err != nil {
		return nil, err
	}
	if response.PublicProfile == nil || response.PublicProfile.ID == "" {
		return nil, platformError("my_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapProfile(s.client.accountID, *response.PublicProfile), nil
}

func (s *PublicProfileService) SearchProfiles(ctx context.Context, input ProfileSearchRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.User], error) {
	if strings.TrimSpace(input.Query) == "" {
		return socialhub.Page[socialhub.User]{}, invalidArgument("search_profiles", "query is required")
	}
	if input.Limit < 0 || input.Limit > 100 {
		return socialhub.Page[socialhub.User]{}, invalidArgument("search_profiles", "limit must be between 1 and 100 when set")
	}
	if input.Category != "" && input.Category != SearchCategoryPerson && input.Category != SearchCategoryBusiness {
		return socialhub.Page[socialhub.User]{}, invalidArgument("search_profiles", "invalid category")
	}
	if input.Tier != "" && input.Tier != SearchTierPublic && input.Tier != SearchTierPublicOfficial {
		return socialhub.Page[socialhub.User]{}, invalidArgument("search_profiles", "invalid tier")
	}
	if err := s.client.requireScope("search_profiles"); err != nil {
		return socialhub.Page[socialhub.User]{}, err
	}
	query := url.Values{"query": {input.Query}}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Category != "" {
		query.Set("category", string(input.Category))
	}
	if input.Tier != "" {
		query.Set("tier", string(input.Tier))
	}
	if input.IncludeStandard {
		query.Set("includeStandard", "true")
	}
	var response profileEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/public/v1/public_profiles/search", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.User]{}, err
	}
	if err := checkResponse("search_profiles", response.responseMeta, profileStates(response.PublicProfiles)); err != nil {
		return socialhub.Page[socialhub.User]{}, err
	}
	items := make([]socialhub.User, 0, len(response.PublicProfiles))
	for _, item := range response.PublicProfiles {
		items = append(items, *mapProfile(s.client.accountID, item.PublicProfile))
	}
	return socialhub.Page[socialhub.User]{Items: items}, nil
}

func (s *PublicProfileService) ListSpotlights(ctx context.Context, input SpotlightListRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	profileID := strings.TrimSpace(input.ProfileID)
	if profileID == "" {
		profileID = s.client.profileID
	}
	if input.Limit < 0 {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_spotlights", "limit must be positive when set")
	}
	if err := s.client.requireScope("list_spotlights"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var response spotlightEnvelope
	path := "/v1/public_profiles/" + url.PathEscape(profileID) + "/spotlights"
	if err := s.client.transport.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := checkResponse("list_spotlights", response.responseMeta, spotlightStates(response.Spotlights)); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Spotlights))
	for _, item := range response.Spotlights {
		items = append(items, *mapSpotlight(s.client.accountID, item.Spotlight))
	}
	return pageOfSpotlights(items, response.Paging), nil
}

func (s *PublicProfileService) Spotlight(ctx context.Context, spotlightID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(spotlightID) == "" {
		return nil, invalidArgument("spotlight", "Spotlight ID is required")
	}
	if err := s.client.requireScope("spotlight"); err != nil {
		return nil, err
	}
	var response spotlightEnvelope
	path := "/v1/public_profiles/" + url.PathEscape(s.client.profileID) + "/spotlights/" + url.PathEscape(spotlightID)
	if err := s.client.transport.JSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkResponse("spotlight", response.responseMeta, spotlightStates(response.Spotlights)); err != nil {
		return nil, err
	}
	for _, item := range response.Spotlights {
		if item.Spotlight.ID == spotlightID {
			return mapSpotlight(s.client.accountID, item.Spotlight), nil
		}
	}
	return nil, platformError("spotlight", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
}

func profileStates(items []profileSubResponse) []subRequestState {
	states := make([]subRequestState, 0, len(items))
	for _, item := range items {
		states = append(states, subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason})
	}
	return states
}

func spotlightStates(items []spotlightSubResponse) []subRequestState {
	states := make([]subRequestState, 0, len(items))
	for _, item := range items {
		states = append(states, subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason})
	}
	return states
}

func pageOfSpotlights(items []socialhub.Post, input paging) socialhub.Page[socialhub.Post] {
	var next *string
	if input.NextPageID != "" {
		value := input.NextPageID
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}
}
